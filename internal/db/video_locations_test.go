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
	if err := gdb.Model(&fixtures[0]).Update("is_delete", true).Error; err != nil {
		t.Fatal(err)
	}
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
		if loc.ID == fixtures[0].ID && !loc.IsDelete {
			t.Fatal("hidden location lost its deletion flag")
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

func TestHideVideoLocationsByIDs(t *testing.T) {
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
				trigger := fmt.Sprintf(`CREATE TRIGGER reject_location_hide BEFORE UPDATE OF is_delete ON video_location
					WHEN OLD.id = %d BEGIN SELECT RAISE(ABORT, 'forced hide failure'); END`, ids[len(ids)-1])
				if err := gdb.Exec(trigger).Error; err != nil {
					t.Fatal(err)
				}
			}
			err := HideVideoLocationsByIDs(t.Context(), ids)
			wantHidden := int64(tc.count)
			if tc.failLast {
				if err == nil || !strings.Contains(err.Error(), "forced hide failure") {
					t.Fatalf("expected forced hide failure, got %v", err)
				}
				wantHidden = 0
			} else if err != nil {
				t.Fatal(err)
			}
			var hidden, videos int64
			if err := gdb.Model(&models.VideoLocation{}).Where("is_delete = ?", true).Count(&hidden).Error; err != nil {
				t.Fatal(err)
			}
			if hidden != wantHidden {
				t.Fatalf("hidden locations = %d, want %d", hidden, wantHidden)
			}
			var untouched models.VideoLocation
			if err := gdb.First(&untouched, locations[tc.count].ID).Error; err != nil || untouched.IsDelete {
				t.Fatalf("unselected location changed: %+v, err=%v", untouched, err)
			}
			if err := gdb.Model(&models.Video{}).Count(&videos).Error; err != nil || videos != int64(tc.count+1) {
				t.Fatalf("video metadata changed: count=%d err=%v", videos, err)
			}
		})
	}
}

func TestVideoLocationPathExistsIgnoresHiddenRows(t *testing.T) {
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
		Fingerprint: "hidden-path-exists-fp",
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
	if err := HideVideoLocationsByIDs(ctx, []int64{loc.ID}); err != nil {
		t.Fatalf("hide location: %v", err)
	}

	exists, err := VideoLocationPathExists(ctx, dir.ID, "deleted.mp4")
	if err != nil {
		t.Fatalf("check path exists: %v", err)
	}
	if exists {
		t.Fatal("hidden location should not reserve its path for rename conflict checks")
	}
}

func TestUpdateVideoLocationPathReusesHiddenPath(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1710000000, 0).UTC()

	dir := models.Directory{Path: "/tmp/media"}
	if err := gdb.Create(&dir).Error; err != nil {
		t.Fatalf("create directory: %v", err)
	}
	hiddenVideo := models.Video{
		DirectoryID: dir.ID,
		Path:        "target.mp4",
		Filename:    "target.mp4",
		Fingerprint: "hidden-target-fp",
		ModifiedAt:  now,
	}
	activeVideo := models.Video{
		DirectoryID: dir.ID,
		Path:        "source.mp4",
		Filename:    "source.mp4",
		Fingerprint: "active-source-fp",
		ModifiedAt:  now,
	}
	if err := gdb.Create(&hiddenVideo).Error; err != nil {
		t.Fatalf("create hidden video: %v", err)
	}
	if err := gdb.Create(&activeVideo).Error; err != nil {
		t.Fatalf("create active video: %v", err)
	}
	hiddenLoc, err := UpsertVideoLocation(ctx, hiddenVideo.ID, dir.ID, "target.mp4", now)
	if err != nil {
		t.Fatalf("upsert hidden location: %v", err)
	}
	activeLoc, err := UpsertVideoLocation(ctx, activeVideo.ID, dir.ID, "source.mp4", now)
	if err != nil {
		t.Fatalf("upsert active location: %v", err)
	}
	if err := HideVideoLocationsByIDs(ctx, []int64{hiddenLoc.ID}); err != nil {
		t.Fatalf("hide target location: %v", err)
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
	if locations[0].ID != activeLoc.ID || locations[0].IsDelete {
		t.Fatalf("target path should belong to the active location: %#v", locations[0])
	}
}
