//go:build !windows

package mpv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The real player test uses vo=null. Supply only display/mouse coordinates to
// ModernZ, while keeping its layout, hit testing, bindings and actions intact.
func subtitlePopupTestAssets(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()
	for _, asset := range modernZRequiredFiles {
		data, err := os.ReadFile(filepath.Join(source, asset.source))
		if err != nil {
			t.Fatal(err)
		}
		if asset.source == "modernz.lua" {
			data = []byte(`local test_w, test_h, test_x, test_y = 960, 540, 0, 0
mp.get_osd_size = function() return test_w, test_h, test_w / test_h end
mp.get_mouse_pos = function() return test_x, test_y end
` + string(data) + `
mp.register_script_message("test-popup-inspect", function()
    osc_init()
    local result = {open = subtitle_menu ~= nil, tooltips = user_opts.tooltip_hints, find = false, items = {}}
    for _, el in ipairs(elements) do
        if el.name == "sub_track" then result.anchor = el.hitbox; result.tooltip = el.tooltipF() end
        if el.name:match("^subtitle_menu_track_") then result.items[el.name] = el.content end
        if el.name == "subtitle_menu_find" then result.find = true; result.find_label = el.content end
    end
    if subtitle_menu then
        result.bounds = subtitle_menu.bounds
        result.width = osc_param.playresx
    end
    mp.set_property_native("user-data/javboss-test/popup", result)
end)
mp.register_script_message("test-popup-size", function(w, h)
    test_w, test_h = tonumber(w), tonumber(h)
    osc_init()
end)
mp.register_script_message("test-popup-click", function(target)
    local x, y = 10, 10
    for _, el in ipairs(elements) do
        if el.name == target then
            x, y = (el.hitbox.x1 + el.hitbox.x2) / 2, (el.hitbox.y1 + el.hitbox.y2) / 2
        end
    end
    local sx, sy = get_virt_scale_factor()
    test_x, test_y = x / sx, y / sy
    state.mouse_in_window = true
    process_event("mouse_move")
    process_event("mbtn_left", "down")
    process_event("mbtn_left", "up")
end)
`)
		}
		if err := os.WriteFile(filepath.Join(dir, asset.source), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func testAnchoredSubtitlePopup(t *testing.T, endpoint string) {
	t.Helper()
	command := func(args ...any) {
		t.Helper()
		if err := runIPCCommand(endpoint, args); err != nil {
			t.Fatal(err)
		}
	}
	inspect := func(open bool) map[string]any {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			command("script-message-to", "modernz", "test-popup-inspect")
			time.Sleep(25 * time.Millisecond)
			value := readMPVTestProperty(t, endpoint, "user-data/javboss-test/popup").(map[string]any)
			if value["open"] == open {
				if value["tooltips"] != true {
					t.Fatal("default hover hints were disabled")
				}
				if tooltip, _ := value["tooltip"].(string); strings.Contains(tooltip, "F9") || strings.Contains(tooltip, "Click:") {
					t.Fatalf("subtitle tooltip still contains the added shortcut instructions: %s", tooltip)
				}
				return value
			}
		}
		t.Fatalf("popup did not reach open=%v", open)
		return nil
	}
	click := func(target string) { command("script-message-to", "modernz", "test-popup-click", target) }
	command("set_property", "pause", true)
	inspect(false)
	click("sub_track")
	inspect(true)
	for _, size := range [][2]string{{"960", "540"}, {"480", "360"}, {"1920", "1080"}} {
		command("script-message-to", "modernz", "test-popup-size", size[0], size[1])
		info := inspect(true)
		bounds, anchor := info["bounds"].(map[string]any), info["anchor"].(map[string]any)
		if bounds["y2"].(float64) >= anchor["y1"].(float64) || bounds["x1"].(float64) < 0 || bounds["x2"].(float64) > info["width"].(float64) {
			t.Fatalf("popup is not anchored above the button within the window: %v", info)
		}
	}
	click("sub_track")
	inspect(false)
	position := readMPVTestProperty(t, endpoint, "time-pos")
	click("sub_track")
	inspect(true)
	click("outside")
	inspect(false)
	waitMPVTestProperty(t, endpoint, "time-pos", position)
	click("sub_track")
	inspect(true)
	command("keypress", "ESC")
	inspect(false)
	waitMPVTestProperty(t, endpoint, "playlist-pos", float64(1))

	selected := readMPVTestProperty(t, endpoint, "sid")
	click("sub_track")
	inspect(true)
	click("subtitle_menu_off")
	inspect(false)
	waitMPVTestProperty(t, endpoint, "sid", false)
	click("sub_track")
	inspect(true)
	click("subtitle_menu_track_1")
	inspect(false)
	waitMPVTestProperty(t, endpoint, "sid", selected)
	if readMPVTestProperty(t, endpoint, "user-data/mpv/console/open") == true {
		t.Fatal("track selection opened a separate dialog")
	}
	click("sub_track")
	inspect(true)
	click("subtitle_menu_offset")
	inspect(false)
	waitMPVTestProperty(t, endpoint, "user-data/javboss/subtitle-offset-open", true)
	command("keypress", "ESC")
	waitMPVTestProperty(t, endpoint, "user-data/javboss/subtitle-offset-open", false)
	click("sub_track")
	inspect(true)
	command("playlist-play-index", 2)
	waitMPVTestProperty(t, endpoint, "playlist-pos", float64(2))
	inspect(false)
	command("playlist-play-index", 1)
	waitMPVTestProperty(t, endpoint, "sub-delay", 30.5)
	waitMPVTestProperty(t, endpoint, "user-data/javboss/subtitle-status", "ready")
}
