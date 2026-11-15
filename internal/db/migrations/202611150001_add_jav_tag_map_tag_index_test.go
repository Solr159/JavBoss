package migrations

import (
	"context"
	"database/sql"
	"reflect"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestAddJavTagMapTagIndex(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE jav_tag_map (
		jav_id integer,
		jav_tag_id integer,
		provider integer NOT NULL DEFAULT 0,
		created_at datetime,
		PRIMARY KEY (jav_id, jav_tag_id, provider)
	);
	CREATE INDEX idx_jav_tag_map_provider ON jav_tag_map(provider);
	INSERT INTO jav_tag_map VALUES
		(1, 10, 1, '2026-11-01'),
		(1, 10, 11, '2026-11-02'),
		(2, 10, 1, '2026-11-03'),
		(2, 20, 3, '2026-11-04');`); err != nil {
		t.Fatal(err)
	}
	readMappings := func() []string {
		t.Helper()
		rows, err := db.Query(`SELECT jav_id || ':' || jav_tag_id || ':' || provider || ':' || created_at FROM jav_tag_map ORDER BY jav_id, jav_tag_id, provider`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var mappings []string
		for rows.Next() {
			var mapping string
			if err := rows.Scan(&mapping); err != nil {
				t.Fatal(err)
			}
			mappings = append(mappings, mapping)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return mappings
	}
	before := readMappings()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for i := 0; i < 2; i++ {
		if err := addJavTagMapTagIndex(context.Background(), tx); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if after := readMappings(); !reflect.DeepEqual(before, after) {
		t.Fatalf("migration changed mappings: before %v, after %v", before, after)
	}

	rows, err := db.Query(`SELECT name FROM pragma_index_info('idx_jav_tag_map_tag_provider') ORDER BY seqno`)
	if err != nil {
		t.Fatal(err)
	}
	var columns []string
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			t.Fatal(err)
		}
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if !reflect.DeepEqual(columns, []string{"jav_tag_id", "provider"}) {
		t.Fatalf("index columns = %v, want [jav_tag_id provider]", columns)
	}

	for _, tc := range []struct {
		name  string
		query string
	}{
		{"tag counts", `SELECT COUNT(DISTINCT jav_id) FROM jav_tag_map WHERE jav_tag_id = 10 AND provider IN (1, 11)`},
		{"tag existence", `SELECT 1 FROM jav_tag_map WHERE jav_tag_id = 30`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := db.Query("EXPLAIN QUERY PLAN " + tc.query)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			usesTagIndex := false
			var details []string
			for rows.Next() {
				var id, parent, unused int
				var detail string
				if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
					t.Fatal(err)
				}
				details = append(details, detail)
				if strings.Contains(detail, "SEARCH") && strings.Contains(detail, "idx_jav_tag_map_tag_provider") {
					usesTagIndex = true
				}
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			if !usesTagIndex {
				t.Fatalf("expected indexed tag lookup, got %v", details)
			}
		})
	}
}
