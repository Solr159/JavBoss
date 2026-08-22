package migrations

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestMoveJavDBTagsToManualScrape(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE jav_tag_map (
		jav_id integer NOT NULL,
		jav_tag_id integer NOT NULL,
		provider integer NOT NULL,
		created_at datetime,
		PRIMARY KEY (jav_id, jav_tag_id, provider)
	)`); err != nil {
		t.Fatalf("create jav_tag_map: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO jav_tag_map (jav_id, jav_tag_id, provider, created_at) VALUES
		(1, 10, 4, '2026-08-01 00:00:00'),
		(2, 20, 4, '2026-08-02 00:00:00'),
		(2, 20, 1, '2026-08-03 00:00:00'),
		(3, 30, 4, '2026-08-04 00:00:00'),
		(3, 30, 3, '2026-08-05 00:00:00'),
		(4, 40, 4, '2026-08-06 00:00:00'),
		(4, 40, 11, '2026-08-07 00:00:00')`); err != nil {
		t.Fatalf("seed jav_tag_map: %v", err)
	}

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin migration: %v", err)
	}
	if err := moveJavDBTagsToManualScrape(context.Background(), tx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("move JavDB tags: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit migration: %v", err)
	}

	assertTagMapExists(t, db, 1, 10, providerManualScrape, true)
	assertTagMapExists(t, db, 2, 20, providerJavDB, false)
	assertTagMapExists(t, db, 2, 20, providerManualScrape, false)
	assertTagMapExists(t, db, 2, 20, providerJavBus, true)
	assertTagMapExists(t, db, 3, 30, providerManualScrape, true)
	assertTagMapExists(t, db, 3, 30, providerUser, true)
	assertTagMapExists(t, db, 4, 40, providerJavDB, false)
	assertTagMapExists(t, db, 4, 40, providerManualScrape, true)
}

func assertTagMapExists(t *testing.T, db *sql.DB, javID, tagID int64, provider int, want bool) {
	t.Helper()
	var count int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM jav_tag_map WHERE jav_id = ? AND jav_tag_id = ? AND provider = ?`,
		javID,
		tagID,
		provider,
	).Scan(&count); err != nil {
		t.Fatalf("count tag map (%d, %d, %d): %v", javID, tagID, provider, err)
	}
	if got := count > 0; got != want {
		t.Fatalf("tag map (%d, %d, %d) exists = %v, want %v", javID, tagID, provider, got, want)
	}
}
