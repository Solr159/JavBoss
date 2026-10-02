package migrations

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestAddJavCoverCropLeftDefaultsExistingRows(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE jav (id integer PRIMARY KEY, title text); INSERT INTO jav (id, title) VALUES (1, 'ABC-001')`); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for i := 0; i < 2; i++ {
		if err := addJavCoverCropLeft(context.Background(), tx); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var title string
	var cropLeft float64
	if err := db.QueryRow(`SELECT title, cover_crop_left FROM jav WHERE id = 1`).Scan(&title, &cropLeft); err != nil {
		t.Fatal(err)
	}
	if title != "ABC-001" || cropLeft != 0.53 {
		t.Fatalf("row after migration: title=%q crop_left=%v", title, cropLeft)
	}
}
