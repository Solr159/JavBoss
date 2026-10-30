package migrations

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestAddJavEnTitlePreservesExistingTitles(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE jav (id integer PRIMARY KEY, title text); INSERT INTO jav (id, title) VALUES (1, '元のタイトル')`); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for i := 0; i < 2; i++ {
		if err := addJavEnTitle(context.Background(), tx); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var title, enTitle string
	if err := db.QueryRow(`SELECT title, en_title FROM jav WHERE id = 1`).Scan(&title, &enTitle); err != nil {
		t.Fatal(err)
	}
	if title != "元のタイトル" || enTitle != "" {
		t.Fatalf("titles after migration: %q, %q", title, enTitle)
	}
}
