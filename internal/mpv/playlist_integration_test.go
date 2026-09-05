//go:build !windows

package mpv

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
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
	started := make(chan int, 100)
	for i := range items {
		index := i
		items[i] = PlaylistItem{Path: imageA, Title: "中文视频 " + strconv.Itoa(i+1), Options: PlayOptions{DataDir: dir, VideoID: int64(i + 1)}, OnStarted: func() { started <- index }}
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
	assertStarted := func(want int) {
		t.Helper()
		select {
		case got := <-started:
			if got != want {
				t.Fatalf("started entry %d, want %d", got, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("no playback notification for %d", want)
		}
	}
	assertStarted(0)
	select {
	case got := <-started:
		t.Fatalf("queued entry counted: %d", got)
	default:
	}
	titles := readMPVTestProperty(t, endpoint, "user-data/javboss/playlist-titles").(map[string]any)
	pendingID := readMPVTestProperty(t, endpoint, "playlist/1/id").(float64)
	if titles[strconv.FormatInt(int64(pendingID), 10)] != "中文视频 2" {
		t.Fatalf("pending entry has no readable title: %v", titles)
	}
	secondItems := append([]PlaylistItem(nil), items[:2]...)
	for i := range secondItems {
		secondItems[i].OnStarted = nil
	}
	if err := playPlaylistInNewProcess(secondItems); err != nil {
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
	if err := runIPCCommand(endpoint, []any{"playlist-play-index", 2}); err != nil {
		t.Fatal(err)
	}
	assertStarted(2)
	if err := runIPCCommand(endpoint, []any{"playlist-play-index", 2}); err != nil {
		t.Fatal(err)
	}
	assertStarted(2)
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
	// A fresh reusable player must keep the first item's seek local to that item.
	wave := make([]byte, 44+8000*2*10)
	copy(wave, "RIFF")
	binary.LittleEndian.PutUint32(wave[4:], uint32(len(wave)-8))
	copy(wave[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(wave[16:], 16)
	binary.LittleEndian.PutUint16(wave[20:], 1)
	binary.LittleEndian.PutUint16(wave[22:], 1)
	binary.LittleEndian.PutUint32(wave[24:], 8000)
	binary.LittleEndian.PutUint32(wave[28:], 16000)
	binary.LittleEndian.PutUint16(wave[32:], 2)
	binary.LittleEndian.PutUint16(wave[34:], 16)
	copy(wave[36:], "data")
	binary.LittleEndian.PutUint32(wave[40:], uint32(len(wave)-44))
	waveA, waveB := filepath.Join(dir, "a.wav"), filepath.Join(dir, "b.wav")
	for _, path := range []string{waveA, waveB} {
		if err := os.WriteFile(path, wave, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	reusable := &playerSession{}
	t.Cleanup(reusable.Reset)
	if err := reusable.PlayPlaylist([]PlaylistItem{
		{Path: waveA, Options: PlayOptions{StartTimeSec: 2, DataDir: dir, VideoID: 700}},
		{Path: waveB, Options: PlayOptions{DataDir: dir, VideoID: 701}},
	}); err != nil {
		t.Fatal(err)
	}
	reuseEndpoint := reusable.ipcPath
	args, err = os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(args), "--start=") {
		t.Fatal("reusable process inherited a global start position")
	}
	deadline = time.Now().Add(5 * time.Second)
	for {
		if position, ok := readMPVTestProperty(t, reuseEndpoint, "time-pos").(float64); ok && position >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("first entry did not honor its start position")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := runIPCCommand(reuseEndpoint, []any{"playlist-play-index", 1}); err != nil {
		t.Fatal(err)
	}
	waitMPVTestProperty(t, reuseEndpoint, "path", waveB)
	waitMPVTestProperty(t, reuseEndpoint, "screenshot-directory", filepath.Join(dir, "video", "701", "screenshot"))
	deadline = time.Now().Add(5 * time.Second)
	for {
		if position, ok := readMPVTestProperty(t, reuseEndpoint, "time-pos").(float64); ok {
			if position >= 1 {
				t.Fatalf("second entry inherited start: %v", position)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("second entry did not start")
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
