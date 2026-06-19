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
	legacyDatabaseName   = "pornboss.db"
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

// MigrateLegacyDatabase moves the pre-rename SQLite database into the current
// default filename when the current database does not exist yet.
func MigrateLegacyDatabase(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("migrate legacy database: missing config")
	}
	if cfg.DatabasePath == "" {
		return fmt.Errorf("migrate legacy database: empty database path")
	}
	legacyPath := filepath.Join(filepath.Dir(cfg.DatabasePath), legacyDatabaseName)
	if filepath.Clean(legacyPath) == filepath.Clean(cfg.DatabasePath) {
		return nil
	}
	return migrateLegacySQLiteFiles(legacyPath, cfg.DatabasePath)
}

func migrateLegacySQLiteFiles(legacyPath, currentPath string) error {
	if _, err := os.Stat(currentPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect current database: %w", err)
	}

	if _, err := os.Stat(legacyPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("inspect legacy database: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(currentPath), 0o755); err != nil {
		return fmt.Errorf("create database directory: %w", err)
	}

	for _, suffix := range []string{"-wal", "-shm"} {
		if err := migrateLegacyFile(legacyPath+suffix, currentPath+suffix); err != nil {
			return err
		}
	}
	return migrateLegacyFile(legacyPath, currentPath)
}

func migrateLegacyFile(legacyPath, currentPath string) error {
	if _, err := os.Stat(legacyPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("inspect legacy file: %w", err)
	}
	if err := os.Rename(legacyPath, currentPath); err != nil {
		return fmt.Errorf("migrate legacy file: %w", err)
	}
	return nil
}
