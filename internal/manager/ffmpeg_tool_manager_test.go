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
	"strings"
	"testing"
	"time"
)

func TestFFmpegDownloadSources(t *testing.T) {
	if len(ffmpegDownloads) != 4 {
		t.Fatalf("download source count = %d, want 4", len(ffmpegDownloads))
	}
	tests := map[string]struct {
		version   string
		urlMarker string
		gzip      bool
	}{
		"windows/amd64": {ffmpegRelease, "shaka-project/static-ffmpeg-binaries/releases/download/n8.1.2-1/", false},
		"linux/amd64":   {ffmpegRelease, "shaka-project/static-ffmpeg-binaries/releases/download/n8.1.2-1/", false},
		"darwin/amd64":  {"6.1.1", "eugeneware/ffmpeg-static/releases/download/b6.1.1/", true},
		"darwin/arm64":  {"6.1.1", "eugeneware/ffmpeg-static/releases/download/b6.1.1/", true},
	}
	for platform, want := range tests {
		download, ok := ffmpegDownloads[platform]
		if !ok {
			t.Errorf("missing download source for %s", platform)
			continue
		}
		if download.version != want.version {
			t.Errorf("%s version = %q, want %q", platform, download.version, want.version)
		}
		if !strings.Contains(download.url, want.urlMarker) {
			t.Errorf("%s download URL = %q, want marker %q", platform, download.url, want.urlMarker)
		}
		if strings.HasSuffix(download.url, ".gz") != want.gzip {
			t.Errorf("%s gzip URL = %t, want %t", platform, strings.HasSuffix(download.url, ".gz"), want.gzip)
		}
		for name, checksum := range map[string]string{
			"download": download.downloadSHA,
			"binary":   download.binarySHA256,
		} {
			if len(checksum) != sha256.Size*2 {
				t.Errorf("%s %s SHA-256 length = %d, want %d", platform, name, len(checksum), sha256.Size*2)
			}
		}
	}
}

func TestFFmpegToolManagerDownloadsAndInstalls(t *testing.T) {
	payload := []byte("#!/bin/sh\necho 'ffmpeg version test'\n")
	archive := gzipTestPayload(t, payload)
	digest := sha256.Sum256(archive)
	binaryDigest := sha256.Sum256(payload)
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
		binarySHA:   hex.EncodeToString(binaryDigest[:]),
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

func TestFFmpegToolManagerDownloadsRawBinary(t *testing.T) {
	payload := []byte("#!/bin/sh\necho 'ffmpeg version test'\n")
	digest := sha256.Sum256(payload)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	targetPath := filepath.Join(t.TempDir(), "internal", "bin", "ffmpeg")
	manager := &FFmpegToolManager{
		context:     context.Background(),
		targetPath:  targetPath,
		displayPath: "internal/bin/ffmpeg",
		downloadURL: server.URL,
		downloadSHA: hex.EncodeToString(digest[:]),
		binarySHA:   hex.EncodeToString(digest[:]),
		httpClient:  server.Client(),
		resolveFFmpeg: func() (string, error) {
			return "", errors.New("FFmpeg not found")
		},
	}

	if err := manager.downloadToTarget(); err != nil {
		t.Fatalf("download raw FFmpeg: %v", err)
	}
	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read installed FFmpeg: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("installed FFmpeg content = %q, want %q", got, payload)
	}
}

func TestFFmpegToolManagerRejectsChecksumMismatch(t *testing.T) {
	payload := []byte("#!/bin/sh\necho 'ffmpeg version test'\n")
	archive := gzipTestPayload(t, payload)
	binaryDigest := sha256.Sum256(payload)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	targetPath := filepath.Join(t.TempDir(), "internal", "bin", "ffmpeg")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("create target directory: %v", err)
	}
	oldContent := []byte("old FFmpeg")
	if err := os.WriteFile(targetPath, oldContent, 0o755); err != nil {
		t.Fatalf("write old FFmpeg: %v", err)
	}
	manager := &FFmpegToolManager{
		context:     context.Background(),
		targetPath:  targetPath,
		displayPath: "internal/bin/ffmpeg",
		downloadURL: server.URL,
		downloadSHA: "wrong-checksum",
		binarySHA:   hex.EncodeToString(binaryDigest[:]),
		httpClient:  server.Client(),
		resolveFFmpeg: func() (string, error) {
			return targetPath, nil
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
	gotContent, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read old FFmpeg after checksum mismatch: %v", err)
	}
	if !bytes.Equal(gotContent, oldContent) {
		t.Fatalf("old FFmpeg changed after checksum mismatch: got %q, want %q", gotContent, oldContent)
	}
}

func TestInstallFFmpegFileStagesAndAtomicallyReplacesExistingTarget(t *testing.T) {
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
	oldContent := []byte("old FFmpeg")
	if err := os.WriteFile(targetPath, oldContent, 0o755); err != nil {
		t.Fatalf("write old FFmpeg: %v", err)
	}

	renameCalls := 0
	renameFile := func(oldPath string, newPath string) error {
		renameCalls++
		if filepath.Dir(oldPath) != filepath.Dir(targetPath) {
			t.Fatalf("staged path directory = %q, want %q", filepath.Dir(oldPath), filepath.Dir(targetPath))
		}
		gotOldContent, err := os.ReadFile(targetPath)
		if err != nil {
			t.Fatalf("read target before atomic replacement: %v", err)
		}
		if !bytes.Equal(gotOldContent, oldContent) {
			t.Fatalf("target changed before atomic replacement: got %q, want %q", gotOldContent, oldContent)
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
	if renameCalls != 1 {
		t.Fatalf("rename calls = %d, want 1", renameCalls)
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

func TestFFmpegToolManagerTreatsManagedChecksumMismatchAsUpgrade(t *testing.T) {
	oldPayload := []byte("#!/bin/sh\necho 'ffmpeg version old'\n")
	newPayload := []byte("#!/bin/sh\necho 'ffmpeg version new'\n")
	newDigest := sha256.Sum256(newPayload)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(newPayload)
	}))
	defer server.Close()

	targetPath := filepath.Join(t.TempDir(), "data", "tools", "linux-x86_64", "ffmpeg")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("create target directory: %v", err)
	}
	if err := os.WriteFile(targetPath, oldPayload, 0o755); err != nil {
		t.Fatalf("write old FFmpeg: %v", err)
	}

	manager := &FFmpegToolManager{
		context:     context.Background(),
		targetPath:  targetPath,
		displayPath: "data/tools/linux-x86_64/ffmpeg",
		version:     ffmpegRelease,
		downloadURL: server.URL,
		downloadSHA: hex.EncodeToString(newDigest[:]),
		binarySHA:   hex.EncodeToString(newDigest[:]),
		httpClient:  server.Client(),
		resolveFFmpeg: func() (string, error) {
			return targetPath, nil
		},
	}

	status := manager.Status()
	if status.Installed {
		t.Fatal("installed = true for outdated managed FFmpeg")
	}
	if !status.UpgradeAvailable {
		t.Fatal("upgrade_available = false for outdated managed FFmpeg")
	}

	started, err := manager.StartDownload()
	if err != nil {
		t.Fatalf("start upgrade: %v", err)
	}
	if !started {
		t.Fatal("StartDownload() = false for outdated managed FFmpeg")
	}
	status = waitForFFmpegDownload(t, manager)
	if !status.Installed || status.UpgradeAvailable {
		t.Fatalf("status after upgrade = %+v", status)
	}
	if status.Version != ffmpegRelease {
		t.Fatalf("version = %q, want %q", status.Version, ffmpegRelease)
	}
	gotPayload, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read upgraded FFmpeg: %v", err)
	}
	if !bytes.Equal(gotPayload, newPayload) {
		t.Fatalf("upgraded FFmpeg = %q, want %q", gotPayload, newPayload)
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
