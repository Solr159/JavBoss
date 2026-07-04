package mpv

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"javboss/internal/common"
	dbpkg "javboss/internal/db"
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

func TestBuildLoadFileCommandReplacesCurrentFile(t *testing.T) {
	command := buildLoadFileCommand("/videos/a.mp4", PlayOptions{StartTimeSec: 12.345})
	expected := []any{"loadfile", "/videos/a.mp4", "replace", "start=12.345"}
	if !reflect.DeepEqual(command, expected) {
		t.Fatalf("expected loadfile command %v, got %v", expected, command)
	}
}

func TestBuildBeforeLoadCommandsRestoreWindowAndConfigureScreenshots(t *testing.T) {
	dataDir := t.TempDir()

	commands, err := buildBeforeLoadCommands(PlayOptions{
		DataDir: dataDir,
		VideoID: 42,
	})
	if err != nil {
		t.Fatalf("buildBeforeLoadCommands returned error: %v", err)
	}

	expectedDir := filepath.Join(dataDir, "video", "42", "screenshot")
	expected := [][]any{
		{"write-watch-later-config"},
		{"set_property", "window-minimized", false},
		{"set_property", "pause", false},
		{"set_property", "screenshot-template", playbackScreenshotTemplate},
		{"set_property", "screenshot-directory", expectedDir},
	}
	if !reflect.DeepEqual(commands, expected) {
		t.Fatalf("expected before-load commands %v, got %v", expected, commands)
	}
}

func TestBuildBeforeLoadCommandsSkipsWatchLaterWhenResumeDisabled(t *testing.T) {
	prevDB := common.DB
	db, err := dbpkg.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	common.DB = db
	t.Cleanup(func() {
		common.DB = prevDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := dbpkg.UpsertConfig(context.Background(), map[string]string{
		playerResumePlaybackConfigKey: "false",
	}); err != nil {
		t.Fatalf("upsert config: %v", err)
	}

	commands, err := buildBeforeLoadCommands(PlayOptions{})
	if err != nil {
		t.Fatalf("buildBeforeLoadCommands returned error: %v", err)
	}

	if len(commands) == 0 || commands[0][0] == "write-watch-later-config" {
		t.Fatalf("expected before-load commands to skip watch-later write, got %v", commands)
	}
}

func TestBuildBeforeLoadCommandsUsesFallbackScreenshotDirWithoutVideoID(t *testing.T) {
	commands, err := buildBeforeLoadCommands(PlayOptions{})
	if err != nil {
		t.Fatalf("buildBeforeLoadCommands returned error: %v", err)
	}

	if len(commands) != 5 {
		t.Fatalf("expected fallback screenshot directory command, got %v", commands)
	}
	if commands[4][0] != "set_property" || commands[4][1] != "screenshot-directory" {
		t.Fatalf("expected fallback screenshot directory command, got %v", commands[4])
	}
}

func TestBuildThumbfastScriptArgsUsesResolvedMPVPath(t *testing.T) {
	mpvPath := filepath.Join(t.TempDir(), "mpv with spaces")
	args := buildThumbfastScriptArgs(mpvPath)
	if len(args) != 1 || args[0] != "--script-opt=thumbfast-mpv_path="+mpvPath {
		t.Fatalf("expected thumbfast mpv path script option, got %v", args)
	}
}
