package db

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"javboss/internal/models"

	"gorm.io/gorm"
)

func createDirectoryLocationFixtures(t *testing.T, gdb *gorm.DB, count int) (models.Directory, []models.VideoLocation) {
	t.Helper()
	dir := models.Directory{Path: "/tmp/large-library"}
	if err := gdb.Create(&dir).Error; err != nil {
		t.Fatal(err)
	}
	videos := make([]models.Video, count)
	for i := range videos {
		videos[i] = models.Video{Fingerprint: fmt.Sprintf("video-%d", i), Size: int64(i + 1), DurationSec: 60}
	}
	if err := gdb.CreateInBatches(&videos, 100).Error; err != nil {
		t.Fatal(err)
	}
	locations := make([]models.VideoLocation, count)
	for i, video := range videos {
		name := fmt.Sprintf("video-%d.mp4", i)
		locations[i] = models.VideoLocation{
			VideoID: video.ID, DirectoryID: dir.ID, RelativePath: name, Filename: name,
			ModifiedAt: time.Unix(1710000000, 0).UTC(),
		}
	}
	if err := gdb.CreateInBatches(&locations, 100).Error; err != nil {
		t.Fatal(err)
	}
	return dir, locations
}

func TestVideoLocationsByDirectoryLargeLibrary(t *testing.T) {
	gdb := openTestDB(t)
	// Reproduce the reported library size, exceeding SQLite's 32766-variable limit.
	dir, fixtures := createDirectoryLocationFixtures(t, gdb, 41888)
	duplicate := models.VideoLocation{VideoID: fixtures[0].VideoID, DirectoryID: dir.ID, RelativePath: "copy.mp4"}
	otherDir := models.Directory{Path: "/tmp/other-library"}
	if err := gdb.Create(&otherDir).Error; err != nil {
		t.Fatal(err)
	}
	other := models.VideoLocation{VideoID: fixtures[0].VideoID, DirectoryID: otherDir.ID, RelativePath: "other.mp4"}
	for _, loc := range []*models.VideoLocation{&duplicate, &other} {
		if err := gdb.Create(loc).Error; err != nil {
			t.Fatal(err)
		}
	}
	locations, err := VideoLocationsByDirectory(t.Context(), dir.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(locations) != len(fixtures)+1 {
		t.Fatalf("got %d locations, want %d", len(locations), len(fixtures)+1)
	}
	seen := make(map[int64]bool, len(locations))
	for _, loc := range locations {
		if loc.DirectoryID != dir.ID || seen[loc.ID] {
			t.Fatalf("unexpected or duplicate location: %d", loc.ID)
		}
		seen[loc.ID] = true
		if loc.Video.ID != loc.VideoID || loc.Video.Size == 0 || loc.Video.DurationSec != 60 || loc.Video.Fingerprint == "" {
			t.Fatalf("video metadata not loaded for location %d: %+v", loc.ID, loc.Video)
		}
	}
	for _, loc := range append(fixtures, duplicate) {
		if !seen[loc.ID] {
			t.Fatalf("missing location %d", loc.ID)
		}
	}
	empty, err := VideoLocationsByDirectory(t.Context(), otherDir.ID+1)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty directory: count=%d err=%v", len(empty), err)
	}
}

func TestDeleteVideoLocationsByIDs(t *testing.T) {
	for _, tc := range []struct {
		name     string
		count    int
		failLast bool
	}{
		{name: "empty", count: 0},
		{name: "single", count: 1},
		{name: "large library", count: 41888},
		{name: "rollback earlier batches", count: 1001, failLast: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gdb := openTestDB(t)
			_, locations := createDirectoryLocationFixtures(t, gdb, tc.count+1)
			ids := make([]int64, tc.count)
			for i := range ids {
				ids[i] = locations[i].ID
			}
			if tc.failLast {
				trigger := fmt.Sprintf(`CREATE TRIGGER reject_location_delete BEFORE DELETE ON video_location
					WHEN OLD.id = %d BEGIN SELECT RAISE(ABORT, 'forced delete failure'); END`, ids[len(ids)-1])
				if err := gdb.Exec(trigger).Error; err != nil {
					t.Fatal(err)
				}
			}
			err := DeleteVideoLocationsByIDs(t.Context(), ids)
			wantDeleted := int64(tc.count)
			if tc.failLast {
				if err == nil || !strings.Contains(err.Error(), "forced delete failure") {
					t.Fatalf("expected forced delete failure, got %v", err)
				}
				wantDeleted = 0
			} else if err != nil {
				t.Fatal(err)
			}
			var remaining, videos int64
			if err := gdb.Model(&models.VideoLocation{}).Count(&remaining).Error; err != nil {
				t.Fatal(err)
			}
			if want := int64(tc.count+1) - wantDeleted; remaining != want {
				t.Fatalf("remaining locations = %d, want %d", remaining, want)
			}
			var untouched models.VideoLocation
			if err := gdb.First(&untouched, locations[tc.count].ID).Error; err != nil {
				t.Fatalf("unselected location changed: %+v, err=%v", untouched, err)
			}
			if err := gdb.Model(&models.Video{}).Count(&videos).Error; err != nil || videos != int64(tc.count+1) {
				t.Fatalf("video metadata changed: count=%d err=%v", videos, err)
			}
		})
	}
}

func TestVideoLocationPathExistsAfterDelete(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1710000000, 0).UTC()

	dir := models.Directory{Path: "/tmp/media"}
	if err := gdb.Create(&dir).Error; err != nil {
		t.Fatalf("create directory: %v", err)
	}
	video := models.Video{
		DirectoryID: dir.ID,
		Path:        "deleted.mp4",
		Filename:    "deleted.mp4",
		Fingerprint: "deleted-path-exists-fp",
		ModifiedAt:  now,
		DurationSec: 1,
	}
	if err := gdb.Create(&video).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}
	loc, err := UpsertVideoLocation(ctx, video.ID, dir.ID, "deleted.mp4", now)
	if err != nil {
		t.Fatalf("upsert location: %v", err)
	}
	if err := DeleteVideoLocationsByIDs(ctx, []int64{loc.ID}); err != nil {
		t.Fatalf("delete location: %v", err)
	}

	exists, err := VideoLocationPathExists(ctx, dir.ID, "deleted.mp4")
	if err != nil {
		t.Fatalf("check path exists: %v", err)
	}
	if exists {
		t.Fatal("deleted location should not reserve its path for rename conflict checks")
	}
}

func TestUpdateVideoLocationPathReusesDeletedPath(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1710000000, 0).UTC()

	dir := models.Directory{Path: "/tmp/media"}
	if err := gdb.Create(&dir).Error; err != nil {
		t.Fatalf("create directory: %v", err)
	}
	deletedVideo := models.Video{
		DirectoryID: dir.ID,
		Path:        "target.mp4",
		Filename:    "target.mp4",
		Fingerprint: "deleted-target-fp",
		ModifiedAt:  now,
	}
	activeVideo := models.Video{
		DirectoryID: dir.ID,
		Path:        "source.mp4",
		Filename:    "source.mp4",
		Fingerprint: "active-source-fp",
		ModifiedAt:  now,
	}
	if err := gdb.Create(&deletedVideo).Error; err != nil {
		t.Fatalf("create deleted video: %v", err)
	}
	if err := gdb.Create(&activeVideo).Error; err != nil {
		t.Fatalf("create active video: %v", err)
	}
	deletedLoc, err := UpsertVideoLocation(ctx, deletedVideo.ID, dir.ID, "target.mp4", now)
	if err != nil {
		t.Fatalf("upsert deleted location: %v", err)
	}
	activeLoc, err := UpsertVideoLocation(ctx, activeVideo.ID, dir.ID, "source.mp4", now)
	if err != nil {
		t.Fatalf("upsert active location: %v", err)
	}
	if err := DeleteVideoLocationsByIDs(ctx, []int64{deletedLoc.ID}); err != nil {
		t.Fatalf("delete target location: %v", err)
	}

	updated, err := UpdateVideoLocationPath(ctx, activeLoc.ID, "target.mp4", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("update active location path: %v", err)
	}
	if updated.ID != activeLoc.ID || updated.RelativePath != "target.mp4" || updated.Filename != "target.mp4" {
		t.Fatalf("unexpected updated location: %#v", updated)
	}

	var locations []models.VideoLocation
	if err := gdb.
		Where("directory_id = ? AND relative_path = ?", dir.ID, "target.mp4").
		Find(&locations).Error; err != nil {
		t.Fatalf("load target locations: %v", err)
	}
	if len(locations) != 1 {
		t.Fatalf("target path should have exactly one row after reuse: %#v", locations)
	}
	if locations[0].ID != activeLoc.ID {
		t.Fatalf("target path should belong to the active location: %#v", locations[0])
	}
}
