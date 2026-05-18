package mpv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureUOSCAssetsCopiesScriptOptionsFontsAndScriptDirectory(t *testing.T) {
	files := uoscTestFiles()
	sourceDir := writeUOSCTestAssets(t)

	t.Setenv(uoscEnvDir, sourceDir)

	assets, err := ensureUOSCAssets()
	if err != nil {
		t.Fatalf("ensureUOSCAssets returned error: %v", err)
	}

	expected := map[string]string{
		filepath.Join(assets.ConfigDir, "script-opts", "uosc.conf"):              files["uosc.conf"],
		filepath.Join(assets.ConfigDir, "scripts", "uosc", "bin", "ziggy-linux"): files[filepath.Join("scripts", "uosc", "bin", "ziggy-linux")],
		filepath.Join(assets.ConfigDir, "scripts", "uosc", "main.lua"):           files[filepath.Join("scripts", "uosc", "main.lua")],
		filepath.Join(assets.ConfigDir, "scripts", "uosc", "lib", "x.lua"):       files[filepath.Join("scripts", "uosc", "lib", "x.lua")],
		filepath.Join(assets.ConfigDir, "fonts", "uosc_icons.otf"):               files[filepath.Join("fonts", "uosc_icons.otf")],
		filepath.Join(assets.ConfigDir, "fonts", "uosc_textures.ttf"):            files[filepath.Join("fonts", "uosc_textures.ttf")],
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
	info, err := os.Stat(assets.ScriptPath)
	if err != nil {
		t.Fatalf("stat uosc script path: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected uosc script path %s to be a directory", assets.ScriptPath)
	}
}

func TestSessionPathsSharePerProcessRoot(t *testing.T) {
	sourceDir := writeUOSCTestAssets(t)
	t.Setenv(uoscEnvDir, sourceDir)

	inputPath, err := ensureInputConf()
	if err != nil {
		t.Fatalf("ensureInputConf returned error: %v", err)
	}
	configPath, err := ensureConfig()
	if err != nil {
		t.Fatalf("ensureConfig returned error: %v", err)
	}
	uosc, err := ensureUOSCAssets()
	if err != nil {
		t.Fatalf("ensureUOSCAssets returned error: %v", err)
	}

	root, err := sessionDir()
	if err != nil {
		t.Fatalf("sessionDir returned error: %v", err)
	}
	expected := []string{inputPath, configPath, uosc.ConfigDir, uosc.ScriptPath}
	for _, path := range expected {
		if !strings.HasPrefix(path, root+string(os.PathSeparator)) {
			t.Fatalf("expected %s to be under isolated mpv session dir %s", path, root)
		}
	}
}

func writeUOSCTestAssets(t *testing.T) string {
	t.Helper()

	sourceDir := t.TempDir()
	for name, content := range uoscTestFiles() {
		path := filepath.Join(sourceDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create source asset dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write source asset: %v", err)
		}
	}
	return sourceDir
}

func uoscTestFiles() map[string]string {
	return map[string]string{
		"uosc.conf": "timeline_size=40\n",
		filepath.Join("scripts", "uosc", "bin", "ziggy-darwin"):      "test ziggy darwin",
		filepath.Join("scripts", "uosc", "bin", "ziggy-linux"):       "test ziggy linux",
		filepath.Join("scripts", "uosc", "bin", "ziggy-windows.exe"): "test ziggy windows",
		filepath.Join("scripts", "uosc", "main.lua"):                 "-- test main\n",
		filepath.Join("scripts", "uosc", "lib", "x.lua"):             "-- test lib\n",
		filepath.Join("fonts", "uosc_icons.otf"):                     "test icons",
		filepath.Join("fonts", "uosc_textures.ttf"):                  "test textures",
	}
}
