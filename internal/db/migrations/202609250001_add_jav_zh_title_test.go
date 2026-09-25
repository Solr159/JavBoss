package migrations

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestAddJavZhTitlePreservesExistingTitles(t *testing.T) {
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
		if err := addJavZhTitle(context.Background(), tx); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var title, zhTitle string
	if err := db.QueryRow(`SELECT title, zh_title FROM jav WHERE id = 1`).Scan(&title, &zhTitle); err != nil {
		t.Fatal(err)
	}
	if title != "元のタイトル" || zhTitle != "" {
		t.Fatalf("titles after migration: %q, %q", title, zhTitle)
	}
}
