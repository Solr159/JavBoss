package migrations

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestTraditionalizeScrapedJavTags(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	statements := []string{
		`CREATE TABLE jav_tag (
			id integer PRIMARY KEY,
			name text,
			is_user numeric NOT NULL DEFAULT 0,
			category_id integer,
			created_at datetime,
			updated_at datetime
		)`,
		`CREATE UNIQUE INDEX idx_jav_tag_name_user ON jav_tag(name, is_user)`,
		`CREATE TABLE jav_tag_map (
			jav_id integer NOT NULL,
			jav_tag_id integer NOT NULL,
			provider integer NOT NULL,
			created_at datetime,
			PRIMARY KEY (jav_id, jav_tag_id, provider)
		)`,
		`INSERT INTO jav_tag (id, name, is_user, category_id) VALUES
			(10, '无码', 0, 7),
			(20, '無碼', 0, NULL),
			(30, '女优', 0, NULL),
			(40, '女优', 1, NULL),
			(50, '原創', 0, NULL)`,
		`INSERT INTO jav_tag_map (jav_id, jav_tag_id, provider, created_at) VALUES
			(1, 10, 1, '2026-08-01 00:00:00'),
			(1, 20, 1, '2026-08-02 00:00:00'),
			(2, 10, 4, '2026-08-03 00:00:00'),
			(3, 20, 11, '2026-08-04 00:00:00'),
			(4, 30, 1, '2026-08-05 00:00:00')`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("prepare migration test database: %v", err)
		}
	}

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin migration: %v", err)
	}
	if err := traditionalizeScrapedJavTags(context.Background(), tx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("traditionalize scraped JAV tags: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit migration: %v", err)
	}

	assertMigratedJavTag(t, db, 20, "無碼", false, sql.NullInt64{Int64: 7, Valid: true})
	assertMigratedJavTag(t, db, 30, "女優", false, sql.NullInt64{})
	assertMigratedJavTag(t, db, 40, "女优", true, sql.NullInt64{})
	assertMigratedJavTag(t, db, 50, "原創", false, sql.NullInt64{})
	assertJavTagMissing(t, db, 10)

	assertTagMapExists(t, db, 1, 20, 1, true)
	assertTagMapExists(t, db, 2, 20, 4, true)
	assertTagMapExists(t, db, 3, 20, 11, true)
	assertTagMapExists(t, db, 4, 30, 1, true)
	assertTagMapExists(t, db, 1, 10, 1, false)

	var duplicateCount int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM jav_tag_map WHERE jav_id = 1 AND jav_tag_id = 20 AND provider = 1`,
	).Scan(&duplicateCount); err != nil {
		t.Fatalf("count merged duplicate map: %v", err)
	}
	if duplicateCount != 1 {
		t.Fatalf("merged duplicate map count = %d, want 1", duplicateCount)
	}
}

func assertMigratedJavTag(t *testing.T, db *sql.DB, id int64, wantName string, wantUser bool, wantCategory sql.NullInt64) {
	t.Helper()
	var (
		name       string
		isUser     bool
		categoryID sql.NullInt64
	)
	if err := db.QueryRow(
		`SELECT name, is_user, category_id FROM jav_tag WHERE id = ?`,
		id,
	).Scan(&name, &isUser, &categoryID); err != nil {
		t.Fatalf("load JAV tag %d: %v", id, err)
	}
	if name != wantName || isUser != wantUser || categoryID != wantCategory {
		t.Fatalf(
			"JAV tag %d = (name %q, user %v, category %+v), want (%q, %v, %+v)",
			id,
			name,
			isUser,
			categoryID,
			wantName,
			wantUser,
			wantCategory,
		)
	}
}

func assertJavTagMissing(t *testing.T, db *sql.DB, id int64) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM jav_tag WHERE id = ?`, id).Scan(&count); err != nil {
		t.Fatalf("count JAV tag %d: %v", id, err)
	}
	if count != 0 {
		t.Fatalf("JAV tag %d still exists", id)
	}
}
