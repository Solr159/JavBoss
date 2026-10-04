//go:build !windows

package mpv

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"javboss/internal/playback"
)

func TestWatchTimeWithMPV(t *testing.T) {
	if os.Getenv("JAVBOSS_TEST_MPV") == "" {
		t.Skip("set JAVBOSS_TEST_MPV for real player tests")
	}
	if os.Getenv("JAVBOSS_TEST_WATCH_CHILD") == "" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestWatchTimeWithMPV$", "-test.timeout=30s")
		cmd.Env = append(os.Environ(), "JAVBOSS_TEST_WATCH_CHILD=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("watch integration: %v\n%s", err, output)
		}
		return
	}
	dir := t.TempDir()
	wrapper := filepath.Join(dir, "mpv-test")
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\nexec \"$JAVBOSS_TEST_MPV\" \"$@\" --vo=null --ao=null --save-position-on-quit=no\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MPV_PATH", wrapper)
	t.Setenv(modernZEnvDir, writeModernZTestAssets(t))
	wave := make([]byte, 44+8000*2*30)
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
	path := filepath.Join(dir, "sound.wav")
	if err := os.WriteFile(path, wave, 0644); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	next := 0
	totals := map[string]int64{}
	factory := func() *playback.Reporter {
		return playback.NewReporter(func(context.Context) (string, error) {
			mu.Lock()
			defer mu.Unlock()
			next++
			return fmt.Sprint(next), nil
		},
			func(_ context.Context, id string, total int64) error {
				mu.Lock()
				defer mu.Unlock()
				totals[id] = total
				return nil
			})
	}
	s := &playerSession{}
	t.Cleanup(s.Reset)
	if err := s.PlayPlaylist([]PlaylistItem{
		{Path: path, Options: PlayOptions{DataDir: dir, VideoID: 1, NewWatchReporter: factory}},
		{Path: path, Options: PlayOptions{DataDir: dir, VideoID: 2, NewWatchReporter: factory}},
	}); err != nil {
		t.Fatal(err)
	}
	waitActive := func(want bool) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for {
			s.events.mu.Lock()
			got := s.events.watch.active()
			s.events.mu.Unlock()
			if got == want {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("active=%v want=%v", got, want)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	command := func(args ...any) {
		t.Helper()
		if err := runIPCCommand(s.ipcPath, args); err != nil {
			t.Fatal(err)
		}
	}
	waitActive(true)
	time.Sleep(150 * time.Millisecond)
	command("set_property", "pause", true)
	waitActive(false)
	s.events.mu.Lock()
	paused := s.events.watch.total
	s.events.mu.Unlock()
	time.Sleep(120 * time.Millisecond)
	s.events.mu.Lock()
	s.events.watch.advance(time.Now())
	stillPaused := s.events.watch.total
	s.events.mu.Unlock()
	if stillPaused != paused {
		t.Fatal("paused time counted")
	}
	command("seek", 10, "absolute")
	command("set_property", "speed", 2)
	command("set_property", "pause", false)
	waitActive(true)
	time.Sleep(120 * time.Millisecond)
	// Holding a seek shortcut keeps restarting the decoder. Its wall time
	// still counts, even if core-idle/seeking briefly toggle on every command.
	s.events.mu.Lock()
	s.events.watch.advance(time.Now())
	beforeSeeks := s.events.watch.total
	s.events.mu.Unlock()
	seekStart := time.Now()
	for i := 0; i < 20; i++ {
		command("seek", 5+float64(i)/2, "absolute+exact")
		time.Sleep(20 * time.Millisecond)
	}
	seekElapsed := time.Since(seekStart)
	s.events.mu.Lock()
	s.events.watch.advance(time.Now())
	seekCounted := s.events.watch.total - beforeSeeks
	s.events.mu.Unlock()
	if seekCounted < seekElapsed-50*time.Millisecond || seekCounted > seekElapsed+50*time.Millisecond {
		t.Fatalf("continuous seeking counted %v of %v wall time", seekCounted, seekElapsed)
	}
	command("playlist-play-index", 1)
	waitMPVTestProperty(t, s.ipcPath, "playlist-pos", float64(1))
	waitActive(true)
	time.Sleep(150 * time.Millisecond)
	s.Reset()
	mu.Lock()
	defer mu.Unlock()
	if len(totals) != 2 {
		t.Fatalf("expected two sessions, got %v", totals)
	}
	for id, total := range totals {
		if total < 100 || total > 2000 {
			t.Fatalf("session %s total=%d; seek/speed must not add media time", id, total)
		}
	}
}
