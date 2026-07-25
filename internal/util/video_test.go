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
	t.Setenv("JAVBOSS_CONTAINER", "")
	t.Setenv("JAVBOSS_DOCKER", "")

	ignoredEnvPath := filepath.Join(baseDir, "ignored-env-ffmpeg")
	if err := os.WriteFile(ignoredEnvPath, []byte("ignored ffmpeg"), 0o755); err != nil {
		t.Fatalf("write ignored environment FFmpeg fixture: %v", err)
	}
	t.Setenv("FFMPEG_PATH", ignoredEnvPath)

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

func TestFindFFmpegPathUsesEnvironmentInContainerMode(t *testing.T) {
	ffmpegPath := filepath.Join(t.TempDir(), "container-ffmpeg")
	if err := os.WriteFile(ffmpegPath, []byte("container ffmpeg"), 0o755); err != nil {
		t.Fatalf("write container FFmpeg fixture: %v", err)
	}
	t.Setenv("JAVBOSS_CONTAINER", "1")
	t.Setenv("JAVBOSS_DOCKER", "")
	t.Setenv("FFMPEG_PATH", ffmpegPath)

	got, err := findFFmpegPath()
	if err != nil {
		t.Fatalf("find FFmpeg: %v", err)
	}
	if filepath.Clean(got) != filepath.Clean(ffmpegPath) {
		t.Fatalf("findFFmpegPath() = %q, want %q", got, ffmpegPath)
	}
}

func TestShouldUseFFmpegPathOnlyInContainerMode(t *testing.T) {
	if shouldUseFFBinaryEnv("ffmpeg", false) {
		t.Fatal("FFMPEG_PATH should be ignored outside container mode")
	}
	if !shouldUseFFBinaryEnv("ffmpeg", true) {
		t.Fatal("FFMPEG_PATH should be used in container mode")
	}
	if !shouldUseFFBinaryEnv("ffprobe", false) {
		t.Fatal("FFPROBE_PATH should remain available outside container mode")
	}
}

func TestFFBinaryCandidatesForBasePlatformOrder(t *testing.T) {
	baseDir := filepath.Join("project", "root")
	binName := "ffmpeg"
	toolPath := filepath.Join("data", "tools", "platform", binName)
	bundledPath := filepath.Join(baseDir, "internal", "bin", binName)
	downloadedPath := filepath.Join(baseDir, toolPath)

	tests := []struct {
		name string
		goos string
		want []string
	}{
		{name: "macOS prioritizes bundled FFmpeg", goos: "darwin", want: []string{bundledPath, downloadedPath}},
		{name: "Windows prioritizes downloaded FFmpeg", goos: "windows", want: []string{downloadedPath, bundledPath}},
		{name: "Linux prioritizes downloaded FFmpeg", goos: "linux", want: []string{downloadedPath, bundledPath}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ffBinaryCandidatesForBase(baseDir, "ffmpeg", binName, tt.goos, toolPath)
			if len(got) != len(tt.want) {
				t.Fatalf("candidate count = %d, want %d", len(got), len(tt.want))
			}
			for index := range tt.want {
				if got[index] != tt.want[index] {
					t.Fatalf("candidate[%d] = %q, want %q", index, got[index], tt.want[index])
				}
			}
		})
	}
}
