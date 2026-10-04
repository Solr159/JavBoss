package db

import (
	"context"
	"testing"

	"javboss/internal/models"
)

func TestWatchedTimeAtomicityAndHistoricalJavTotal(t *testing.T) {
	database := openTestDB(t)
	ctx := context.Background()
	video := models.Video{Fingerprint: "watch-video"}
	jav := models.Jav{Code: "WATCH-001"}
	for _, row := range []any{&video, &jav} {
		if err := database.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	staleVideo := video
	if err := AddWatchedTime(ctx, video.ID, jav.ID, 1500); err != nil {
		t.Fatal(err)
	}
	staleVideo.DurationSec = 600
	if err := SaveVideo(ctx, &staleVideo); err != nil {
		t.Fatal(err)
	}
	if err := database.First(&video, video.ID).Error; err != nil {
		t.Fatal(err)
	}
	if video.WatchedMS != 1500 {
		t.Fatalf("metadata update lost watch time: %d", video.WatchedMS)
	}
	if err := database.Exec(`CREATE TRIGGER reject_watch BEFORE UPDATE OF watched_ms ON jav BEGIN SELECT RAISE(ABORT, 'test failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	if err := AddWatchedTime(ctx, video.ID, jav.ID, 500); err == nil {
		t.Fatal("expected failed transaction")
	}
	if err := database.First(&video, video.ID).Error; err != nil {
		t.Fatal(err)
	}
	if video.WatchedMS != 1500 {
		t.Fatalf("video update was not rolled back: %d", video.WatchedMS)
	}
	if err := database.Exec(`DROP TRIGGER reject_watch`).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Delete(&video).Error; err != nil {
		t.Fatal(err)
	}
	// The open player can still checkpoint after deleting the video.
	if err := AddWatchedTime(ctx, video.ID, jav.ID, 500); err != nil {
		t.Fatal(err)
	}
	if err := DeleteOrphanJavs(ctx); err != nil {
		t.Fatal(err)
	}
	if err := database.First(&jav, jav.ID).Error; err != nil {
		t.Fatal(err)
	}
	if jav.WatchedMS != 2000 {
		t.Fatalf("historical JAV time=%d", jav.WatchedMS)
	}
}

func TestWatchedTimeSort(t *testing.T) {
	database := openTestDB(t)
	ctx := context.Background()
	dir := models.Directory{Path: "/watch-sort"}
	if err := database.Create(&dir).Error; err != nil {
		t.Fatal(err)
	}
	videos := []models.Video{{Fingerprint: "watch-a", WatchedMS: 1000}, {Fingerprint: "watch-b", WatchedMS: 3000}}
	javs := []models.Jav{{Code: "WATCH-A", WatchedMS: 5000}, {Code: "WATCH-B", WatchedMS: 2000}}
	if err := database.Create(&videos).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&javs).Error; err != nil {
		t.Fatal(err)
	}
	for i := range videos {
		loc := models.VideoLocation{VideoID: videos[i].ID, DirectoryID: dir.ID, RelativePath: javs[i].Code + ".mp4", JavID: &javs[i].ID}
		if err := database.Create(&loc).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		sort                 string
		videoIndex, javIndex int
	}{{"watched", 1, 0}, {"watched_asc", 0, 1}} {
		vs, err := ListVideos(ctx, 10, 0, nil, "", tc.sort, nil, nil)
		if err != nil || len(vs) != 2 || vs[0].ID != videos[tc.videoIndex].ID {
			t.Fatalf("video order %s: %v %v", tc.sort, vs, err)
		}
		js, _, err := SearchJav(ctx, nil, nil, "", tc.sort, 10, 0, nil, nil)
		if err != nil || len(js) != 2 || js[0].ID != javs[tc.javIndex].ID {
			t.Fatalf("jav order %s: %v %v", tc.sort, js, err)
		}
	}
}
