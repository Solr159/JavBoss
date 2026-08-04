package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const localSettingsFilename = "client-settings.json"

var localPlayerConfigKeys = map[string]struct{}{
	"default_player":          {},
	"player_hotkeys":          {},
	"player_ontop":            {},
	"player_resume_playback":  {},
	"player_reuse_window":     {},
	"player_show_hotkey_hint": {},
	"player_volume":           {},
	"player_window_height":    {},
	"player_window_size":      {},
	"player_window_width":     {},
}

type localSettings struct {
	Player map[string]string `json:"player"`
}

type settingsStore struct {
	mu   sync.RWMutex
	path string
	data localSettings
}

func loadSettingsStore(baseDir string) (*settingsStore, error) {
	store := &settingsStore{
		path: filepath.Join(baseDir, "data", localSettingsFilename),
		data: localSettings{Player: map[string]string{
			"default_player": "mpv",
		}},
	}
	data, err := os.ReadFile(store.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store, nil
		}
		return nil, fmt.Errorf("read client settings: %w", err)
	}
	if err := json.Unmarshal(data, &store.data); err != nil {
		return nil, fmt.Errorf("parse client settings: %w", err)
	}
	if store.data.Player == nil {
		store.data.Player = make(map[string]string)
	}
	if store.data.Player["default_player"] == "" {
		store.data.Player["default_player"] = "mpv"
	}
	return store, nil
}

func (s *settingsStore) playerConfig() (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]string, len(s.data.Player))
	for key, value := range s.data.Player {
		result[key] = value
	}
	return result, nil
}

func (s *settingsStore) updatePlayer(entries map[string]string) error {
	if len(entries) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, value := range entries {
		s.data.Player[key] = value
	}
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode client settings: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create client settings directory: %w", err)
	}
	if err := writeFileAtomic(s.path, data, 0o600); err != nil {
		return fmt.Errorf("save client settings: %w", err)
	}
	return nil
}

func splitConfigPayload(payload map[string]json.RawMessage) (map[string]string, map[string]json.RawMessage, error) {
	local := make(map[string]string)
	remote := make(map[string]json.RawMessage)
	for key, raw := range payload {
		if _, ok := localPlayerConfigKeys[key]; !ok {
			remote[key] = raw
			continue
		}
		value, err := normalizeLocalPlayerSetting(key, raw)
		if err != nil {
			return nil, nil, err
		}
		local[key] = value
	}
	return local, remote, nil
}

func normalizeLocalPlayerSetting(key string, raw json.RawMessage) (string, error) {
	switch key {
	case "default_player":
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", errors.New("default player is invalid")
		}
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "mpv" && value != "browser" {
			return "", errors.New("client default player must be mpv or browser")
		}
		return value, nil
	case "player_window_size", "player_window_width", "player_window_height":
		value, err := rawInt(raw)
		if err != nil || value < 10 || value > 100 {
			return "", errors.New("player window size must be between 10 and 100")
		}
		return strconv.Itoa(value), nil
	case "player_volume":
		value, err := rawInt(raw)
		if err != nil || value < 0 || value > 130 {
			return "", errors.New("player volume must be between 0 and 130")
		}
		return strconv.Itoa(value), nil
	case "player_ontop", "player_resume_playback", "player_reuse_window", "player_show_hotkey_hint":
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", errors.New("player boolean setting is invalid")
		}
		return strconv.FormatBool(value), nil
	case "player_hotkeys":
		var value []map[string]any
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", errors.New("player hotkeys are invalid")
		}
		if len(value) > 100 {
			return "", errors.New("too many player hotkeys")
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return "", errors.New("player hotkeys are invalid")
		}
		return string(encoded), nil
	default:
		return "", errors.New("unsupported local player setting")
	}
}

func rawInt(raw json.RawMessage) (int, error) {
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, err
	}
	return value, nil
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".javboss-client-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	cleanup := func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}
	if err := temp.Chmod(mode); err != nil {
		cleanup()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		cleanup()
		return err
	}
	if err := temp.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	renameErr := os.Rename(tempPath, path)
	if renameErr == nil {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		_ = os.Remove(tempPath)
		return renameErr
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(tempPath)
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	return nil
}
