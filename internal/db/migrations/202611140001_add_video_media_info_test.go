package migrations

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestAddVideoMediaInfo(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE video (id integer PRIMARY KEY, fingerprint text); INSERT INTO video VALUES (1, 'original')`); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for i := 0; i < 2; i++ {
		if err := addVideoMediaInfo(context.Background(), tx); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var fingerprint string
	var settings string
	if err := db.QueryRow(`SELECT fingerprint, media_info FROM video WHERE id = 1`).Scan(&fingerprint, &settings); err != nil {
		t.Fatal(err)
	}
	if fingerprint != "original" || settings != "{}" {
		t.Fatalf("migrated row = %q, %q", fingerprint, settings)
	}
}
