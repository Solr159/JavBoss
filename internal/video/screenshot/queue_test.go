package screenshot

import (
	"slices"
	"testing"
)

func TestBuildFFmpegScreenshotArgsIncludesHeadlessImageOptions(t *testing.T) {
	args := buildFFmpegScreenshotArgs(12, "/tmp/screenshots/00000001.jpg", "/videos/example.mp4")

	required := []string{
		"-nostdin",
		"-hide_banner",
		"-loglevel",
		"error",
		"-y",
		"-ss",
		"12",
		"-i",
		"/videos/example.mp4",
		"-map",
		"0:v:0",
		"-frames:v",
		"1",
		"-q:v",
		"2",
		"/tmp/screenshots/00000001.jpg",
	}
	for _, option := range required {
		if !slices.Contains(args, option) {
			t.Fatalf("expected ffmpeg screenshot args to include %q, got %v", option, args)
		}
	}
}

func TestBuildMPVScreenshotArgsIncludesImageOutputOptions(t *testing.T) {
	args := buildMPVScreenshotArgs(12, "/tmp/screenshots", "/videos/example.mp4")

	required := []string{
		"--no-config",
		"--ao=null",
		"--start=12",
		"--frames=1",
		"--vo=image",
		"--vo-image-format=jpg",
		"--vo-image-outdir=/tmp/screenshots",
		"/videos/example.mp4",
	}
	for _, option := range required {
		if !slices.Contains(args, option) {
			t.Fatalf("expected mpv screenshot args to include %q, got %v", option, args)
		}
	}
}
