package mpv

import (
	"path/filepath"
	"testing"
)

func TestEnsurePlaybackScreenshotDirUsesVideoDataDirectory(t *testing.T) {
	dataDir := t.TempDir()

	dir, err := ensurePlaybackScreenshotDir(PlayOptions{
		DataDir: dataDir,
		VideoID: 42,
	})
	if err != nil {
		t.Fatalf("ensurePlaybackScreenshotDir returned error: %v", err)
	}

	expected := filepath.Join(dataDir, "video", "42", "screenshot")
	if dir != expected {
		t.Fatalf("expected screenshot dir %q, got %q", expected, dir)
	}
}

func TestBuildPlaybackScreenshotArgsIncludeTimeTemplate(t *testing.T) {
	dataDir := t.TempDir()

	args, err := buildPlaybackScreenshotArgs(PlayOptions{
		DataDir: dataDir,
		VideoID: 42,
	})
	if err != nil {
		t.Fatalf("buildPlaybackScreenshotArgs returned error: %v", err)
	}

	expectedDir := filepath.Join(dataDir, "video", "42", "screenshot")
	expected := []string{
		"--screenshot-directory=" + expectedDir,
		"--screenshot-template=mpv_%wH-%wM-%wS.%wT",
	}
	if len(args) != len(expected) {
		t.Fatalf("expected screenshot args %v, got %v", expected, args)
	}
	for i := range expected {
		if args[i] != expected[i] {
			t.Fatalf("expected screenshot args %v, got %v", expected, args)
		}
	}
}

func TestBuildPlaybackStartArgsIncludesStartTime(t *testing.T) {
	args := buildPlaybackStartArgs(PlayOptions{StartTimeSec: 12.345})
	if len(args) != 1 || args[0] != "--start=12.345" {
		t.Fatalf("expected start args, got %v", args)
	}
}

func TestBuildThumbfastScriptArgsUsesResolvedMPVPath(t *testing.T) {
	mpvPath := filepath.Join(t.TempDir(), "mpv with spaces")
	args := buildThumbfastScriptArgs(mpvPath)
	if len(args) != 1 || args[0] != "--script-opt=thumbfast-mpv_path="+mpvPath {
		t.Fatalf("expected thumbfast mpv path script option, got %v", args)
	}
}
