package manager

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestFFmpegToolManagerDownloadsAndInstalls(t *testing.T) {
	archive := gzipTestPayload(t, []byte("#!/bin/sh\necho 'ffmpeg version test'\n"))
	digest := sha256.Sum256(archive)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	targetPath := filepath.Join(t.TempDir(), "internal", "bin", "ffmpeg")
	manager := &FFmpegToolManager{
		context:     context.Background(),
		targetPath:  targetPath,
		displayPath: "internal/bin/ffmpeg",
		downloadURL: server.URL,
		downloadSHA: hex.EncodeToString(digest[:]),
		httpClient:  server.Client(),
		resolveFFmpeg: func() (string, error) {
			return "", errors.New("FFmpeg not found")
		},
	}

	started, err := manager.StartDownload()
	if err != nil {
		t.Fatalf("start download: %v", err)
	}
	if !started {
		t.Fatal("StartDownload() = false, want true")
	}

	status := waitForFFmpegDownload(t, manager)
	if !status.Installed {
		t.Fatalf("installed = false; error=%q", status.Error)
	}
	if status.Progress != 100 {
		t.Fatalf("progress = %d, want 100", status.Progress)
	}
	if status.Source != "downloaded" {
		t.Fatalf("source = %q, want downloaded", status.Source)
	}
	if status.Path != "internal/bin/ffmpeg" {
		t.Fatalf("path = %q, want internal/bin/ffmpeg", status.Path)
	}
	if _, err := os.Stat(targetPath); err != nil {
		t.Fatalf("stat installed FFmpeg: %v", err)
	}

	started, err = manager.StartDownload()
	if err != nil {
		t.Fatalf("start existing download: %v", err)
	}
	if started {
		t.Fatal("StartDownload() for installed FFmpeg = true, want false")
	}
}

func TestFFmpegToolManagerRejectsChecksumMismatch(t *testing.T) {
	archive := gzipTestPayload(t, []byte("#!/bin/sh\necho 'ffmpeg version test'\n"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	targetPath := filepath.Join(t.TempDir(), "internal", "bin", "ffmpeg")
	manager := &FFmpegToolManager{
		context:     context.Background(),
		targetPath:  targetPath,
		displayPath: "internal/bin/ffmpeg",
		downloadURL: server.URL,
		downloadSHA: "wrong-checksum",
		httpClient:  server.Client(),
		resolveFFmpeg: func() (string, error) {
			return "", errors.New("FFmpeg not found")
		},
	}

	if _, err := manager.StartDownload(); err != nil {
		t.Fatalf("start download: %v", err)
	}
	status := waitForFFmpegDownload(t, manager)
	if status.Installed {
		t.Fatal("installed = true after checksum mismatch")
	}
	if status.Error == "" {
		t.Fatal("error is empty after checksum mismatch")
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target exists after checksum mismatch: %v", err)
	}
}

func TestInstallFFmpegFileFallsBackToCopyAcrossFilesystems(t *testing.T) {
	baseDir := t.TempDir()
	sourcePath := filepath.Join(baseDir, "system-temp", "ffmpeg.exe")
	targetPath := filepath.Join(baseDir, "project", "data", "tools", "ffmpeg.exe")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("create source directory: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("create target directory: %v", err)
	}
	wantContent := []byte("verified FFmpeg")
	if err := os.WriteFile(sourcePath, wantContent, 0o755); err != nil {
		t.Fatalf("write source FFmpeg: %v", err)
	}

	renameCalls := 0
	renameFile := func(oldPath string, newPath string) error {
		renameCalls++
		if renameCalls == 1 {
			return errors.New("simulated cross-filesystem rename")
		}
		return os.Rename(oldPath, newPath)
	}
	if err := installFFmpegFileWithRename(sourcePath, targetPath, renameFile); err != nil {
		t.Fatalf("install FFmpeg: %v", err)
	}

	gotContent, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read installed FFmpeg: %v", err)
	}
	if !bytes.Equal(gotContent, wantContent) {
		t.Fatalf("installed FFmpeg content = %q, want %q", gotContent, wantContent)
	}
	if renameCalls != 2 {
		t.Fatalf("rename calls = %d, want 2", renameCalls)
	}
	if _, err := os.Stat(sourcePath); err != nil {
		t.Fatalf("source FFmpeg should remain for caller cleanup: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(targetPath), ".ffmpeg-install-*"))
	if err != nil {
		t.Fatalf("glob staged FFmpeg files: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("staged FFmpeg files were not cleaned up: %v", matches)
	}
}

func TestNewFFmpegToolManagerUsesPersistentDataPath(t *testing.T) {
	baseDir := t.TempDir()
	manager := NewFFmpegToolManager(context.Background(), baseDir)

	wantRelativePath := filepath.Join(
		"data",
		"tools",
		currentTestPlatformLabel(),
		currentTestFFmpegBinaryName(),
	)
	if manager.targetPath != filepath.Join(baseDir, wantRelativePath) {
		t.Fatalf("targetPath = %q, want %q", manager.targetPath, filepath.Join(baseDir, wantRelativePath))
	}
	if manager.displayPath != filepath.ToSlash(wantRelativePath) {
		t.Fatalf("displayPath = %q, want %q", manager.displayPath, filepath.ToSlash(wantRelativePath))
	}
	if manager.tempDir != os.TempDir() {
		t.Fatalf("tempDir = %q, want system temp directory %q", manager.tempDir, os.TempDir())
	}
}

func TestFFmpegToolManagerDetectsBuiltInFFmpeg(t *testing.T) {
	tests := []struct {
		name          string
		containerMode bool
		resolvedPath  func(string) string
	}{
		{
			name: "macOS release bundle",
			resolvedPath: func(baseDir string) string {
				return filepath.Join(baseDir, "internal", "bin", "ffmpeg")
			},
		},
		{
			name:          "Docker image",
			containerMode: true,
			resolvedPath: func(baseDir string) string {
				return filepath.Join(baseDir, "usr", "local", "bin", "ffmpeg")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseDir := t.TempDir()
			resolvedPath := tt.resolvedPath(baseDir)
			if err := os.MkdirAll(filepath.Dir(resolvedPath), 0o755); err != nil {
				t.Fatalf("create built-in FFmpeg directory: %v", err)
			}
			if err := os.WriteFile(resolvedPath, []byte("built-in ffmpeg"), 0o755); err != nil {
				t.Fatalf("write built-in FFmpeg: %v", err)
			}

			manager := &FFmpegToolManager{
				context:       context.Background(),
				targetPath:    filepath.Join(baseDir, "data", "tools", "test", "ffmpeg"),
				bundledDir:    filepath.Join(baseDir, "internal", "bin"),
				containerMode: tt.containerMode,
				resolveFFmpeg: func() (string, error) {
					return resolvedPath, nil
				},
			}

			status := manager.Status()
			if !status.Installed {
				t.Fatal("installed = false, want true")
			}
			if status.Source != "builtin" {
				t.Fatalf("source = %q, want builtin", status.Source)
			}
			started, err := manager.StartDownload()
			if err != nil {
				t.Fatalf("start download: %v", err)
			}
			if started {
				t.Fatal("StartDownload() = true for built-in FFmpeg")
			}
		})
	}
}

func currentTestPlatformLabel() string {
	platformOS := runtime.GOOS
	if platformOS == "darwin" {
		platformOS = "macos"
	}
	platformArch := runtime.GOARCH
	if platformArch == "amd64" {
		platformArch = "x86_64"
	}
	return platformOS + "-" + platformArch
}

func currentTestFFmpegBinaryName() string {
	if runtime.GOOS == "windows" {
		return "ffmpeg.exe"
	}
	return "ffmpeg"
}

func gzipTestPayload(t *testing.T, payload []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, err := writer.Write(payload); err != nil {
		t.Fatalf("write gzip payload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close gzip payload: %v", err)
	}
	return buffer.Bytes()
}

func waitForFFmpegDownload(t *testing.T, manager *FFmpegToolManager) FFmpegToolStatus {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		status := manager.Status()
		if !status.Downloading {
			return status
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for FFmpeg download")
	return FFmpegToolStatus{}
}
