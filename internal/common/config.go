package common

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config represents the application configuration (hardcoded defaults).
type Config struct {
	DatabasePath  string `json:"database_path"`
	ThumbnailsDir string `json:"thumbnails_dir"`
	JavCoverDir   string `json:"jav_cover_dir"`
}

const (
	defaultDatabasePath  = "data/javboss.db"
	defaultThumbnailsDir = "data/thumbnails"
	defaultJavCoverDir   = "data/cover"
)

// Load returns the hardcoded configuration.
func Load() (*Config, error) {
	baseDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("resolve base directory: %w", err)
	}
	return LoadWithBaseDir(baseDir)
}

// LoadWithBaseDir returns the hardcoded configuration relative to baseDir.
func LoadWithBaseDir(baseDir string) (*Config, error) {
	if baseDir == "" {
		return nil, fmt.Errorf("resolve base directory: empty")
	}
	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("resolve base directory: %w", err)
	}

	cfg := Config{
		DatabasePath:  defaultDatabasePath,
		ThumbnailsDir: defaultThumbnailsDir,
		JavCoverDir:   defaultJavCoverDir,
	}

	// Resolve paths relative to the base directory.
	resolve := func(p string) (string, error) {
		if p == "" {
			return "", nil
		}
		if filepath.IsAbs(p) {
			return filepath.Clean(p), nil
		}
		return filepath.Clean(filepath.Join(absBaseDir, p)), nil
	}

	absDB, err := resolve(cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	cfg.DatabasePath = absDB

	if err := os.MkdirAll(filepath.Dir(cfg.DatabasePath), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	if cfg.ThumbnailsDir == "" {
		cfg.ThumbnailsDir = filepath.Join(filepath.Dir(cfg.DatabasePath), "thumbnails")
	}
	absThumbs, err := resolve(cfg.ThumbnailsDir)
	if err != nil {
		return nil, fmt.Errorf("resolve thumbnails_dir: %w", err)
	}
	if err := os.MkdirAll(absThumbs, 0o755); err != nil {
		return nil, fmt.Errorf("create thumbnails_dir: %w", err)
	}
	cfg.ThumbnailsDir = absThumbs

	if cfg.JavCoverDir == "" {
		cfg.JavCoverDir = filepath.Join(filepath.Dir(cfg.DatabasePath), "cover")
	}
	absCovers, err := resolve(cfg.JavCoverDir)
	if err != nil {
		return nil, fmt.Errorf("resolve jav_cover_dir: %w", err)
	}
	if err := os.MkdirAll(absCovers, 0o755); err != nil {
		return nil, fmt.Errorf("create jav_cover_dir: %w", err)
	}
	cfg.JavCoverDir = absCovers

	return &cfg, nil
}
