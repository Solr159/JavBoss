//go:build !windows

package mpv

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Opt in with JAVBOSS_TEST_MPV pointing to the bundled MPV executable.
// The wrapper keeps this regression test headless and records actual startup arguments.
func TestPlaylistIPCAndScreenshotRoutingWithMPV(t *testing.T) {
	if os.Getenv("JAVBOSS_TEST_MPV") == "" {
		t.Skip("set JAVBOSS_TEST_MPV to run the real MPV regression test")
	}
	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args")
	wrapper := filepath.Join(dir, "mpv-test")
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$JAVBOSS_TEST_ARGS\"\nexec \"$JAVBOSS_TEST_MPV\" \"$@\" --vo=null --ao=null --image-display-duration=inf --save-position-on-quit=no\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MPV_PATH", wrapper)
	t.Setenv("JAVBOSS_TEST_ARGS", argsPath)
	t.Setenv(modernZEnvDir, writeModernZTestAssets(t))
	imageA, err := filepath.Abs("../../web/public/icon-192.png")
	if err != nil {
		t.Fatal(err)
	}
	imageB, err := filepath.Abs("../../web/public/icon-512.png")
	if err != nil {
		t.Fatal(err)
	}
	items := make([]PlaylistItem, 600)
	for i := range items {
		items[i] = PlaylistItem{Path: imageA, Options: PlayOptions{DataDir: dir, VideoID: int64(i + 1)}}
	}
	if err := playPlaylistInNewProcess(items); err != nil {
		t.Fatal(err)
	}
	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	var endpoint string
	for _, arg := range strings.Split(string(args), "\n") {
		if strings.HasPrefix(arg, "--input-ipc-server=") {
			endpoint = strings.TrimPrefix(arg, "--input-ipc-server=")
		}
	}
	if endpoint == "" {
		t.Fatal("one-shot playlist did not create IPC")
	}
	t.Cleanup(func() { _ = runIPCCommand(endpoint, []any{"quit"}) })
	if len(args) >= 32767 || strings.Contains(string(args), imageA) {
		t.Fatal("playlist filenames were expanded into the startup command")
	}
	if got := readMPVTestProperty(t, endpoint, "playlist-count"); got != float64(600) {
		t.Fatalf("playlist-count=%v, want 600", got)
	}
	if got := readMPVTestProperty(t, endpoint, "idle"); got != false {
		t.Fatalf("one-shot idle=%v, want false", got)
	}
	if err := playPlaylistInNewProcess(items[:2]); err != nil {
		t.Fatal(err)
	}
	secondArgs, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	var secondEndpoint string
	for _, arg := range strings.Split(string(secondArgs), "\n") {
		if strings.HasPrefix(arg, "--input-ipc-server=") {
			secondEndpoint = strings.TrimPrefix(arg, "--input-ipc-server=")
		}
	}
	if secondEndpoint == "" || secondEndpoint == endpoint {
		t.Fatal("independent players shared the same IPC endpoint")
	}
	t.Cleanup(func() { _ = runIPCCommand(secondEndpoint, []any{"quit"}) })
	if got := readMPVTestProperty(t, endpoint, "playlist-count"); got != float64(600) {
		t.Fatalf("second player replaced the first playlist: %v", got)
	}
	if got := readMPVTestProperty(t, secondEndpoint, "playlist-count"); got != float64(2) {
		t.Fatalf("second playlist-count=%v, want 2", got)
	}
	waitMPVTestProperty(t, endpoint, "screenshot-directory", filepath.Join(dir, "video", "1", "screenshot"))
	s := &playerSession{ipcPath: endpoint}
	if err := s.playVideoLocked(imageB, PlayOptions{DataDir: dir, VideoID: 999}); err != nil {
		t.Fatal(err)
	}
	waitMPVTestProperty(t, endpoint, "path", imageB)
	expectedDir := filepath.Join(dir, "video", "999", "screenshot")
	waitMPVTestProperty(t, endpoint, "screenshot-directory", expectedDir)
	// A successful screenshot verifies the resulting routing, not just command construction.
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := runIPCCommand(endpoint, []any{"screenshot", "video"})
		files, _ := os.ReadDir(expectedDir)
		if err == nil && len(files) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("screenshot did not reach the single video's directory: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := runIPCCommand(endpoint, []any{"quit"}); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(endpoint); os.IsNotExist(err) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("IPC socket was not cleaned up after player exit")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func readMPVTestProperty(t *testing.T, endpoint, name string) any {
	t.Helper()
	conn, err := dialPlatformMPVIPC(endpoint, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if conn, ok := conn.(interface{ SetDeadline(time.Time) error }); ok {
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	}
	if err := json.NewEncoder(conn).Encode(map[string]any{"command": []any{"get_property", name}, "request_id": 1}); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			t.Fatal(err)
		}
		var response struct {
			RequestID int `json:"request_id"`
			Data      any `json:"data"`
		}
		if err := json.Unmarshal(line, &response); err != nil {
			t.Fatal(err)
		}
		if response.RequestID == 1 {
			return response.Data
		}
	}
}

func waitMPVTestProperty(t *testing.T, endpoint, name string, expected any) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		got := readMPVTestProperty(t, endpoint, name)
		if got == expected {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s=%v, want %v", name, got, expected)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
