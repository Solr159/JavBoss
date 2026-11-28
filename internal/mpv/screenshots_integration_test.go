//go:build !windows

package mpv

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"javboss/internal/video/screenshot"
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
    local markers, move_to, line_to = 0, drawing.move_to, drawing.line_to
    local widths, positions, start_x = {}, {}, nil
    drawing.move_to = function(self, x, y)
        markers = markers + 1
        start_x = x
        positions[#positions + 1] = x * test_width / osc_param.displayresx
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
        drawing = drawing.text, markers = markers, tooltip = tooltip,
        chapters = el.slider.markerF(), widths = widths, positions = positions,
        pointer_tooltip = el.slider.tooltipF(get_slider_value(el)),
        pointer_second = get_slider_value(el) * duration / 100,
        preview_second = state.sliderpos * duration / 100,
        hit = mouse_hit(el),
        delete_pending = screenshot_delete_pending ~= nil,
        render_width = osc_param.displayresx, render_height = osc_param.playresy,
        screen_width = test_width, screen_height = test_height,
        slider_left = el.slider.min.glob_pos, slider_right = el.slider.max.glob_pos,
        track_bottom = el.hitbox.y2 - el.layout.slider.gap, duration = duration,
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
	wave := make([]byte, 44+8000*2*90)
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
	var failDeletion atomic.Bool
	var deleteCalls atomic.Int32
	deleter := func(ctx context.Context, second float64) error {
		deleteCalls.Add(1)
		if failDeletion.Load() {
			return errors.New("test deletion failure")
		}
		names, err := screenshot.PlaybackNamesAt(dir, 1, second)
		if err != nil {
			return err
		}
		for _, name := range names {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := os.Remove(filepath.Join(shots, name)); err != nil {
				return err
			}
		}
		return nil
	}
	if err := s.PlayPlaylist([]PlaylistItem{
		{Path: path, Options: PlayOptions{DataDir: dir, VideoID: 1, DeleteScreenshot: deleter}},
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
		if value["markers"] != float64(2) || !strings.Contains(value["drawing"].(string), "E8D658") {
			t.Fatalf("expected two cyan markers including zero, at %v: %v", size, value)
		}
		assertScreenshotMarkerPixels(t, value, false)
		if chapters, _ := value["chapters"].([]any); len(chapters) != 2 {
			t.Fatalf("screenshot markers changed chapter rendering: %v", value)
		}
		// Above and on the track (including its lower edge), nearby screenshots
		// must not capture the hover preview or the normal click position.
		for _, region := range []string{"above", "edge", "track"} {
			command("script-message-to", "modernz", "test-screenshots-pointer", "2.5", "2", region, "move")
			value = inspect(size[0], size[1])
			if strings.Contains(value["pointer_tooltip"].(string), "Screenshot") || strings.Contains(value["drawing"].(string), "1c&H00E6FF&") {
				t.Fatalf("screenshot captured %s hover at %v: %v", region, size, value)
			}
			if value["preview_second"] != value["pointer_second"] || value["preview_second"] == 2.5 {
				t.Fatalf("preview snapped to screenshot in %s: %v", region, value)
			}
		}
		wantTime := value["pointer_second"].(float64)
		command("script-message-to", "modernz", "test-screenshots-pointer", "2.5", "2", "track", "click")
		waitPosition(wantTime)
		// Hit the padded area five screen pixels to the side and two below
		// the original seekbar. The real OSC event dispatcher must accept it.
		command("script-message-to", "modernz", "test-screenshots-pointer", "2.5", "5", "2", "move")
		value = inspect(size[0], size[1])
		if value["hit"] != true || value["preview_second"] != 2.5 || !strings.Contains(value["pointer_tooltip"].(string), "00:00:02.500") || !strings.Contains(value["drawing"].(string), "1c&H00E6FF&") {
			t.Fatalf("missing padded hover feedback at %v: %v", size, value)
		}
		widths := value["widths"].([]any)
		for i, width := range widths {
			pixels := width.(float64) * value["screen_width"].(float64) / value["render_width"].(float64)
			want := 1.0
			if i == len(widths)-1 {
				want = 2
			}
			if math.Abs(pixels-want) > 0.001 {
				t.Fatalf("marker width=%v screen pixels, want %v", pixels, want)
			}
		}
		assertScreenshotMarkerPixels(t, value, true)
		// Outside the tighter radius, preview the pointer's time without a
		// screenshot highlight, even in the strip below the progress bar.
		command("script-message-to", "modernz", "test-screenshots-pointer", "2.5", "8", "2", "move")
		value = inspect(size[0], size[1])
		if strings.Contains(value["drawing"].(string), "1c&H00E6FF&") || strings.Contains(value["pointer_tooltip"].(string), "Screenshot") || value["preview_second"] != value["pointer_second"] {
			t.Fatalf("screenshot still captured pointer eight pixels away: %v", value)
		}
		command("script-message-to", "modernz", "test-screenshots-pointer", "2.5", "2", "2", "click")
		waitPosition(2.5)
		command("script-message-to", "modernz", "test-screenshots-pointer", "2.5", "2", "2", "leave")
		value = inspect(size[0], size[1])
		if strings.Contains(value["drawing"].(string), "1c&H00E6FF&") {
			t.Fatal("hover highlight remained after leaving")
		}
	}
	// Every timestamp must be drawn even when several share a screen pixel,
	// including neighbors of the hovered marker.
	denseNames := []string{"mpv_00-00-02.501.jpg", "mpv_00-00-02.600.jpg", "mpv_00-00-02.700.jpg"}
	for _, name := range denseNames {
		writeShot(name)
	}
	waitTimes([]any{float64(0), 2.5, 2.501, 2.6, 2.7, float64(120)})
	for _, action := range []string{"leave", "move"} {
		command("script-message-to", "modernz", "test-screenshots-pointer", "2.6", "0", "2", action)
		value := inspect("960", "540")
		if positions := value["positions"].([]any); len(positions) != 5 {
			t.Fatalf("dense markers were skipped: %v", positions)
		}
		if action == "move" && !strings.Contains(value["pointer_tooltip"].(string), "00:00:02.600") {
			t.Fatalf("dense timestamp lost its hover target: %v", value)
		}
	}
	for _, name := range denseNames {
		if err := os.Remove(filepath.Join(shots, name)); err != nil {
			t.Fatal(err)
		}
	}
	waitTimes([]any{float64(0), 2.5, float64(120)})
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
	command("script-message-to", "modernz", "test-screenshots-pointer", "2.5", "0", "track", "right")
	waitPosition(0)
	if deleteCalls.Load() != 0 {
		t.Fatal("right-click on the track deleted a screenshot instead of navigating chapters")
	}
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
	command("seek", 20, "absolute+exact")
	waitPosition(20)
	failDeletion.Store(true)
	command("script-message-to", "modernz", "test-screenshots-pointer", "2.5", "2", "2", "right")
	deleteDeadline := time.Now().Add(3 * time.Second)
	for {
		value = inspect("960", "540")
		if deleteCalls.Load() == 1 && value["delete_pending"] == false {
			break
		}
		if time.Now().After(deleteDeadline) {
			t.Fatal("failed deletion did not acknowledge the request")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !reflect.DeepEqual(value["times"], first["times"]) {
		t.Fatal("failed deletion removed the marker")
	}
	failDeletion.Store(false)
	command("script-message-to", "modernz", "test-screenshots-pointer", "2.5", "2", "2", "right")
	waitTimes([]any{float64(0), float64(120)})
	if deleteCalls.Load() != 2 {
		t.Fatalf("delete calls=%d, want 2", deleteCalls.Load())
	}
	for _, name := range []string{"mpv_00-00-02.500.jpg", "mpv_00-00-02.500.png"} {
		if _, err := os.Stat(filepath.Join(shots, name)); !os.IsNotExist(err) {
			t.Fatalf("right-click did not delete %s: %v", name, err)
		}
	}
	// Exercise a fractional timestamp beyond a minute through JSON and Lua.
	writeShot("mpv_00-01-01.029.jpg")
	waitTimes([]any{float64(0), 61.029, float64(120)})
	command("script-message-to", "modernz", "test-screenshots-pointer", "61.029", "0", "2", "right")
	waitTimes([]any{float64(0), float64(120)})
	if _, err := os.Stat(filepath.Join(shots, "mpv_00-01-01.029.jpg")); !os.IsNotExist(err) {
		t.Fatalf("fractional timestamp was not deleted: %v", err)
	}
	waitPosition(20)
	command("stop")
	waitTimes(nil)
}

// Render the actual ASS with libass, rather than assuming its position tags
// behave like the Lua geometry. Duplicate \pos tags used to pass hit tests
// while placing the visible markers far away from their clickable positions.
func assertScreenshotMarkerPixels(t *testing.T, value map[string]any, hovered bool) {
	t.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Log("ffmpeg unavailable; skipping ASS pixel verification")
		return
	}
	dir := t.TempDir()
	resX, resY := int(math.Round(value["render_width"].(float64))), int(math.Round(value["render_height"].(float64)))
	w, h := int(value["screen_width"].(float64)), int(value["screen_height"].(float64))
	sx, sy := float64(w)/float64(resX), float64(h)/float64(resY)
	ass := fmt.Sprintf(`[Script Info]
ScriptType: v4.00+
PlayResX: %d
PlayResY: %d
[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Default,Arial,16,&H00FFFFFF,&H00FFFFFF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,0,0,7,0,0,0,1
[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
`, resX, resY)
	for _, line := range strings.Split(value["drawing"].(string), "\n") {
		if strings.TrimSpace(line) != "" {
			ass += "Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,," + line + "\n"
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "markers.ass"), []byte(ass), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", fmt.Sprintf("color=c=black:s=%dx%d:d=1", w, h), "-vf", "format=rgb24,ass=markers.ass", "-frames:v", "1", "-threads", "1", "markers.png")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("render marker ASS: %v\n%s", err, output)
	}
	f, err := os.Open(filepath.Join(dir, "markers.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	pixels, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	left, right := value["slider_left"].(float64), value["slider_right"].(float64)
	xs := []float64{left, left + (right-left)*2.5/value["duration"].(float64)}
	for i, x := range xs {
		xs[i] = math.Floor(x*sx) + 0.5
	}
	y := (value["track_bottom"].(float64) + 1) * sy
	counts := [2]int{}
	for py := 0; py < h; py++ {
		for px := 0; px < w; px++ {
			r, g, b, _ := pixels.At(px, py).RGBA()
			cyan := g > 2500 && float64(g) > float64(r)*1.6 && float64(b) > float64(r)*1.6
			highlight := r > 2500 && float64(g) > float64(r)*0.6 && float64(b) < float64(r)*0.4
			if !cyan && !highlight {
				continue
			}
			matched := false
			for i, x := range xs {
				isHovered := hovered && i == 1
				halfWidth, height := 0.5, 9.0*sy
				if isHovered {
					halfWidth = 1
					height = 11 * sy
				}
				if highlight == isHovered && math.Abs(float64(px)+0.5-x) <= halfWidth+1 && float64(py)+0.5 >= y-1 && float64(py)+0.5 <= y+height+1 {
					counts[i]++
					matched = true
					break
				}
			}
			if !matched {
				t.Fatalf("marker pixel (%d,%d) is outside expected lines at x=%v, y=%v (hovered=%v)", px, py, xs, y, hovered)
			}
		}
	}
	if counts[0] == 0 || counts[1] == 0 {
		t.Fatalf("missing marker pixels at x=%v, y=%v: %v", xs, y, counts)
	}
}
