package db

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"javboss/internal/common"
	"javboss/internal/models"
)

var ErrJavVideoShared = errors.New("video is linked to another jav")

// DeleteJavVideos removes all copies of a work's videos, including locations in disabled or deleted directories,
// while preserving the JAV and all its metadata associations.
// cleanup runs under the database write lock before any records are removed. A file
// failure leaves records available for retry; filesystem changes cannot be rolled back.
func DeleteJavVideos(ctx context.Context, id int64, cleanup func([]models.VideoLocation, []int64) error) ([]int64, error) {
	var videoIDs []int64
	err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Jav{}).Where("id = ?", id).UpdateColumn("id", id).Error; err != nil {
			return err
		}
		var item models.Jav
		if err := tx.First(&item, id).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.VideoLocation{}).Where("jav_id = ?", id).Distinct().Pluck("video_id", &videoIDs).Error; err != nil {
			return err
		}
		var locations []models.VideoLocation
		if len(videoIDs) > 0 {
			if err := tx.Where("video_id IN ?", videoIDs).Preload("DirectoryRef").Find(&locations).Error; err != nil {
				return err
			}
			for _, loc := range locations {
				if loc.JavID != nil && *loc.JavID != id {
					return ErrJavVideoShared
				}
			}
		}
		if err := cleanup(locations, videoIDs); err != nil {
			return fmt.Errorf("delete jav video files: %w", err)
		}
		if len(videoIDs) > 0 {
			if err := tx.Where("video_id IN ?", videoIDs).Delete(&models.VideoTag{}).Error; err != nil {
				return err
			}
			if err := tx.Where("video_id IN ?", videoIDs).Delete(&models.VideoLocation{}).Error; err != nil {
				return err
			}
			if err := tx.Where("id IN ?", videoIDs).Delete(&models.Video{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return videoIDs, err
}
