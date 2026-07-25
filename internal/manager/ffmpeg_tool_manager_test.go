package manager

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
