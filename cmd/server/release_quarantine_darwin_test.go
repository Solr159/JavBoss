package main

import (
	"bytes"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"javboss/internal/common/logging"

	"github.com/gin-gonic/gin"
	"golang.org/x/sys/unix"
)

func TestClearReleaseQuarantine(t *testing.T) {
	dir := t.TempDir()
	quarantined := []string{dir}
	for _, name := range []string{"javboss", "internal/bin/ffmpeg", "internal/bin/mpv/mpv.app/Contents/MacOS/mpv", "internal/bin/mpv/mpv.app/Contents/Frameworks/libMoltenVK.dylib", "data/keep", "logs/keep"} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("test fixture"), 0o644); err != nil {
			t.Fatal(err)
		}
		quarantined = append(quarantined, path)
	}
	quarantined = append(quarantined, filepath.Join(dir, "internal/bin/mpv/mpv.app"))
	external := t.TempDir()
	quarantined = append(quarantined, external)
	if err := os.Symlink(external, filepath.Join(dir, "external")); err != nil {
		t.Fatal(err)
	}
	for _, path := range quarantined {
		if err := unix.Setxattr(path, quarantineAttribute, []byte("0081;00000000;JavBossTest;"), 0); err != nil {
			t.Fatal(err)
		}
	}
	otherAttribute := "com.javboss.test"
	if err := unix.Setxattr(dir, otherAttribute, []byte("preserve"), 0); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := clearReleaseQuarantine(dir); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range quarantined {
		_, err := unix.Getxattr(path, quarantineAttribute, nil)
		preserved := path == external || path == filepath.Join(dir, "data/keep") || path == filepath.Join(dir, "logs/keep")
		if preserved && err != nil {
			t.Errorf("quarantine outside release files was changed on %s: %v", path, err)
		} else if !preserved && !errors.Is(err, unix.ENOATTR) {
			t.Errorf("quarantine remains on %s: %v", path, err)
		}
	}
	if _, err := unix.Getxattr(dir, otherAttribute, nil); err != nil {
		t.Fatalf("unrelated attribute was changed: %v", err)
	}
}

func TestClearQuarantineLogsEachFailureAndContinues(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can remove attributes from read-only files")
	}
	dir := t.TempDir()
	frameworks := filepath.Join(dir, "internal/bin/mpv/mpv.app/Contents/Frameworks")
	if err := os.MkdirAll(frameworks, 0o755); err != nil {
		t.Fatal(err)
	}
	readOnly := []string{"a-readonly.dylib", "b-readonly.dylib"}
	for _, name := range append(readOnly, "z-writable.dylib") {
		path := filepath.Join(frameworks, name)
		if err := os.WriteFile(path, []byte("fixture"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := unix.Setxattr(path, quarantineAttribute, []byte("0081;00000000;JavBossTest;"), 0); err != nil {
			t.Fatal(err)
		}
		if name != "z-writable.dylib" {
			if err := os.Chmod(path, 0o444); err != nil {
				t.Fatal(err)
			}
		}
	}
	var output bytes.Buffer
	previousOutput := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(previousOutput) })
	previousMode := gin.Mode()
	gin.SetMode(gin.ReleaseMode)
	t.Cleanup(func() { gin.SetMode(previousMode) })
	logger, closeLogs, err := buildLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeLogs)
	logging.SetLogger(logger)
	t.Cleanup(func() { logging.SetLogger(log.Default()) })
	if err := clearReleaseQuarantine(dir); err != nil {
		t.Fatalf("individual file failures stopped cleanup: %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("cleanup wrote to the console: %s", output.String())
	}
	fileOutput, err := os.ReadFile(filepath.Join(dir, "logs", "javboss.log"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range readOnly {
		path := filepath.Join(frameworks, name)
		if !strings.Contains(string(fileOutput), "failed on "+path+", skipping:") {
			t.Errorf("missing failure log for %s: %s", path, fileOutput)
		}
		if _, err := unix.Getxattr(path, quarantineAttribute, nil); err != nil {
			t.Errorf("read-only file was changed: %v", err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o444 {
			t.Errorf("read-only permissions changed: %o", info.Mode().Perm())
		}
	}
	if _, err := unix.Getxattr(filepath.Join(frameworks, "z-writable.dylib"), quarantineAttribute, nil); !errors.Is(err, unix.ENOATTR) {
		t.Fatalf("cleanup did not continue after failed files: %v", err)
	}
}
