package mpv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureModernZAssetsCopiesScriptOptionsAndFont(t *testing.T) {
	files := map[string]string{
		"modernz.lua":       "-- test lua\n",
		"modernz.conf":      "layout=modern\n",
		"modernz-icons.ttf": "test font",
		"thumbfast.lua":     "-- thumbfast lua\n",
		"thumbfast.conf":    "max_height=200\n",
	}
	sourceDir := writeModernZTestAssets(t)

	t.Setenv(modernZEnvDir, sourceDir)

	assets, err := ensureModernZAssets()
	if err != nil {
		t.Fatalf("ensureModernZAssets returned error: %v", err)
	}

	expected := map[string]string{
		filepath.Join(assets.ConfigDir, "scripts", "modernz.lua"):        files["modernz.lua"],
		filepath.Join(assets.ConfigDir, "script-opts", "modernz.conf"):   files["modernz.conf"],
		filepath.Join(assets.ConfigDir, "fonts", "modernz-icons.ttf"):    files["modernz-icons.ttf"],
		filepath.Join(assets.ConfigDir, "scripts", "thumbfast.lua"):      files["thumbfast.lua"],
		filepath.Join(assets.ConfigDir, "script-opts", "thumbfast.conf"): files["thumbfast.conf"],
		assets.ScriptPath:          files["modernz.lua"],
		assets.ThumbfastScriptPath: files["thumbfast.lua"],
	}
	for path, content := range expected {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read copied asset %s: %v", path, err)
		}
		if string(got) != content {
			t.Fatalf("expected copied asset %s to contain %q, got %q", path, content, string(got))
		}
	}
}

func TestBundledModernZEnablesFullscreenAutohide(t *testing.T) {
	sourceDir, err := findModernZSourceDir()
	if err != nil {
		t.Fatalf("find ModernZ source dir: %v", err)
	}

	config, err := os.ReadFile(filepath.Join(sourceDir, "modernz.conf"))
	if err != nil {
		t.Fatalf("read ModernZ config: %v", err)
	}
	if !strings.Contains(string(config), "fullscreen_autohide=yes\n") {
		t.Fatalf("expected bundled ModernZ config to enable fullscreen autohide")
	}
	if !strings.Contains(string(config), "osc_height=47\n") {
		t.Fatalf("expected bundled ModernZ config to use compact OSC height")
	}
	for _, expected := range []string{
		"playpause_size=22\n",
		"midbuttons_size=19\n",
		"sidebuttons_size=19\n",
	} {
		if !strings.Contains(string(config), expected) {
			t.Fatalf("expected bundled ModernZ config to contain %q", expected)
		}
	}

	script, err := os.ReadFile(filepath.Join(sourceDir, "modernz.lua"))
	if err != nil {
		t.Fatalf("read ModernZ script: %v", err)
	}
	for _, expected := range []string{
		`visibility_mode("auto", true)`,
		`mp.set_property_number("video-margin-ratio-bottom", 0)`,
		`user_opts.deadzonesize = 0`,
		`user_opts.keep_with_cursor = false`,
		`visibility_mode(restore_visibility, true)`,
	} {
		if !strings.Contains(string(script), expected) {
			t.Fatalf("expected bundled ModernZ script to contain %q", expected)
		}
	}
}

func TestSessionPathsSharePerProcessRoot(t *testing.T) {
	sourceDir := writeModernZTestAssets(t)
	t.Setenv(modernZEnvDir, sourceDir)

	inputPath, err := ensureInputConf()
	if err != nil {
		t.Fatalf("ensureInputConf returned error: %v", err)
	}
	configPath, err := ensureConfig()
	if err != nil {
		t.Fatalf("ensureConfig returned error: %v", err)
	}
	modernZ, err := ensureModernZAssets()
	if err != nil {
		t.Fatalf("ensureModernZAssets returned error: %v", err)
	}

	root, err := sessionDir()
	if err != nil {
		t.Fatalf("sessionDir returned error: %v", err)
	}
	expected := []string{inputPath, configPath, modernZ.ConfigDir, modernZ.ScriptPath, modernZ.ThumbfastScriptPath}
	for _, path := range expected {
		if !strings.HasPrefix(path, root+string(os.PathSeparator)) {
			t.Fatalf("expected %s to be under isolated mpv session dir %s", path, root)
		}
	}
}

func writeModernZTestAssets(t *testing.T) string {
	t.Helper()

	sourceDir := t.TempDir()
	files := map[string]string{
		"modernz.lua":       "-- test lua\n",
		"modernz.conf":      "layout=modern\n",
		"modernz-icons.ttf": "test font",
		"thumbfast.lua":     "-- thumbfast lua\n",
		"thumbfast.conf":    "max_height=200\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(sourceDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write source asset: %v", err)
		}
	}
	return sourceDir
}
