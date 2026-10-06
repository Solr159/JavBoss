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
		Where(activeDirectoryWhereSQL("directory")).First(&loc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &loc, err
}

// AddWatchedTime keeps historical JAV totals independent of video locations.
// Missing rows are allowed: a video may be deleted while its player is open.
func AddWatchedTime(ctx context.Context, videoID, javID, deltaMS int64) error {
	_, err := AddWatchedTimeWithTotals(ctx, videoID, javID, deltaMS)
	return err
}

type WatchedTimeTotal struct {
	ID        int64 `json:"id"`
	WatchedMS int64 `json:"watched_ms"`
}

type WatchedTimeSnapshot struct {
	Videos []WatchedTimeTotal `json:"videos"`
	Javs   []WatchedTimeTotal `json:"javs"`
}

// AddWatchedTimeWithTotals returns the totals from the same transaction, only
// after it commits. Callers may then notify viewers without publishing rollbacks.
func AddWatchedTimeWithTotals(ctx context.Context, videoID, javID, deltaMS int64) (WatchedTimeSnapshot, error) {
	var snapshot WatchedTimeSnapshot
	if videoID <= 0 || deltaMS < 0 {
		return snapshot, fmt.Errorf("invalid watched time increment")
	}
	if deltaMS == 0 {
		return snapshot, nil
	}
	err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
		var err error
		snapshot, err = readWatchedTimes(tx, []int64{videoID}, []int64{javID})
		return err
	})
	if err != nil {
		return WatchedTimeSnapshot{}, err
	}
	return snapshot, nil
}

// GetWatchedTimes reads just the counters, without reloading or reordering lists.
func GetWatchedTimes(ctx context.Context, videoIDs, javIDs []int64) (WatchedTimeSnapshot, error) {
	var snapshot WatchedTimeSnapshot
	err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		snapshot, err = readWatchedTimes(tx, videoIDs, javIDs)
		return err
	})
	return snapshot, err
}

func readWatchedTimes(tx *gorm.DB, videoIDs, javIDs []int64) (WatchedTimeSnapshot, error) {
	snapshot := WatchedTimeSnapshot{Videos: []WatchedTimeTotal{}, Javs: []WatchedTimeTotal{}}
	if len(videoIDs) > 0 {
		if err := tx.Model(&models.Video{}).Select("id, watched_ms").Where("id IN ?", videoIDs).Find(&snapshot.Videos).Error; err != nil {
			return snapshot, fmt.Errorf("read video watched time: %w", err)
		}
	}
	if len(javIDs) > 0 {
		if err := tx.Model(&models.Jav{}).Select("id, watched_ms").Where("id IN ?", javIDs).Find(&snapshot.Javs).Error; err != nil {
			return snapshot, fmt.Errorf("read jav watched time: %w", err)
		}
	}
	return snapshot, nil
}
