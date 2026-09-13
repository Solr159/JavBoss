package db

import (
	"errors"
	"testing"
	"time"

	"javboss/internal/models"
)

func TestCompleteVideoTranscodePreservesAndDeduplicatesMetadata(t *testing.T) {
	for _, copies := range []int{1, 2} {
		t.Run(string(rune('0'+copies)), func(t *testing.T) {
			gdb := openTestDB(t)
			dir := models.Directory{Path: t.TempDir()}
			video := models.Video{Fingerprint: "original", PlayCount: 12, JavScrapeOverride: ":manual:ABC-123", CoverScreenshotName: "custom.jpg", CreatedAt: time.Unix(1700000000, 0)}
			jav := models.Jav{Code: "ABC-123"}
			tag := models.Tag{Name: "favorite"}
			for _, item := range []any{&dir, &video, &jav, &tag} {
				if err := gdb.Create(item).Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := gdb.Create(&models.VideoTag{VideoID: video.ID, TagID: tag.ID}).Error; err != nil {
				t.Fatal(err)
			}
			locations := make([]models.VideoLocation, copies)
			for i := range locations {
				locations[i] = models.VideoLocation{VideoID: video.ID, DirectoryID: dir.ID, RelativePath: string(rune('a'+i)) + ".mkv", JavID: &jav.ID, ModifiedAt: time.Unix(1710000000, 0).UTC()}
				if err := gdb.Create(&locations[i]).Error; err != nil {
					t.Fatal(err)
				}
			}
			var outputID int64
			for i, location := range locations {
				target := string(rune('a'+i)) + ".mp4"
				if err := CompleteVideoTranscode(t.Context(), location, target, "converted", 1234, 30, time.Now(), nil); err != nil {
					t.Fatal(err)
				}
				var stored models.VideoLocation
				if err := gdb.Preload("Video").Preload("Video.Tags").First(&stored, location.ID).Error; err != nil {
					t.Fatal(err)
				}
				if stored.RelativePath != target || stored.JavID == nil || *stored.JavID != jav.ID || stored.Video.PlayCount != 12 || len(stored.Video.Tags) != 1 || stored.Video.Tags[0].ID != tag.ID || stored.Video.JavScrapeOverride != video.JavScrapeOverride || stored.Video.CoverScreenshotName != video.CoverScreenshotName || !stored.Video.CreatedAt.Equal(video.CreatedAt) {
					t.Fatalf("metadata not preserved: %+v video=%+v", stored, stored.Video)
				}
				if i == 0 {
					outputID = stored.VideoID
				} else if stored.VideoID != outputID {
					t.Fatal("identical output was not deduplicated")
				}
			}
			if copies == 1 && outputID != video.ID {
				t.Fatal("single-file conversion changed video ID")
			}
			var count int64
			if err := gdb.Model(&models.Video{}).Where("fingerprint = ?", "converted").Count(&count).Error; err != nil || count != 1 {
				t.Fatalf("converted rows=%d error=%v", count, err)
			}
		})
	}
}

func TestCompleteVideoTranscodeRollsBackOnAssetFailure(t *testing.T) {
	gdb := openTestDB(t)
	dir := models.Directory{Path: t.TempDir()}
	video := models.Video{Fingerprint: "original", PlayCount: 5}
	for _, item := range []any{&dir, &video} {
		if err := gdb.Create(item).Error; err != nil {
			t.Fatal(err)
		}
	}
	locations := []models.VideoLocation{
		{DirectoryID: dir.ID, VideoID: video.ID, RelativePath: "a.mkv"},
		{DirectoryID: dir.ID, VideoID: video.ID, RelativePath: "b.mkv"},
	}
	if err := gdb.Create(&locations).Error; err != nil {
		t.Fatal(err)
	}
	err := CompleteVideoTranscode(t.Context(), locations[0], "a.mp4", "converted", 100, 10, time.Now(), func(int64, int64) error { return errors.New("disk full") })
	if err == nil {
		t.Fatal("expected asset failure")
	}
	var stored models.VideoLocation
	if err := gdb.First(&stored, locations[0].ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.VideoID != video.ID || stored.RelativePath != "a.mkv" {
		t.Fatalf("location changed after rollback: %+v", stored)
	}
	var count int64
	gdb.Model(&models.Video{}).Count(&count)
	if count != 1 {
		t.Fatalf("orphan encoding after rollback: %d", count)
	}
}
