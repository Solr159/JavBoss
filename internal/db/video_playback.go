package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"javboss/internal/models"

	"gorm.io/gorm"
)

// GetVideoForPlayback reads only playback metadata and one active file location
// in a single query. A zero locationID selects the first active location, just
// like GetVideo. The returned partial Video must not be saved as a full record.
func (store *Store) GetVideoForPlayback(ctx context.Context, videoID, locationID int64) (*models.Video, error) {
	if videoID <= 0 || locationID < 0 {
		return nil, errors.New("valid video id and non-negative location id are required")
	}
	var row struct {
		ID            int64
		LocationID    int64
		DirectoryID   int64
		DirectoryPath string
		Path          string
		Size          int64
		ModifiedAt    time.Time
		DurationSec   int64
		MediaInfo     models.MediaInfo `gorm:"serializer:json"`
	}
	query := store.conn.WithContext(ctx).
		Table("video_location").
		Select(`video.id, video.size, video.duration_sec, video.media_info,
			video_location.id AS location_id, video_location.directory_id,
			video_location.relative_path AS path, video_location.modified_at,
			directory.path AS directory_path`).
		Joins("JOIN video ON video.id = video_location.video_id").
		Joins("JOIN directory ON directory.id = video_location.directory_id").
		Where("video.id = ?", videoID).
		Where(activeDirectoryWhereSQL("directory"))
	if locationID > 0 {
		query = query.Where("video_location.id = ?", locationID)
	}
	if err := query.Order("video_location.id").Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get video for playback: %w", err)
	}
	return &models.Video{
		ID:           row.ID,
		LocationID:   row.LocationID,
		DirectoryID:  row.DirectoryID,
		Path:         row.Path,
		Size:         row.Size,
		ModifiedAt:   row.ModifiedAt,
		DurationSec:  row.DurationSec,
		MediaInfo:    row.MediaInfo,
		DirectoryRef: models.Directory{ID: row.DirectoryID, Path: row.DirectoryPath},
	}, nil
}
