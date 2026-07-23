package db

import (
	"testing"

	"gorm.io/gorm"
)

func TestJavSchemaOmitsFrontendEnglishMetadataColumns(t *testing.T) {
	db := openTestDB(t)

	assertTableColumns(t, db, "jav", []string{
		"id", "code", "title", "studio_id", "series_id", "series_en_id", "release_unix",
		"duration_min", "fetched_at", "created_at", "updated_at", "is_uncensored",
	})
	assertTableColumns(t, db, "jav_series", []string{
		"id", "name", "is_english", "studio_id", "created_at", "updated_at",
	})
	assertTableColumns(t, db, "jav_idol", []string{
		"id", "name", "roman_name", "japanese_name", "chinese_name", "height_cm",
		"birth_date", "bust", "waist", "hips", "cup", "created_at", "updated_at",
		"cover_jav_id", "cover_crop_left",
	})
	assertTableColumns(t, db, "jav_idol_alias", []string{
		"id", "jav_idol_id", "alias", "created_at",
	})
}

func assertTableColumns(t *testing.T, db *gorm.DB, table string, want []string) {
	t.Helper()

	rows, err := db.Raw("PRAGMA table_info(" + table + ")").Rows()
	if err != nil {
		t.Fatalf("load %s columns: %v", table, err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultV   any
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultV, &primaryKey); err != nil {
			t.Fatalf("scan %s columns: %v", table, err)
		}
		got = append(got, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate %s columns: %v", table, err)
	}
	if len(got) != len(want) {
		t.Fatalf("%s columns = %#v, want %#v", table, got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("%s columns = %#v, want %#v", table, got, want)
		}
	}
}
