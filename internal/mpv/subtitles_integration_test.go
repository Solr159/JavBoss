//go:build !windows

package mpv

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestAutomaticSubtitleWithMPV(t *testing.T) {
	if os.Getenv("JAVBOSS_TEST_MPV") == "" {
		t.Skip("set JAVBOSS_TEST_MPV for real player tests")
	}
	if os.Getenv("JAVBOSS_TEST_SUBTITLE_CHILD") == "" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestAutomaticSubtitleWithMPV$", "-test.timeout=30s")
		cmd.Env = append(os.Environ(), "JAVBOSS_TEST_SUBTITLE_CHILD=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("subtitle integration: %v\n%s", err, output)
		}
		return
	}
	dir := t.TempDir()
	wrapper := filepath.Join(dir, "mpv-test")
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\nexec \"$JAVBOSS_TEST_MPV\" \"$@\" --vo=null --ao=null --save-position-on-quit=no\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MPV_PATH", wrapper)
	assetsDir, err := filepath.Abs("../../modernz")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(modernZEnvDir, subtitlePopupTestAssets(t, assetsDir))
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
	if err := os.WriteFile(path, wave, 0600); err != nil {
		t.Fatal(err)
	}
	srt := filepath.Join(dir, "download.srt")
	if err := os.WriteFile(srt, []byte("1\n00:00:01,000 --> 00:00:20,000\nTest\n"), 0600); err != nil {
		t.Fatal(err)
	}
	localVideo := filepath.Join(dir, "local.wav")
	if err := os.WriteFile(localVideo, wave, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "local.srt"), []byte("1\n00:00:01,000 --> 00:00:20,000\nLocal\n"), 0600); err != nil {
		t.Fatal(err)
	}
	started, release := make(chan struct{}), make(chan struct{})
	saved := make(chan int64, 8)
	var storedMS atomic.Int64
	var manualLookups atomic.Int64
	var storedEnabled, freshDownload atomic.Bool
	var selectedTrack atomic.Value
	selectedTrack.Store("download:zh-CN")
	visibilitySaved := make(chan bool, 16)
	storedMS.Store(30500)
	defer close(release)
	s := &playerSession{}
	t.Cleanup(s.Reset)
	if err := s.PlayPlaylist([]PlaylistItem{
		{Path: path, Options: PlayOptions{ResolveSubtitle: func(ctx context.Context, request SubtitleRequest) (*SubtitleFile, error) {
			if request.SettingsOnly {
				return &SubtitleFile{}, nil
			}
			close(started)
			select {
			case <-release:
			case <-ctx.Done():
			}
			return &SubtitleFile{Path: srt, Title: "old", Language: "zh-CN"}, nil
		}}},
		{Path: path, Options: PlayOptions{
			ResolveSubtitle: func(_ context.Context, request SubtitleRequest) (*SubtitleFile, error) {
				file := &SubtitleFile{OffsetSec: float64(storedMS.Load()) / 1000}
				if !request.SettingsOnly {
					file.Path, file.Title, file.Language = srt, "current", "zh-CN"
				}
				return file, nil
			},
			SaveSubtitleOffset: func(_ context.Context, ms int64) error {
				if ms != 20000 {
					storedMS.Store(ms)
				}
				saved <- ms
				if ms == 20000 {
					return fmt.Errorf("test save failure")
				}
				return nil
			},
		}},
		{Path: path},
		{Path: path, Options: PlayOptions{
			ResolveSubtitle: func(ctx context.Context, request SubtitleRequest) (*SubtitleFile, error) {
				if request.SettingsOnly {
					return &SubtitleFile{Status: "disabled"}, nil
				}
				if !request.Manual {
					return &SubtitleFile{Status: "disabled"}, nil
				}
				manualLookups.Add(1)
				select {
				case <-time.After(150 * time.Millisecond):
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				return &SubtitleFile{Path: srt, Language: "zh-CN", Status: "ready"}, nil
			},
			SaveSubtitleOffset: func(context.Context, int64) error { return nil },
		}},
		{Path: path, Options: PlayOptions{ResolveSubtitle: func(_ context.Context, request SubtitleRequest) (*SubtitleFile, error) {
			if request.SettingsOnly {
				return &SubtitleFile{Status: "disabled"}, nil
			}
			if request.Manual {
				return nil, fmt.Errorf("expected automatic cache load")
			}
			return &SubtitleFile{Path: srt, Language: "zh-CN", Status: "ready"}, nil
		}}},
		{Path: path, Options: PlayOptions{
			ResolveSubtitle: func(_ context.Context, request SubtitleRequest) (*SubtitleFile, error) {
				file := &SubtitleFile{Track: "off"}
				if storedEnabled.Load() {
					file.Track = "download:zh-CN"
				}
				if !request.SettingsOnly {
					file.Path, file.Language, file.Status, file.Key = srt, "zh-CN", "ready", "download:zh-CN"
					file.Downloaded = freshDownload.Load()
				}
				return file, nil
			},
			SaveSubtitleSelection: func(_ context.Context, track string) error {
				storedEnabled.Store(track != "off")
				visibilitySaved <- track != "off"
				return nil
			},
		}},
		{Path: localVideo, Options: PlayOptions{
			ResolveSubtitle: func(_ context.Context, request SubtitleRequest) (*SubtitleFile, error) {
				file := &SubtitleFile{Track: selectedTrack.Load().(string)}
				if request.SettingsOnly {
					return file, nil
				}
				if !request.Manual && (!request.CacheOnly || !request.DownloadOnly) {
					return nil, fmt.Errorf("local track should prevent automatic download")
				}
				file.Path, file.Title, file.Language, file.Status, file.Key = srt, "subtitlecat", "zh-CN", "ready", "download:zh-CN"
				return file, nil
			},
			SaveSubtitleSelection: func(_ context.Context, key string) error {
				selectedTrack.Store(key)
				return nil
			},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("subtitle resolver not called")
	}
	waitMPVTestProperty(t, s.ipcPath, "user-data/javboss/subtitle-status", "searching")
	if err := runIPCCommand(s.ipcPath, []any{"playlist-play-index", 1}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, err := queryIPCCommand(s.ipcPath, []any{"get_property", "track-list"})
		var tracks []struct {
			Type, Title string
			Selected    bool
		}
		if err == nil && json.Unmarshal(data, &tracks) == nil {
			for _, track := range tracks {
				if track.Type == "sub" {
					if track.Title != "current" || !track.Selected {
						t.Fatalf("wrong subtitle: %+v", track)
					}
					waitMPVTestProperty(t, s.ipcPath, "sub-delay", 30.5)
					waitMPVTestProperty(t, s.ipcPath, "user-data/javboss/subtitle-status", "ready")
					testAnchoredSubtitlePopup(t, s.ipcPath)
					testLiveSubtitleEditing(t, s.ipcPath, saved, &storedMS)
					if err := runIPCCommand(s.ipcPath, []any{"playlist-play-index", 2}); err != nil {
						t.Fatal(err)
					}
					waitMPVTestProperty(t, s.ipcPath, "playlist-pos", float64(2))
					waitMPVTestProperty(t, s.ipcPath, "sub-delay", float64(0))
					waitMPVTestProperty(t, s.ipcPath, "user-data/javboss/subtitle-status", "")
					if err := runIPCCommand(s.ipcPath, []any{"playlist-play-index", 3}); err != nil {
						t.Fatal(err)
					}
					waitMPVTestProperty(t, s.ipcPath, "playlist-pos", float64(3))
					time.Sleep(100 * time.Millisecond)
					if manualLookups.Load() != 0 {
						t.Fatal("automatic lookup ran while disabled")
					}
					command := func(args ...any) {
						t.Helper()
						if err := runIPCCommand(s.ipcPath, args); err != nil {
							t.Fatal(err)
						}
					}
					command("script-message-to", "modernz", "test-popup-inspect")
					command("script-message-to", "modernz", "test-popup-click", "sub_track")
					for until := time.Now().Add(3 * time.Second); ; {
						command("script-message-to", "modernz", "test-popup-inspect")
						time.Sleep(25 * time.Millisecond)
						popup := readMPVTestProperty(t, s.ipcPath, "user-data/javboss-test/popup").(map[string]any)
						if popup["open"] == true {
							if popup["find"] != true || popup["find_label"] != "查找字幕" {
								t.Fatal("missing Find subtitle action", popup)
							}
							break
						}
						if time.Now().After(until) {
							t.Fatal("subtitle menu unavailable without tracks")
						}
					}
					command("script-message-to", "modernz", "test-popup-click", "subtitle_menu_find")
					waitMPVTestProperty(t, s.ipcPath, "user-data/javboss/subtitle-status", "searching")
					session := readMPVTestProperty(t, s.ipcPath, "user-data/javboss/subtitle-session").(map[string]any)
					command("script-message", "javboss-subtitle-action", "find", fmt.Sprint(session["generation"]), fmt.Sprint(session["entry"]))
					waitMPVTestProperty(t, s.ipcPath, "user-data/javboss/subtitle-status", "ready")
					if manualLookups.Load() != 1 {
						t.Fatal("manual click started duplicate lookups")
					}
					command("playlist-play-index", 4)
					waitMPVTestProperty(t, s.ipcPath, "playlist-pos", float64(4))
					waitMPVTestProperty(t, s.ipcPath, "user-data/javboss/subtitle-status", "ready")
					if sid := readMPVTestProperty(t, s.ipcPath, "sid"); sid == false {
						t.Fatal("cached subtitle not selected with downloads disabled")
					}

					testRememberedSubtitleVisibility(t, s.ipcPath, visibilitySaved, &freshDownload)
					testLocalAndDownloadedSubtitleSelection(t, s.ipcPath, &selectedTrack)
					return
				}
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("downloaded subtitle not loaded")
}

func testLiveSubtitleEditing(t *testing.T, endpoint string, saved <-chan int64, storedMS *atomic.Int64) {
	t.Helper()
	command := func(args ...any) {
		t.Helper()
		if err := runIPCCommand(endpoint, args); err != nil {
			t.Fatal(err)
		}
	}
	open := func() {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			command("script-message", "javboss-subtitle-offset")
			time.Sleep(25 * time.Millisecond)
			data, err := queryIPCCommand(endpoint, []any{"get_property", "user-data/javboss/subtitle-offset-open"})
			if err == nil && string(data) == "true" {
				return
			}
		}
		t.Fatal("subtitle offset action did not open the dialog")
	}
	typeText := func(text string) {
		t.Helper()
		command("keypress", "Ctrl+u")
		for _, char := range text {
			command("keypress", string(char))
		}
	}
	submit := func(value string, wantMS int64) {
		t.Helper()
		open()
		typeText(value)
		command("keypress", "ENTER")
		waitMPVTestProperty(t, endpoint, "sub-delay", float64(wantMS)/1000)
		select {
		case got := <-saved:
			if got != wantMS {
				t.Fatalf("saved %d, want %d", got, wantMS)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("MPV edit was not persisted")
		}
		waitMPVTestProperty(t, endpoint, "user-data/javboss/subtitle-offset-open", false)
	}
	command("set_property", "pause", true)
	// Exercise the native fallback menu alongside the anchored ModernZ menu.
	chooseMenu := func(index int) {
		t.Helper()
		command("script-message", "javboss-subtitle-menu")
		waitMPVTestProperty(t, endpoint, "user-data/mpv/console/open", true)
		for i := 1; i < index; i++ {
			command("keypress", "DOWN")
		}
		command("keypress", "ENTER")
		// Menu transitions cross script queues and defer the next native input
		// until the previous handler's closed callback has been unregistered.
		time.Sleep(100 * time.Millisecond)
	}
	selectedSubtitle := readMPVTestProperty(t, endpoint, "sid")
	chooseMenu(2)
	waitMPVTestProperty(t, endpoint, "sid", false)
	waitMPVTestProperty(t, endpoint, "user-data/mpv/console/open", false)
	chooseMenu(1)
	waitMPVTestProperty(t, endpoint, "sid", selectedSubtitle)
	waitMPVTestProperty(t, endpoint, "user-data/mpv/console/open", false)
	chooseMenu(3)
	waitMPVTestProperty(t, endpoint, "user-data/javboss/subtitle-offset-open", true)
	typeText("-1.5")
	command("keypress", "ENTER")
	waitMPVTestProperty(t, endpoint, "sub-delay", -1.5)
	select {
	case got := <-saved:
		if got != -1500 {
			t.Fatalf("menu saved %d, want -1500", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("menu offset was not persisted")
	}
	waitMPVTestProperty(t, endpoint, "user-data/javboss/subtitle-offset-open", false)
	command("seek", 5, "absolute+exact")
	waitMPVTestProperty(t, endpoint, "seeking", false)
	position := readMPVTestProperty(t, endpoint, "time-pos")
	submit("12.346", 12346)
	waitMPVTestProperty(t, endpoint, "pause", true)
	waitMPVTestProperty(t, endpoint, "time-pos", position)
	submit("20", 20000) // Current playback changes even if persistence fails.
	if storedMS.Load() != 12346 {
		t.Fatal("failed save changed persisted settings")
	}
	submit("0", 0) // Reopen and retry after failure.
	submit("12.346", 12346)
	open()
	// Typing must not trigger playback shortcuts, including q (quit).
	command("keypress", "q")
	command("keypress", "SPACE")
	waitMPVTestProperty(t, endpoint, "pause", true)
	typeText("3601")
	command("keypress", "ENTER")
	waitMPVTestProperty(t, endpoint, "user-data/javboss/subtitle-offset-open", true)
	waitMPVTestProperty(t, endpoint, "sub-delay", 12.346)
	typeText("99")
	command("keypress", "ESC")
	waitMPVTestProperty(t, endpoint, "user-data/javboss/subtitle-offset-open", false)
	waitMPVTestProperty(t, endpoint, "playlist-pos", float64(1))
	waitMPVTestProperty(t, endpoint, "sub-delay", 12.346)
	select {
	case <-saved:
		t.Fatal("cancelled input was saved")
	default:
	}
	// Reopening restores the saved duration; switching during input closes it.
	command("playlist-play-index", 2)
	waitMPVTestProperty(t, endpoint, "sub-delay", float64(0))
	command("playlist-play-index", 1)
	waitMPVTestProperty(t, endpoint, "sub-delay", 12.346)
	open()
	command("playlist-play-index", 2)
	waitMPVTestProperty(t, endpoint, "user-data/javboss/subtitle-offset-open", false)
	waitMPVTestProperty(t, endpoint, "sub-delay", float64(0))
}

func testRememberedSubtitleVisibility(t *testing.T, endpoint string, saved <-chan bool, fresh *atomic.Bool) {
	t.Helper()
	command := func(args ...any) {
		t.Helper()
		if err := runIPCCommand(endpoint, args); err != nil {
			t.Fatal(err)
		}
	}
	open := func() {
		t.Helper()
		command("playlist-play-index", 2)
		waitMPVTestProperty(t, endpoint, "playlist-pos", float64(2))
		waitMPVTestProperty(t, endpoint, "user-data/javboss/subtitle-status", "")
		command("playlist-play-index", 5)
		waitMPVTestProperty(t, endpoint, "playlist-pos", float64(5))
		waitMPVTestProperty(t, endpoint, "user-data/javboss/subtitle-status", "ready")
	}
	waitSaved := func(want bool) {
		t.Helper()
		select {
		case got := <-saved:
			if got != want {
				t.Fatalf("saved visibility %v, want %v", got, want)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("visibility not saved")
		}
	}
	open()
	waitMPVTestProperty(t, endpoint, "sid", false)
	select {
	case got := <-saved:
		t.Fatalf("restore unexpectedly saved %v", got)
	default:
	}
	command("set_property", "sid", 1)
	waitSaved(true)
	open()
	waitMPVTestProperty(t, endpoint, "sid", float64(1))
	session := readMPVTestProperty(t, endpoint, "user-data/javboss/subtitle-session").(map[string]any)
	command("script-message", "javboss-subtitle-action", "off", fmt.Sprint(session["generation"]), fmt.Sprint(session["entry"]))
	waitMPVTestProperty(t, endpoint, "sid", false)
	waitSaved(false)
	open()
	waitMPVTestProperty(t, endpoint, "sid", false)
	fresh.Store(true)
	open()
	waitMPVTestProperty(t, endpoint, "sid", float64(1))
	waitSaved(true)
}

func testLocalAndDownloadedSubtitleSelection(t *testing.T, endpoint string, stored *atomic.Value) {
	t.Helper()
	command := func(args ...any) {
		t.Helper()
		if err := runIPCCommand(endpoint, args); err != nil {
			t.Fatal(err)
		}
	}
	open := func() {
		t.Helper()
		command("playlist-play-index", 2)
		waitMPVTestProperty(t, endpoint, "playlist-pos", float64(2))
		waitMPVTestProperty(t, endpoint, "user-data/javboss/subtitle-status", "")
		command("playlist-play-index", 6)
		waitMPVTestProperty(t, endpoint, "playlist-pos", float64(6))
		waitMPVTestProperty(t, endpoint, "user-data/javboss/subtitle-status", "ready")
	}
	waitSaved := func(key string) {
		t.Helper()
		until := time.Now().Add(3 * time.Second)
		for stored.Load().(string) != key {
			if time.Now().After(until) {
				t.Fatalf("saved key %q, want %q", stored.Load(), key)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	selectPopupTrack := func(id int) {
		t.Helper()
		command("script-message-to", "modernz", "javboss-subtitle-popup")
		command("script-message-to", "modernz", "test-popup-inspect")
		time.Sleep(30 * time.Millisecond)
		command("script-message-to", "modernz", "test-popup-click", fmt.Sprintf("subtitle_menu_track_%d", id))
		waitMPVTestProperty(t, endpoint, "sid", float64(id))
		if readMPVTestProperty(t, endpoint, "user-data/mpv/console/open") == true {
			t.Fatal("track selection opened a separate dialog")
		}
	}
	open()
	waitMPVTestProperty(t, endpoint, "sid", float64(2))
	selectPopupTrack(1)
	waitSaved("local:local.srt")
	open()
	waitMPVTestProperty(t, endpoint, "sid", float64(1))
	selectPopupTrack(2)
	waitSaved("download:zh-CN")
	open()
	waitMPVTestProperty(t, endpoint, "sid", float64(2))
	session := readMPVTestProperty(t, endpoint, "user-data/javboss/subtitle-session").(map[string]any)
	command("script-message", "javboss-subtitle-action", "find", fmt.Sprint(session["generation"]), fmt.Sprint(session["entry"]))
	waitMPVTestProperty(t, endpoint, "user-data/javboss/subtitle-status", "existing")
	tracks := readMPVTestProperty(t, endpoint, "track-list").([]any)
	count := 0
	for _, item := range tracks {
		if item.(map[string]any)["type"] == "sub" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("expected local + cached subtitle, got %d tracks", count)
	}
	waitMPVTestProperty(t, endpoint, "user-data/javboss/subtitle-downloaded", true)
	command("script-message-to", "modernz", "javboss-subtitle-popup")
	command("script-message-to", "modernz", "test-popup-inspect")
	time.Sleep(30 * time.Millisecond)
	popup := readMPVTestProperty(t, endpoint, "user-data/javboss-test/popup").(map[string]any)
	if popup["open"] != true || popup["find"] != false {
		t.Fatal("Find remains visible with a downloaded track", popup)
	}
	items := popup["items"].(map[string]any)
	if !strings.Contains(items["subtitle_menu_track_1"].(string), "字幕：同目录字幕") || strings.Contains(items["subtitle_menu_track_1"].(string), "local.srt") || items["subtitle_menu_track_2"].(string) != "● 字幕：已下载字幕" {
		t.Fatal("popup did not label the tracks and mark the selected download", items)
	}
	command("sub-remove", 2)
	waitMPVTestProperty(t, endpoint, "user-data/javboss/subtitle-downloaded", false)
	command("script-message-to", "modernz", "test-popup-inspect")
	time.Sleep(30 * time.Millisecond)
	popup = readMPVTestProperty(t, endpoint, "user-data/javboss-test/popup").(map[string]any)
	if popup["find"] != true || popup["find_label"] != "查找字幕" {
		t.Fatal("Find missing after removing downloaded track", popup)
	}

}
