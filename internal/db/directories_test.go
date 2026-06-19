package db

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"javboss/internal/models"
)

func TestUpdateDirectoryPathHidesExistingVideoLocations(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1710000000, 0).UTC()

	oldRoot := t.TempDir()
	newRoot := t.TempDir()
	dir := models.Directory{Path: oldRoot}
	if err := gdb.Create(&dir).Error; err != nil {
		t.Fatalf("create directory: %v", err)
	}
	video := models.Video{
		DirectoryID: dir.ID,
		Path:        "movie.mp4",
		Filename:    "movie.mp4",
		Fingerprint: "movie-fingerprint",
		Size:        1024,
		DurationSec: 120,
		ModifiedAt:  now,
	}
	if err := gdb.Create(&video).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}
	loc, err := UpsertVideoLocation(ctx, video.ID, dir.ID, "movie.mp4", now)
	if err != nil {
		t.Fatalf("upsert video location: %v", err)
	}

	updated, err := UpdateDirectory(ctx, dir.ID, &newRoot, nil)
	if err != nil {
		t.Fatalf("update directory path: %v", err)
	}
	if updated == nil {
		t.Fatal("updated directory is nil")
	}
	if updated.Path != filepath.Clean(newRoot) {
		t.Fatalf("unexpected updated path: got %q want %q", updated.Path, filepath.Clean(newRoot))
	}

	var hidden models.VideoLocation
	if err := gdb.First(&hidden, loc.ID).Error; err != nil {
		t.Fatalf("load hidden location: %v", err)
	}
	if !hidden.IsDelete {
		t.Fatal("existing location should be hidden immediately after directory path changes")
	}

	items, err := ListVideos(ctx, 20, 0, nil, "", "recent", nil, []int64{dir.ID})
	if err != nil {
		t.Fatalf("list videos after hiding locations: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("hidden locations should not be listed before rescan: %#v", items)
	}

	recovered, err := UpsertVideoLocation(ctx, video.ID, dir.ID, "movie.mp4", now)
	if err != nil {
		t.Fatalf("restore video location: %v", err)
	}
	if recovered.ID != loc.ID {
		t.Fatalf("location should be restored in place: got id %d want %d", recovered.ID, loc.ID)
	}
	if recovered.IsDelete {
		t.Fatal("location should be visible after scan upsert")
	}
}
