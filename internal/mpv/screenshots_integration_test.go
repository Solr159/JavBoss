//go:build !windows

package mpv

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestScreenshotMarkersWithMPV(t *testing.T) {
	if os.Getenv("JAVBOSS_TEST_MPV") == "" {
		t.Skip("set JAVBOSS_TEST_MPV for real player tests")
	}
	// Player executable and session asset paths are cached process-wide.
	if os.Getenv("JAVBOSS_TEST_SCREENSHOTS_CHILD") == "" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestScreenshotMarkersWithMPV$", "-test.timeout=40s")
		cmd.Env = append(os.Environ(), "JAVBOSS_TEST_SCREENSHOTS_CHILD=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("screenshot integration: %v\n%s", err, output)
		}
		return
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "mpv.log")
	t.Setenv("JAVBOSS_TEST_SCREENSHOT_LOG", logPath)
	t.Cleanup(func() {
		if t.Failed() {
			data, _ := os.ReadFile(logPath)
			t.Log(string(data))
		}
	})
	wrapper := filepath.Join(dir, "mpv-test")
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\nexec \"$JAVBOSS_TEST_MPV\" \"$@\" --vo=null --ao=null --save-position-on-quit=no --log-file=\"$JAVBOSS_TEST_SCREENSHOT_LOG\"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MPV_PATH", wrapper)
	assets := t.TempDir()
	for _, asset := range modernZRequiredFiles {
		data, err := os.ReadFile(filepath.Join("../../modernz", asset.source))
		if err != nil {
			t.Fatal(err)
		}
		if asset.source == "modernz.lua" {
			// Supply display dimensions in headless mode. All layout, drawing,
			// message handling and playback events use the production script.
			data = []byte(`local test_width, test_height, test_mouse_x, test_mouse_y = 960, 540, -1, -1
mp.get_osd_size = function() return test_width, test_height, test_width / test_height end
mp.get_mouse_pos = function() return test_mouse_x, test_mouse_y end
` + string(data) + `
mp.register_script_message("test-screenshots-inspect", function(width, height)
    test_width, test_height = tonumber(width), tonumber(height)
    osc_init()
    local el = state.seekbar_element
    local drawing = assdraw.ass_new()
    local triangles, move_to, line_to = 0, drawing.move_to, drawing.line_to
    local widths, start_x = {}, nil
    drawing.move_to = function(self, x, y)
        triangles = triangles + 1
        start_x = x
        move_to(self, x, y)
    end
    drawing.line_to = function(self, x, y)
        if start_x then widths[#widths + 1] = x - start_x; start_x = nil end
        line_to(self, x, y)
    end
    draw_screenshot_markers(el, drawing)
    local duration = mp.get_property_number("duration", 0)
    state.sliderpos = -1
    render_elements(assdraw.ass_new(), true, false)
    local tooltip = duration > 0 and el.slider.tooltipF(2.5 / duration * 100) or ""
    mp.set_property_native("user-data/javboss-test/screenshots", {
        times = state.screenshot_markers, generation = screenshot_generation,
        entry = screenshot_entry(), loaded = screenshots_loaded,
        drawing = drawing.text, triangles = triangles, tooltip = tooltip,
        chapters = el.slider.markerF(), widths = widths,
        pointer_tooltip = el.slider.tooltipF(get_slider_value(el)),
        pointer_second = get_slider_value(el) * duration / 100,
        preview_second = state.sliderpos * duration / 100,
        hit = mouse_hit(el),
    })
end)
mp.register_script_message("test-screenshots-pointer", function(second, dx, dy, action)
    local el = state.seekbar_element
    local sx, sy = get_virt_scale_factor()
    local duration = mp.get_property_number("duration", 0)
    test_mouse_x = (el.hitbox.x1 + get_slider_ele_pos_for(el, tonumber(second) / duration * 100)) / sx + tonumber(dx)
    if dy == "track" then
        test_mouse_y = (el.hitbox.y1 + el.hitbox.y2) / 2 / sy
    elseif dy == "edge" then
        test_mouse_y = (el.hitbox.y2 - el.layout.slider.gap) / sy
    elseif dy == "above" then
        test_mouse_y = (el.hitbox.y1 + el.layout.slider.gap - 1) / sy
    else
        test_mouse_y = el.hitbox.y2 / sy + tonumber(dy)
    end
    if action == "leave" then state.mouse_in_window = false; request_tick(); return end
    state.mouse_in_window = true
    if action == "click" then
        process_event("mbtn_left", "down")
        process_event("mbtn_left", "up")
    elseif action == "right" then
        process_event("mbtn_right", "down")
        process_event("mbtn_right", "up")
    elseif action == "move" then
        process_event("mouse_move")
    else
        process_event("mbtn_left", action)
    end
end)
`)
		}
		if err := os.WriteFile(filepath.Join(assets, asset.source), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(modernZEnvDir, assets)
	wave := make([]byte, 44+8000*2*60)
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
	path := filepath.Join(dir, "video.wav")
	if err := os.WriteFile(path, wave, 0600); err != nil {
		t.Fatal(err)
	}
	shots := filepath.Join(dir, "video", "1", "screenshot")
	if err := os.MkdirAll(shots, 0700); err != nil {
		t.Fatal(err)
	}
	writeShot := func(name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(shots, name), []byte("image"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeShot("mpv_00-00-00.jpg")
	writeShot("mpv_00-00-02.500.jpg")
	writeShot("mpv_00-00-02.500.png")
	writeShot("mpv_00-02-00.jpg") // Outside this file's duration: never draw it.
	writeShot("128.jpg")
	s := &playerSession{}
	t.Cleanup(s.Reset)
	if err := s.PlayPlaylist([]PlaylistItem{
		{Path: path, Options: PlayOptions{DataDir: dir, VideoID: 1}},
		{Path: path, Options: PlayOptions{DataDir: dir, VideoID: 2}},
	}); err != nil {
		t.Fatal(err)
	}
	command := func(args ...any) {
		t.Helper()
		if err := runIPCCommand(s.ipcPath, args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	command("set_property", "pause", true)
	waitPosition := func(want float64) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for {
			pos, _ := readMPVTestProperty(t, s.ipcPath, "time-pos").(float64)
			// The WAV fixture can report the preceding audio sample (0.125 ms).
			if math.Abs(pos-want) < 0.001 {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("time-pos=%v, want %v within 1 ms", pos, want)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	inspect := func(width, height string) map[string]any {
		t.Helper()
		command("set_property", "user-data/javboss-test/screenshots", nil)
		command("script-message-to", "modernz", "test-screenshots-inspect", width, height)
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if value, ok := readMPVTestProperty(t, s.ipcPath, "user-data/javboss-test/screenshots").(map[string]any); ok {
				return value
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("ModernZ did not respond to inspection")
		return nil
	}
	waitTimes := func(want []any) map[string]any {
		t.Helper()
		deadline := time.Now().Add(6 * time.Second)
		var value map[string]any
		for time.Now().Before(deadline) {
			value = inspect("960", "540")
			got, _ := value["times"].([]any)
			if len(got) == len(want) && (len(want) == 0 || reflect.DeepEqual(got, want)) {
				return value
			}
			time.Sleep(40 * time.Millisecond)
		}
		t.Fatalf("markers=%v, want=%v", value, want)
		return nil
	}
	first := waitTimes([]any{float64(0), 2.5, float64(120)})
	command("set_property", "chapter-list", []map[string]any{{"time": 0, "title": "Intro"}, {"time": 10, "title": "Chapter 2"}})
	// Property observers and script messages are dispatched independently.
	chapterDeadline := time.Now().Add(3 * time.Second)
	for {
		value := inspect("960", "540")
		if chapters, _ := value["chapters"].([]any); len(chapters) == 2 {
			break
		}
		if time.Now().After(chapterDeadline) {
			t.Fatal("ModernZ did not observe the test chapters")
		}
		time.Sleep(10 * time.Millisecond)
	}
	for _, size := range [][2]string{{"480", "360"}, {"960", "540"}, {"1920", "1080"}} {
		value := inspect(size[0], size[1])
		if value["triangles"] != float64(2) || !strings.Contains(value["drawing"].(string), "E8D658") {
			t.Fatalf("expected two cyan markers including zero, at %v: %v", size, value)
		}
		if chapters, _ := value["chapters"].([]any); len(chapters) != 2 {
			t.Fatalf("screenshot markers changed chapter rendering: %v", value)
		}
		// Above and on the track (including its lower edge), nearby screenshots
		// must not capture the hover preview or the normal click position.
		for _, region := range []string{"above", "edge", "track"} {
			command("script-message-to", "modernz", "test-screenshots-pointer", "2.5", "8", region, "move")
			value = inspect(size[0], size[1])
			if strings.Contains(value["pointer_tooltip"].(string), "Screenshot") || strings.Contains(value["drawing"].(string), "1c&HFFFFFF&") {
				t.Fatalf("screenshot captured %s hover at %v: %v", region, size, value)
			}
			if value["preview_second"] != value["pointer_second"] || value["preview_second"] == 2.5 {
				t.Fatalf("preview snapped to screenshot in %s: %v", region, value)
			}
		}
		wantTime := value["pointer_second"].(float64)
		command("script-message-to", "modernz", "test-screenshots-pointer", "2.5", "8", "track", "click")
		waitPosition(wantTime)
		// Hit the padded area eight screen pixels to the side and two below
		// the original seekbar. The real OSC event dispatcher must accept it.
		command("script-message-to", "modernz", "test-screenshots-pointer", "2.5", "8", "2", "move")
		value = inspect(size[0], size[1])
		if value["hit"] != true || value["preview_second"] != 2.5 || !strings.Contains(value["pointer_tooltip"].(string), "00:00:02.500") || !strings.Contains(value["drawing"].(string), "1c&HFFFFFF&") {
			t.Fatalf("missing padded hover feedback at %v: %v", size, value)
		}
		widths := value["widths"].([]any)
		if widths[len(widths)-1] != float64(12) {
			t.Fatalf("hovered marker was not enlarged: %v", widths)
		}
		command("script-message-to", "modernz", "test-screenshots-pointer", "2.5", "8", "2", "click")
		waitPosition(2.5)
		command("script-message-to", "modernz", "test-screenshots-pointer", "2.5", "8", "2", "leave")
		value = inspect(size[0], size[1])
		if strings.Contains(value["drawing"].(string), "1c&HFFFFFF&") {
			t.Fatal("hover highlight remained after leaving")
		}
	}
	// Dragging remains continuous rather than snapping back to the screenshot.
	command("script-message-to", "modernz", "test-screenshots-pointer", "2.5", "0", "0", "down")
	command("script-message-to", "modernz", "test-screenshots-pointer", "2.5", "8", "0", "move")
	command("script-message-to", "modernz", "test-screenshots-pointer", "2.5", "8", "0", "up")
	deadline := time.Now().Add(3 * time.Second)
	for {
		pos, _ := readMPVTestProperty(t, s.ipcPath, "time-pos").(float64)
		if pos > 2.55 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("drag snapped back to screenshot: %v", pos)
		}
		time.Sleep(10 * time.Millisecond)
	}
	command("script-message-to", "modernz", "test-screenshots-pointer", "30", "0", "0", "click")
	waitPosition(30)
	command("script-message-to", "modernz", "test-screenshots-pointer", "2.5", "0", "0", "leave")
	writeShot("mpv_00-00-15.jpg")
	waitTimes([]any{float64(0), 2.5, float64(15), float64(120)})
	if err := os.Remove(filepath.Join(shots, "mpv_00-00-15.jpg")); err != nil {
		t.Fatal(err)
	}
	waitTimes([]any{float64(0), 2.5, float64(120)})
	command("playlist-play-index", 1)
	waitTimes(nil)
	command("script-message-to", "modernz", "javboss-screenshot-markers", fmt.Sprint(first["generation"]), fmt.Sprint(first["entry"]), "[55]")
	if got, _ := inspect("960", "540")["times"].([]any); len(got) != 0 {
		t.Fatalf("late response leaked into next video: %v", got)
	}
	command("playlist-play-index", 0)
	waitTimes([]any{float64(0), 2.5, float64(120)})
	command("script-message-to", "modernz", "javboss-screenshot-markers", fmt.Sprint(first["generation"]), fmt.Sprint(first["entry"]), "[55]")
	value := inspect("960", "540")
	if !reflect.DeepEqual(value["times"], first["times"]) {
		t.Fatalf("old generation overwrote replay markers: %v", value)
	}
	command("stop")
	waitTimes(nil)
}
