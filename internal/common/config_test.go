package common

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateLegacyDatabaseMovesSQLiteFiles(t *testing.T) {
	dataDir := t.TempDir()
	cfg := &Config{DatabasePath: filepath.Join(dataDir, "javboss.db")}
	legacyPath := filepath.Join(dataDir, "pornboss.db")

	writeFile(t, legacyPath, "main")
	writeFile(t, legacyPath+"-wal", "wal")
	writeFile(t, legacyPath+"-shm", "shm")

	if err := MigrateLegacyDatabase(cfg); err != nil {
		t.Fatalf("migrate legacy database: %v", err)
	}

	assertFileContent(t, cfg.DatabasePath, "main")
	assertFileContent(t, cfg.DatabasePath+"-wal", "wal")
	assertFileContent(t, cfg.DatabasePath+"-shm", "shm")
	assertMissing(t, legacyPath)
	assertMissing(t, legacyPath+"-wal")
	assertMissing(t, legacyPath+"-shm")
}

func TestMigrateLegacyDatabaseDoesNotOverwriteCurrentDatabase(t *testing.T) {
	dataDir := t.TempDir()
	cfg := &Config{DatabasePath: filepath.Join(dataDir, "javboss.db")}
	legacyPath := filepath.Join(dataDir, "pornboss.db")

	writeFile(t, cfg.DatabasePath, "current")
	writeFile(t, legacyPath, "legacy")

	if err := MigrateLegacyDatabase(cfg); err != nil {
		t.Fatalf("migrate legacy database: %v", err)
	}

	assertFileContent(t, cfg.DatabasePath, "current")
	assertFileContent(t, legacyPath, "legacy")
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertFileContent(t *testing.T, path string, expected string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(content) != expected {
		t.Fatalf("expected %s to contain %q, got %q", path, expected, string(content))
	}
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be missing, stat err: %v", path, err)
	}
}
