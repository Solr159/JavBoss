package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"

	"javboss/internal/models"
)

func TestRemoveVideoLocationIsDeleteMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrade.db")
	legacy, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { legacy.Close() })
	if err := goose.UpToContext(t.Context(), legacy, migrationDir, 202610040001); err != nil {
		t.Fatal(err)
	}
	// Include NULL (historically visible) and a video whose only location was deleted.
	for _, statement := range []string{
		`INSERT INTO directory(id, path) VALUES (1, '/media')`,
		`INSERT INTO video(id, fingerprint, watched_ms, play_count) VALUES (1, 'one', 1000, 2), (2, 'two', 2000, 3)`,
		`INSERT INTO jav(id, code, watched_ms) VALUES (1, 'ABC-001', 3000)`,
		`INSERT INTO video_location(id, video_id, directory_id, relative_path, filename, jav_id, is_delete, created_at, updated_at)
		 VALUES (1, 1, 1, 'a.mp4', 'a.mp4', 1, 0, '2026-01-01', '2026-02-01'),
		        (2, 1, 1, 'b.mp4', 'b.mp4', 1, NULL, '2026-01-01', '2026-02-01'),
		        (3, 2, 1, 'gone.mp4', 'gone.mp4', 1, 1, '2026-01-01', '2026-02-01')`,
	} {
		if _, err := legacy.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	upgraded, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer closeDBForSchemaCompare(t, upgraded)
	fresh, err := Open(filepath.Join(t.TempDir(), "fresh.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer closeDBForSchemaCompare(t, fresh)
	assertSchemaSnapshotEqual(t, loadSchemaSnapshot(t, upgraded), loadSchemaSnapshot(t, fresh))
	if upgraded.Migrator().HasColumn(&models.VideoLocation{}, "is_delete") {
		t.Fatal("is_delete still exists")
	}
	var locations []models.VideoLocation
	if err := upgraded.Order("id").Find(&locations).Error; err != nil {
		t.Fatal(err)
	}
	if len(locations) != 2 || locations[0].ID != 1 || locations[1].ID != 2 ||
		locations[0].JavID == nil || *locations[0].JavID != 1 || locations[0].RelativePath != "a.mp4" ||
		locations[0].CreatedAt.Year() != 2026 || locations[0].UpdatedAt.Month() != 2 {
		t.Fatalf("unexpected migrated locations: %+v", locations)
	}
	var video models.Video
	if err := upgraded.First(&video, 2).Error; err != nil || video.WatchedMS != 2000 || video.PlayCount != 3 {
		t.Fatalf("orphan video history changed: %+v, err=%v", video, err)
	}
	var jav models.Jav
	if err := upgraded.First(&jav, 1).Error; err != nil || jav.WatchedMS != 3000 {
		t.Fatalf("JAV history changed: %+v, err=%v", jav, err)
	}
	// Deleted IDs must not be reused by subsequent scans or pending worker tasks.
	next := models.VideoLocation{VideoID: 2, DirectoryID: 1, RelativePath: "gone.mp4"}
	if err := upgraded.Create(&next).Error; err != nil || next.ID <= 3 {
		t.Fatalf("new location = %+v, err=%v", next, err)
	}
	rows, err := upgraded.Raw("PRAGMA foreign_key_check").Rows()
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() || rows.Err() != nil {
		t.Fatal("foreign key integrity check failed")
	}
}
