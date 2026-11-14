package metadata

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/db"
	"javboss/internal/models"
	"javboss/internal/util"
)

// These gates only coordinate concurrent first-use backfills; no metadata is
// cached in memory. Never hold a database transaction while running ffprobe.
var backfillGates = func() [64]chan struct{} {
	var gates [64]chan struct{}
	for i := range gates {
		gates[i] = make(chan struct{}, 1)
	}
	return gates
}()

var ErrFileChanged = errors.New("video file has changed; rescan required")

// CheckFile rejects files that no longer match the identity recorded by the scanner.
func CheckFile(path string, size int64, modifiedAt time.Time) (os.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() != size || !info.ModTime().Equal(modifiedAt) {
		return nil, ErrFileChanged
	}
	return info, nil
}

func Load(ctx context.Context, store *db.Store, video *models.Video, path string) (*models.MediaInfo, error) {
	return load(ctx, store, video, path, util.ProbeVideoContext)
}

func load(ctx context.Context, store *db.Store, video *models.Video, path string, probe func(context.Context, string) (*util.VideoMetadata, error)) (*models.MediaInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := CheckFile(path, video.Size, video.ModifiedAt); err != nil {
		return nil, err
	}
	if video.MediaInfo.Complete() {
		logging.Info("playback media_info reuse: video_id=%d location_id=%d path=%q source=database", video.ID, video.LocationID, path)
		info := video.MediaInfo
		return &info, nil
	}
	gate := backfillGates[uint64(video.ID)%uint64(len(backfillGates))]
	select {
	case gate <- struct{}{}:
		defer func() { <-gate }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	// Another request or scan may have populated the row while we were waiting.
	current, err := store.GetVideoForPlayback(ctx, video.ID, video.LocationID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, os.ErrNotExist
	}
	if filepath.Clean(filepath.Join(current.DirectoryRef.Path, filepath.FromSlash(current.Path))) != filepath.Clean(path) {
		return nil, fmt.Errorf("video location changed during playback preparation: %w", ErrFileChanged)
	}
	before, err := CheckFile(path, current.Size, current.ModifiedAt)
	if err != nil {
		return nil, err
	}
	if current.MediaInfo.Complete() {
		logging.Info("playback media_info reuse: video_id=%d location_id=%d path=%q source=database after_wait=true", current.ID, current.LocationID, path)
		info := current.MediaInfo
		return &info, nil
	}
	logging.Info("playback media_info probe required: video_id=%d location_id=%d path=%q reason=missing_or_outdated_media_info", current.ID, current.LocationID, path)
	info, err := probe(ctx, path)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	after, err := CheckFile(path, current.Size, current.ModifiedAt)
	if err != nil {
		return nil, err
	}
	if !os.SameFile(before, after) {
		return nil, ErrFileChanged
	}
	filled, err := store.FillVideoMediaInfo(ctx, current.ID, *info)
	if err != nil {
		logging.Error("playback media_info persist failed: video_id=%d location_id=%d path=%q err=%v", current.ID, current.LocationID, path, err)
		return nil, err
	}
	logging.Info("playback media_info persisted: video_id=%d location_id=%d path=%q updated=%t", current.ID, current.LocationID, path, filled)
	return info, nil
}
