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
	deps := newTestDependencies()
	gdb := openTestDB(deps, t)
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
	locations, err := deps.store.VideoLocationsByDirectory(t.Context(), dir.ID)
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
	empty, err := deps.store.VideoLocationsByDirectory(t.Context(), otherDir.ID+1)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty directory: count=%d err=%v", len(empty), err)
	}
}

func TestDeleteVideoLocationsByIDs(t *testing.T) {
	deps := newTestDependencies()
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
			gdb := openTestDB(deps, t)
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
			err := deps.store.DeleteVideoLocationsByIDs(t.Context(), ids)
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
	deps := newTestDependencies()
	gdb := openTestDB(deps, t)
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
	loc, err := deps.store.UpsertVideoLocation(ctx, video.ID, dir.ID, "deleted.mp4", now)
	if err != nil {
		t.Fatalf("upsert location: %v", err)
	}
	if err := deps.store.DeleteVideoLocationsByIDs(ctx, []int64{loc.ID}); err != nil {
		t.Fatalf("delete location: %v", err)
	}

	exists, err := deps.store.VideoLocationPathExists(ctx, dir.ID, "deleted.mp4")
	if err != nil {
		t.Fatalf("check path exists: %v", err)
	}
	if exists {
		t.Fatal("deleted location should not reserve its path for rename conflict checks")
	}
}

func TestUpdateVideoLocationPathReusesDeletedPath(t *testing.T) {
	deps := newTestDependencies()
	gdb := openTestDB(deps, t)
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
	deletedLoc, err := deps.store.UpsertVideoLocation(ctx, deletedVideo.ID, dir.ID, "target.mp4", now)
	if err != nil {
		t.Fatalf("upsert deleted location: %v", err)
	}
	activeLoc, err := deps.store.UpsertVideoLocation(ctx, activeVideo.ID, dir.ID, "source.mp4", now)
	if err != nil {
		t.Fatalf("upsert active location: %v", err)
	}
	if err := deps.store.DeleteVideoLocationsByIDs(ctx, []int64{deletedLoc.ID}); err != nil {
		t.Fatalf("delete target location: %v", err)
	}

	updated, err := deps.store.UpdateVideoLocationPath(ctx, activeLoc.ID, "target.mp4", now.Add(time.Minute))
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

func TestListAutoSubtitleLocationsFiltersAndPages(t *testing.T) {
	deps := newTestDependencies()
	gdb := openTestDB(deps, t)
	ctx := context.Background()

	active := models.Directory{Path: "/tmp/auto-sub-active"}
	missing := models.Directory{Path: "/tmp/auto-sub-missing", Missing: true}
	deleted := models.Directory{Path: "/tmp/auto-sub-deleted", IsDelete: true}
	disabled := models.Directory{Path: "/tmp/auto-sub-disabled"}
	if err := gdb.Create(&active).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&missing).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&deleted).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&disabled).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Model(&models.Directory{}).Where("id = ?", disabled.ID).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}

	video := models.Video{Fingerprint: "auto-subtitle-locations", Size: 1, DurationSec: 60}
	if err := gdb.Create(&video).Error; err != nil {
		t.Fatal(err)
	}
	locations := []models.VideoLocation{
		{VideoID: video.ID, DirectoryID: active.ID, RelativePath: "a.mp4", Filename: "a.mp4"},
		{VideoID: video.ID, DirectoryID: missing.ID, RelativePath: "b.mp4", Filename: "b.mp4"},
		{VideoID: video.ID, DirectoryID: deleted.ID, RelativePath: "c.mp4", Filename: "c.mp4"},
		{VideoID: video.ID, DirectoryID: disabled.ID, RelativePath: "d.mp4", Filename: "d.mp4"},
		{VideoID: video.ID, DirectoryID: active.ID, RelativePath: "e.mp4", Filename: "e.mp4"},
	}
	if err := gdb.Create(&locations).Error; err != nil {
		t.Fatal(err)
	}

	first, err := deps.store.ListAutoSubtitleLocations(ctx, 0, 1)
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if len(first) != 1 || first[0].RelativePath != "a.mp4" {
		t.Fatalf("unexpected first page: %#v", first)
	}
	if first[0].DirectoryRef.Path != active.Path {
		t.Fatalf("directory path = %q, want %q", first[0].DirectoryRef.Path, active.Path)
	}

	recent, err := deps.store.ListRecentAutoSubtitleLocations(ctx, 10)
	if err != nil {
		t.Fatalf("recent locations: %v", err)
	}
	if len(recent) != 2 || recent[0].RelativePath != "e.mp4" || recent[1].RelativePath != "a.mp4" {
		t.Fatalf("unexpected recent locations: %#v", recent)
	}

	second, err := deps.store.ListAutoSubtitleLocations(ctx, first[0].ID, 10)
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	if len(second) != 1 || second[0].RelativePath != "e.mp4" {
		t.Fatalf("unexpected second page: %#v", second)
	}

	none, err := deps.store.ListAutoSubtitleLocations(ctx, second[0].ID, 10)
	if err != nil {
		t.Fatalf("end page: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("expected no more locations, got %#v", none)
	}
}
