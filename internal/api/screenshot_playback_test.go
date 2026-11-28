package api

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	dbpkg "javboss/internal/db"
	"javboss/internal/models"
)

func TestPlaybackScreenshotDeletionClearsCoverAndPreservesOtherTimes(t *testing.T) {
	dir := t.TempDir()
	database, err := dbpkg.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB, _ := database.DB(); _ = sqlDB.Close() })
	deps := newTestDependencies()
	deps.database = database
	deps.bindDatabase(t)
	video := models.Video{Fingerprint: "screenshot-delete", CoverScreenshotName: "mpv_00-00-02.500.jpg"}
	if err := database.Create(&video).Error; err != nil {
		t.Fatal(err)
	}
	shots := filepath.Join(dir, "video", strconv.FormatInt(video.ID, 10), "screenshot")
	if err := os.MkdirAll(shots, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"mpv_00-00-02.500.jpg", "mpv_00-00-02.500.png", "mpv_00-00-02.501.jpg", "128.jpg"} {
		if err := os.WriteFile(filepath.Join(shots, name), []byte("image"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	deleteScreenshot := localScreenshotDeleter(dir, video.ID)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := deleteScreenshot(ctx, 2.5); err != context.Canceled {
		t.Fatalf("canceled delete: %v", err)
	}
	if err := deleteScreenshot(t.Context(), 2.5); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"mpv_00-00-02.500.jpg", "mpv_00-00-02.500.png"} {
		if _, err := os.Stat(filepath.Join(shots, name)); !os.IsNotExist(err) {
			t.Fatalf("screenshot still exists: %s (%v)", name, err)
		}
	}
	for _, name := range []string{"mpv_00-00-02.501.jpg", "128.jpg"} {
		if _, err := os.Stat(filepath.Join(shots, name)); err != nil {
			t.Fatalf("unrelated screenshot removed: %s (%v)", name, err)
		}
	}
	var got models.Video
	if err := database.First(&got, video.ID).Error; err != nil || got.CoverScreenshotName != "" {
		t.Fatalf("cover not cleared: %+v, %v", got, err)
	}
	if err := deleteScreenshot(t.Context(), 2.5); err != nil {
		t.Fatalf("repeated delete: %v", err)
	}
}
