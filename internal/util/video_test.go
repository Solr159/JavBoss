package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsVideoRecognizesRMVBRealMediaSignature(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.rmvb")
	if err := os.WriteFile(path, append([]byte(".RMF\x00\x00\x00\x12"), make([]byte, 32)...), 0o644); err != nil {
		t.Fatalf("write rmvb fixture: %v", err)
	}

	if !IsVideo(path) {
		t.Fatal("IsVideo should accept rmvb files with a RealMedia signature")
	}
}

func TestIsVideoRejectsRMVBWithoutRealMediaSignature(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.rmvb")
	if err := os.WriteFile(path, []byte("not a realmedia file"), 0o644); err != nil {
		t.Fatalf("write rmvb fixture: %v", err)
	}

	if IsVideo(path) {
		t.Fatal("IsVideo should reject rmvb files without a RealMedia signature")
	}
}

func TestDetectContainerRecognizesRMVBExtension(t *testing.T) {
	if got := detectContainer("rm", "/videos/sample.rmvb"); got != "rmvb" {
		t.Fatalf("detectContainer() = %q, want %q", got, "rmvb")
	}
}

func TestFindFFmpegPathUsesPersistentDataTool(t *testing.T) {
	originalWorkingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalWorkingDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	baseDir := t.TempDir()
	if err := os.Chdir(baseDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Setenv("FFMPEG_PATH", "")

	ffmpegPath := filepath.Join(baseDir, FFmpegToolRelativePath())
	if err := os.MkdirAll(filepath.Dir(ffmpegPath), 0o755); err != nil {
		t.Fatalf("create FFmpeg directory: %v", err)
	}
	if err := os.WriteFile(ffmpegPath, []byte("test ffmpeg"), 0o755); err != nil {
		t.Fatalf("write FFmpeg fixture: %v", err)
	}

	got, err := findFFmpegPath()
	if err != nil {
		t.Fatalf("find FFmpeg: %v", err)
	}
	if filepath.Clean(got) != filepath.Clean(ffmpegPath) {
		t.Fatalf("findFFmpegPath() = %q, want %q", got, ffmpegPath)
	}
}
