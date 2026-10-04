package db

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"javboss/internal/common"
	"javboss/internal/models"
)

func GetVideoLocationByPath(ctx context.Context, dirPath, relativePath string) (*models.VideoLocation, error) {
	var loc models.VideoLocation
	err := common.DB.WithContext(ctx).Model(&models.VideoLocation{}).
		Joins("JOIN directory ON directory.id = video_location.directory_id").
		Where("directory.path = ? AND video_location.relative_path = ?", dirPath, cleanRelativePathForDB(relativePath)).
		Where(activeLocationWhereSQL("video_location", "directory")).First(&loc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &loc, err
}

// AddWatchedTime keeps historical JAV totals independent of video locations.
// Missing rows are allowed: a video may be deleted while its player is open.
func AddWatchedTime(ctx context.Context, videoID, javID, deltaMS int64) error {
	if videoID <= 0 || deltaMS < 0 {
		return fmt.Errorf("invalid watched time increment")
	}
	if deltaMS == 0 {
		return nil
	}
	return common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Video{}).Where("id = ?", videoID).
			UpdateColumn("watched_ms", gorm.Expr("watched_ms + ?", deltaMS)).Error; err != nil {
			return fmt.Errorf("update video watched time: %w", err)
		}
		if javID > 0 {
			if err := tx.Model(&models.Jav{}).Where("id = ?", javID).
				UpdateColumn("watched_ms", gorm.Expr("watched_ms + ?", deltaMS)).Error; err != nil {
				return fmt.Errorf("update jav watched time: %w", err)
			}
		}
		return nil
	})
}
