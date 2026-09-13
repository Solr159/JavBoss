package manager

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ulikunitz/xz"
)

func TestFFmpegToolManagerDownloadsArchives(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("validation fixture uses a shell script")
	}
	payload := []byte("#!/bin/sh\necho 'ffmpeg version archive-test'\n")
	for _, format := range []string{"zip", "tar.xz"} {
		for _, scenario := range []string{"install", "missing binary", "wrong archive checksum", "wrong binary checksum", "truncated archive"} {
			t.Run(format+"/"+scenario, func(t *testing.T) {
				member := "ffmpeg-build/bin/ffmpeg"
				if scenario == "missing binary" {
					member = "ffmpeg-build/bin/ffprobe"
				}
				archive := ffmpegTestArchive(t, format, member, payload)
				if scenario == "truncated archive" {
					archive = archive[:len(archive)/2]
				}
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					_, _ = w.Write(archive)
				}))
				defer server.Close()
				dir := t.TempDir()
				targetPath := filepath.Join(dir, "ffmpeg")
				old := []byte("old FFmpeg")
				if err := os.WriteFile(targetPath, old, 0o755); err != nil {
					t.Fatal(err)
				}
				manager := &FFmpegToolManager{
					context: context.Background(), targetPath: targetPath, tempDir: dir,
					downloadURL: server.URL, httpClient: server.Client(),
					downloadSHA: fmt.Sprintf("%x", sha256.Sum256(archive)),
					binarySHA:   fmt.Sprintf("%x", sha256.Sum256(payload)),
				}
				if scenario == "wrong archive checksum" {
					manager.downloadSHA = strings.Repeat("0", 64)
				}
				if scenario == "wrong binary checksum" {
					manager.binarySHA = strings.Repeat("0", 64)
				}
				err := manager.downloadToTarget()
				want := old
				if scenario == "install" {
					if err != nil {
						t.Fatal(err)
					}
					want = payload
				} else if err == nil {
					t.Fatal("invalid download was accepted")
				}
				got, err := os.ReadFile(targetPath)
				if err != nil || !bytes.Equal(got, want) {
					t.Fatalf("installed content = %q, error = %v; want %q", got, err, want)
				}
				entries, err := os.ReadDir(dir)
				if err != nil || len(entries) != 1 {
					t.Fatalf("temporary files not cleaned up: %v, %v", entries, err)
				}
			})
		}
	}
}

func ffmpegTestArchive(t *testing.T, format, member string, payload []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if format == "zip" {
		writer := zip.NewWriter(&buffer)
		for _, name := range []string{"ffmpeg-build/README.txt", member} {
			entry, err := writer.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := entry.Write(payload); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
	} else {
		compressed, err := xz.NewWriter(&buffer)
		if err != nil {
			t.Fatal(err)
		}
		writer := tar.NewWriter(compressed)
		for _, name := range []string{"ffmpeg-build/README.txt", member} {
			if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(payload))}); err != nil {
				t.Fatal(err)
			}
			if _, err := writer.Write(payload); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		if err := compressed.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return buffer.Bytes()
}
