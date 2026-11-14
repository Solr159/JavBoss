package screenshot

import (
	"slices"
	"testing"
)

func TestBuildFFmpegScreenshotArgsIncludesHeadlessImageOptions(t *testing.T) {
	args := buildFFmpegScreenshotArgs(12, "/tmp/screenshots/00000001.jpg", "/videos/example.mp4", false)

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
	args := buildMPVScreenshotArgs(12, "/tmp/screenshots", "/videos/example.mp4", false)

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

func TestScreenshotsAlwaysUsePreciseSeeking(t *testing.T) {
	for _, preview := range []bool{false, true} {
		args := buildFFmpegScreenshotArgs(20, "frame.jpg", "video.mp4", preview)
		if slices.Contains(args, "-noaccurate_seek") || slices.Contains(args, "-copyts") {
			t.Fatalf("unexpected approximate FFmpeg seek: %v", args)
		}
		if preview && !slices.Contains(args, "scale=320:180:force_original_aspect_ratio=decrease") {
			t.Fatalf("preview must retain thumbnail scaling: %v", args)
		}
		args = buildMPVScreenshotArgs(20, "screenshots", "video.mp4", preview)
		if !slices.Contains(args, "--hr-seek=yes") || slices.Contains(args, "--hr-seek=no") {
			t.Fatalf("MPV must seek accurately: %v", args)
		}
	}
}
