package migrations

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestAddDirectoryEnabledDefaultsExistingRowsToEnabled(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE directory (id integer PRIMARY KEY, path text)`); err != nil {
		t.Fatalf("create directory table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO directory (id, path) VALUES (1, '/media/one')`); err != nil {
		t.Fatalf("seed directory: %v", err)
	}

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin migration: %v", err)
	}
	if err := addDirectoryEnabled(context.Background(), tx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("add directory enabled: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit migration: %v", err)
	}

	var enabled bool
	if err := db.QueryRow(`SELECT enabled FROM directory WHERE id = 1`).Scan(&enabled); err != nil {
		t.Fatalf("read migrated directory: %v", err)
	}
	if !enabled {
		t.Fatal("existing directory should be enabled after migration")
	}
}
