//go:build !windows

package mpv

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestShutdownAllPlaybackModesWithMPV(t *testing.T) {
	if os.Getenv("JAVBOSS_TEST_MPV") == "" {
		t.Skip("set JAVBOSS_TEST_MPV to run the real MPV regression test")
	}
	if os.Getenv("JAVBOSS_MPV_SHUTDOWN_HELPER") == "" {
		// Isolate cached MPV paths and the terminal Shutdown state from other tests.
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(executable, "-test.run=^TestShutdownAllPlaybackModesWithMPV$")
		cmd.Env = append(os.Environ(), "JAVBOSS_MPV_SHUTDOWN_HELPER=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("mpv shutdown integration: %v\n%s", err, output)
		}
		return
	}
	dir := t.TempDir()
	wrapper := filepath.Join(dir, "mpv-headless")
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\nexec \"$JAVBOSS_TEST_MPV\" --vo=null --ao=null --image-display-duration=inf \"$@\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MPV_PATH", wrapper)
	t.Setenv("JAVBOSS_BUILD_MODE", "release")
	t.Setenv(modernZEnvDir, writeModernZTestAssets(t))
	t.Cleanup(Shutdown)
	image, err := filepath.Abs("../../web/public/icon-192.png")
	if err != nil {
		t.Fatal(err)
	}
	options := PlayOptions{Title: "中文 {测试} 100%.mp4", DataDir: dir, VideoID: 1}
	// Client playback uses the single-video entry point with subtitle callbacks.
	options.SaveSubtitleOffset = func(context.Context, int64) error { return nil }
	if err := PlayVideo(image, options); err != nil {
		t.Fatal(err)
	}
	if err := playVideoInNewProcess(image, options); err != nil {
		t.Fatal(err)
	}
	if err := playPlaylistInNewProcess([]PlaylistItem{{Path: image, Title: options.Title, Options: options}}); err != nil {
		t.Fatal(err)
	}
	playerProcesses.mu.Lock()
	var players []*playerProcess
	for p := range playerProcesses.processes {
		players = append(players, p)
	}
	playerProcesses.mu.Unlock()
	if len(players) != 3 {
		t.Fatalf("tracked %d players, want all three playback modes", len(players))
	}
	for _, p := range players {
		if err := waitForIPCReady(p.ipcPath); err != nil {
			t.Fatal(err)
		}
		waitMPVTestProperty(t, p.ipcPath, "media-title", options.Title)
	}
	// Loading a subsequent untitled file must restore MPV's filename fallback.
	if err := defaultSession.PlayVideo(image, PlayOptions{DataDir: dir, VideoID: 2}); err != nil {
		t.Fatal(err)
	}
	waitMPVTestProperty(t, defaultSession.ipcPath, "media-title", filepath.Base(image))
	Shutdown()
	for _, p := range players {
		if p.running() || p.cmd.ProcessState == nil || !p.cmd.ProcessState.Success() {
			t.Fatalf("player did not exit gracefully: %v", p.cmd.ProcessState)
		}
		if _, err := os.Stat(p.ipcPath); !os.IsNotExist(err) {
			t.Fatalf("player IPC socket was not cleaned up: %v", err)
		}
	}
}
