package db

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"javboss/internal/common"
	"javboss/internal/models"
)

// CompleteVideoTranscode changes existing rows only; no additional schema is
// required. A shared content record is copied for the new encoding so untouched
// copies still resolve by their original fingerprint on subsequent scans.
// copyAssets preserves ID-based screenshots before the transaction commits.
func CompleteVideoTranscode(ctx context.Context, location models.VideoLocation, target, fingerprint string, size, duration int64, modifiedAt time.Time, copyAssets func(int64, int64) error) error {
	return common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current models.VideoLocation
		if err := tx.First(&current, location.ID).Error; err != nil {
			return err
		}
		if current.VideoID != location.VideoID || current.RelativePath != location.RelativePath || !current.ModifiedAt.Equal(location.ModifiedAt) || current.IsDelete != location.IsDelete {
			return errors.New("video location changed during conversion")
		}
		if err := tx.Where("directory_id = ? AND relative_path = ? AND id <> ? AND is_delete = ?", current.DirectoryID, target, current.ID, true).Delete(&models.VideoLocation{}).Error; err != nil {
			return err
		}
		var video models.Video
		if err := tx.First(&video, current.VideoID).Error; err != nil {
			return err
		}
		var existing models.Video
		err := tx.Where("fingerprint = ?", fingerprint).First(&existing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		targetID := video.ID
		if err == nil && existing.ID != video.ID {
			// Identical outputs may already exist from another copy in the same batch.
			// Do not silently overwrite conflicting manual metadata.
			if (video.JavScrapeOverride != "" && existing.JavScrapeOverride != "" && video.JavScrapeOverride != existing.JavScrapeOverride) ||
				(video.CoverScreenshotName != "" && existing.CoverScreenshotName != "" && video.CoverScreenshotName != existing.CoverScreenshotName) {
				return errors.New("converted video conflicts with existing manual metadata")
			}
			targetID = existing.ID
			updates := map[string]any{"play_count": gorm.Expr("MAX(play_count, ?)", video.PlayCount)}
			if existing.JavScrapeOverride == "" {
				updates["jav_scrape_override"] = video.JavScrapeOverride
			}
			if existing.CoverScreenshotName == "" {
				updates["cover_screenshot_name"] = video.CoverScreenshotName
			}
			if err := tx.Model(&models.Video{}).Where("id = ?", targetID).Updates(updates).Error; err != nil {
				return err
			}
		} else {
			var others int64
			if err := tx.Model(&models.VideoLocation{}).Where("video_id = ? AND id <> ?", video.ID, current.ID).Count(&others).Error; err != nil {
				return err
			}
			if others > 0 {
				encoded := video
				encoded.ID, encoded.Fingerprint, encoded.Size, encoded.DurationSec = 0, fingerprint, size, duration
				if err := tx.Omit(clause.Associations).Create(&encoded).Error; err != nil {
					return err
				}
				targetID = encoded.ID
			} else {
				if err := tx.Model(&models.Video{}).Where("id = ?", video.ID).Updates(map[string]any{"fingerprint": fingerprint, "size": size, "duration_sec": duration}).Error; err != nil {
					return err
				}
			}
		}
		if targetID != video.ID {
			if err := tx.Exec(`INSERT INTO video_tag (video_id, tag_id, created_at) SELECT ?, tag_id, created_at FROM video_tag WHERE video_id = ? ON CONFLICT(video_id, tag_id) DO NOTHING`, targetID, video.ID).Error; err != nil {
				return err
			}
			if copyAssets != nil {
				if err := copyAssets(video.ID, targetID); err != nil {
					return fmt.Errorf("copy video assets: %w", err)
				}
			}
		}
		return tx.Model(&models.VideoLocation{}).Where("id = ?", current.ID).Updates(map[string]any{
			"video_id": targetID, "relative_path": target, "filename": filepath.Base(filepath.FromSlash(target)), "modified_at": modifiedAt, "is_delete": false,
		}).Error
	})
}
