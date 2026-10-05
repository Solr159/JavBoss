package manager_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"javboss/internal/manager"
	"javboss/internal/models"
)

func TestGetThumbnailReturnsCachedImageWithoutGenerationMetadata(t *testing.T) {
	dataDir := t.TempDir()
	path := filepath.Join(dataDir, "video", "1", "screenshot", "128.jpg")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("cached image"), 0600); err != nil {
		t.Fatal(err)
	}
	m := manager.NewScreenshotManager(dataDir, func(context.Context, int64) (*models.Video, error) {
		t.Error("cached thumbnail must not fetch or regenerate the video")
		return nil, nil
	})
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	// Reading a cached image needs no location mtime or running screenshot workers.
	got, err := m.GetThumbnail(ctx, &models.Video{ID: 1, DurationSec: 300})
	if err != nil || got != path {
		t.Fatalf("GetThumbnail = %q, %v; want %q", got, err, path)
	}
}

func TestGetThumbnailReportsUnavailableTimestamp(t *testing.T) {
	m := manager.NewScreenshotManager(t.TempDir(), func(context.Context, int64) (*models.Video, error) { return nil, nil })
	for _, video := range []*models.Video{nil, {ID: 1, DurationSec: 0}, {ID: 1, DurationSec: -1}} {
		if _, err := m.GetThumbnail(t.Context(), video); !errors.Is(err, manager.ErrNoThumbnail) {
			t.Fatalf("GetThumbnail(%+v) = %v, want ErrNoThumbnail", video, err)
		}
	}
}
