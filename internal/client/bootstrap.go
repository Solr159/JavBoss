package client

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const bootstrapConfigName = "config.toml"

// BootstrapConfig contains the settings needed before either runtime mode is
// initialized. Server-only application settings continue to live in SQLite.
type BootstrapConfig struct {
	ServerURL string `toml:"server_url"`
	Port      int    `toml:"port"`
}

func LoadBootstrapConfig(baseDir string) (BootstrapConfig, error) {
	var cfg BootstrapConfig
	data, err := os.ReadFile(filepath.Join(baseDir, bootstrapConfigName))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read client bootstrap config: %w", err)
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse client bootstrap config: %w", err)
	}
	return cfg, nil
}

func NormalizeServerURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", errors.New("remote JavBoss address is required")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return "", errors.New("remote JavBoss address is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("remote JavBoss address must use http or https")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("remote JavBoss address must not contain credentials, query, or fragment")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", errors.New("remote JavBoss must be hosted at the address root")
	}
	parsed.Path = ""
	parsed.RawPath = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}
