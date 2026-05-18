package mpv

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const uoscEnvDir = "PORNBOSS_UOSC_DIR"

var uoscAssetMappings = []struct {
	source string
	target string
	dir    bool
}{
	{source: "uosc.conf", target: filepath.Join("script-opts", "uosc.conf")},
	{source: "thumbfast.conf", target: filepath.Join("script-opts", "thumbfast.conf")},
	{source: filepath.Join("scripts", "thumbfast.lua"), target: filepath.Join("scripts", "thumbfast.lua")},
	{source: filepath.Join("scripts", "uosc"), target: filepath.Join("scripts", "uosc"), dir: true},
	{source: "fonts", target: "fonts", dir: true},
}

var uoscRequiredFiles = []string{
	"uosc.conf",
	"thumbfast.conf",
	filepath.Join("scripts", "thumbfast.lua"),
	filepath.Join("scripts", "uosc", "main.lua"),
	filepath.Join("scripts", "uosc", "bin", "ziggy-darwin"),
	filepath.Join("scripts", "uosc", "bin", "ziggy-linux"),
	filepath.Join("scripts", "uosc", "bin", "ziggy-windows.exe"),
	filepath.Join("fonts", "uosc_icons.otf"),
	filepath.Join("fonts", "uosc_textures.ttf"),
}

type uoscAssets struct {
	ConfigDir           string
	ScriptPath          string
	ThumbfastScriptPath string
}

func ensureUOSCAssets() (uoscAssets, error) {
	sourceDir, err := findUOSCSourceDir()
	if err != nil {
		return uoscAssets{}, err
	}

	configDir, err := sessionPath("config")
	if err != nil {
		return uoscAssets{}, err
	}
	for _, asset := range uoscAssetMappings {
		sourcePath := filepath.Join(sourceDir, asset.source)
		targetPath := filepath.Join(configDir, asset.target)
		if asset.dir {
			err = syncUOSCDir(sourcePath, targetPath)
		} else {
			err = syncUOSCFile(sourcePath, targetPath)
		}
		if err != nil {
			return uoscAssets{}, err
		}
	}

	return uoscAssets{
		ConfigDir:           configDir,
		ScriptPath:          filepath.Join(configDir, "scripts", "uosc"),
		ThumbfastScriptPath: filepath.Join(configDir, "scripts", "thumbfast.lua"),
	}, nil
}

func writeThumbfastRuntimeConfig(configDir, mpvPath string) error {
	path := filepath.Join(configDir, "script-opts", "thumbfast.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read thumbfast config: %w", err)
	}

	line := "mpv_path=" + mpvPath
	lines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	found := false
	for i, item := range lines {
		if strings.HasPrefix(strings.TrimSpace(item), "mpv_path=") {
			lines[i] = line
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, line)
	}

	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		return fmt.Errorf("write thumbfast config: %w", err)
	}
	return nil
}

func syncUOSCDir(sourceDir, targetDir string) error {
	return filepath.WalkDir(sourceDir, func(sourcePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk uosc asset %s: %w", sourcePath, walkErr)
		}
		relative, err := filepath.Rel(sourceDir, sourcePath)
		if err != nil {
			return fmt.Errorf("resolve uosc asset path %s: %w", sourcePath, err)
		}
		targetPath := filepath.Join(targetDir, relative)
		if entry.IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return fmt.Errorf("create uosc asset dir %s: %w", targetPath, err)
			}
			return nil
		}
		return syncUOSCFile(sourcePath, targetPath)
	})
}

func syncUOSCFile(sourcePath, targetPath string) error {
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read uosc asset %s: %w", sourcePath, err)
	}

	info, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("stat uosc asset %s: %w", sourcePath, err)
	}
	mode := info.Mode().Perm()
	if mode == 0 {
		mode = 0o644
	}

	if current, err := os.ReadFile(targetPath); err == nil && bytes.Equal(current, content) {
		_ = os.Chmod(targetPath, mode)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create uosc asset dir: %w", err)
	}
	if err := os.WriteFile(targetPath, content, mode); err != nil {
		return fmt.Errorf("write uosc asset %s: %w", targetPath, err)
	}
	return nil
}

func findUOSCSourceDir() (string, error) {
	if dir := os.Getenv(uoscEnvDir); dir != "" {
		return validateUOSCSourceDir(dir)
	}

	var bases []string
	if cwd, err := os.Getwd(); err == nil {
		bases = append(bases, cwd)
	}
	if exe, err := os.Executable(); err == nil {
		bases = append(bases, filepath.Dir(exe))
	}

	seen := make(map[string]struct{}, len(bases))
	for _, base := range bases {
		for _, candidate := range uoscCandidateDirs(base) {
			abs, err := filepath.Abs(candidate)
			if err == nil {
				candidate = abs
			}
			if _, ok := seen[candidate]; ok {
				continue
			}
			seen[candidate] = struct{}{}
			if dir, err := validateUOSCSourceDir(candidate); err == nil {
				return dir, nil
			}
		}
	}

	return "", errors.New("uosc assets not found; expected uosc/uosc.conf, thumbfast.conf, scripts/thumbfast.lua, scripts/uosc, scripts/uosc/bin, and fonts")
}

func uoscCandidateDirs(base string) []string {
	var candidates []string
	dir := base
	for i := 0; i < 5; i++ {
		candidates = append(candidates, filepath.Join(dir, "uosc"))
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return candidates
}

func validateUOSCSourceDir(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for _, file := range uoscRequiredFiles {
		if _, err := os.Stat(filepath.Join(abs, file)); err != nil {
			return "", err
		}
	}
	return abs, nil
}
