package db

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"javboss/internal/common"
	"javboss/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrVideoLocationPathConflict is returned when a location already uses the target path.
var ErrVideoLocationPathConflict = errors.New("video location path already exists")

// AllVideoLocations returns every known video location; used for scan bookkeeping.
func AllVideoLocations(ctx context.Context) ([]models.VideoLocation, error) {
	var locations []models.VideoLocation
	if err := common.DB.WithContext(ctx).Find(&locations).Error; err != nil {
		return nil, fmt.Errorf("load video locations: %w", err)
	}
	return locations, nil
}

// VideoLocationsByDirectory returns every known location in a directory, without filtering directory visibility.
func VideoLocationsByDirectory(ctx context.Context, directoryID int64) ([]models.VideoLocation, error) {
	if directoryID <= 0 {
		return nil, errors.New("directory id cannot be zero")
	}
	var locations []models.VideoLocation
	// Join the belongs-to relation so loading a large directory does not expand
	// every video ID into a Preload IN query and exceed SQLite's parameter limit.
	if err := common.DB.WithContext(ctx).
		Where("video_location.directory_id = ?", directoryID).
		Joins("Video").
		Find(&locations).Error; err != nil {
		return nil, fmt.Errorf("load video locations for directory %d: %w", directoryID, err)
	}
	return locations, nil
}

// UpsertVideoLocation records or updates the on-disk location for a video.
func UpsertVideoLocation(ctx context.Context, videoID, directoryID int64, relativePath string, modifiedAt time.Time) (*models.VideoLocation, error) {
	relativePath = cleanRelativePathForDB(relativePath)
	filename := filepath.Base(filepath.FromSlash(relativePath))
	if videoID <= 0 || directoryID <= 0 || relativePath == "" {
		return nil, errors.New("video_id, directory_id, and relative_path are required")
	}

	loc := models.VideoLocation{
		VideoID:      videoID,
		DirectoryID:  directoryID,
		RelativePath: relativePath,
		Filename:     filename,
		ModifiedAt:   modifiedAt,
	}
	tx := common.DB.WithContext(ctx)
	if err := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "directory_id"}, {Name: "relative_path"}},
		DoUpdates: clause.Assignments(map[string]any{
			"video_id":    videoID,
			"filename":    filename,
			"modified_at": modifiedAt,
			"updated_at":  gorm.Expr("CURRENT_TIMESTAMP"),
		}),
	}).Create(&loc).Error; err != nil {
		return nil, fmt.Errorf("upsert video location: %w", err)
	}

	var saved models.VideoLocation
	if err := tx.
		Where("directory_id = ? AND relative_path = ?", directoryID, relativePath).
		First(&saved).Error; err != nil {
		return nil, fmt.Errorf("load saved video location: %w", err)
	}
	return &saved, nil
}

// DeleteVideoLocationsByIDs removes file locations while preserving video metadata and history.
func DeleteVideoLocationsByIDs(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	// Keep batches below SQLite's older 999-variable limit and delete them atomically.
	const batchSize = 400
	err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for start := 0; start < len(ids); start += batchSize {
			batch := ids[start:min(start+batchSize, len(ids))]
			if err := tx.Model(&models.VideoLocation{}).
				Where("id IN ?", batch).
				Delete(&models.VideoLocation{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("delete video locations: %w", err)
	}
	return nil
}

// GetVideoIDByPath returns the video ID for an active directory path + relative path.
func GetVideoIDByPath(ctx context.Context, dirPath, relPath string) (int64, error) {
	if strings.TrimSpace(dirPath) == "" || strings.TrimSpace(relPath) == "" {
		return 0, errors.New("directory path and relative path are required")
	}

	var loc models.VideoLocation
	err := common.DB.WithContext(ctx).
		Model(&models.VideoLocation{}).
		Select("video_location.video_id").
		Joins("JOIN directory ON directory.id = video_location.directory_id").
		Joins("JOIN video ON video.id = video_location.video_id").
		Where("directory.path = ?", dirPath).
		Where("video_location.relative_path = ?", cleanRelativePathForDB(relPath)).
		Where(activeDirectoryWhereSQL("directory")).
		Where("COALESCE(directory.missing, 0) = 0").
		First(&loc).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, fmt.Errorf("lookup video by path: %w", err)
	}

	return loc.VideoID, nil
}

// GetPrimaryVideoLocation returns the preferred active location for a video.
func GetPrimaryVideoLocation(ctx context.Context, videoID int64) (*models.VideoLocation, error) {
	if videoID <= 0 {
		return nil, errors.New("video id cannot be zero")
	}
	var loc models.VideoLocation
	err := common.DB.WithContext(ctx).
		Model(&models.VideoLocation{}).
		Joins("JOIN directory ON directory.id = video_location.directory_id").
		Where("video_location.video_id = ?", videoID).
		Where(activeDirectoryWhereSQL("directory")).
		Order("video_location.id").
		Preload("DirectoryRef").
		First(&loc).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get primary video location: %w", err)
	}
	return &loc, nil
}

// GetActiveVideoLocation returns a specific active location for a video.
func GetActiveVideoLocation(ctx context.Context, videoID, locationID int64) (*models.VideoLocation, error) {
	if videoID <= 0 || locationID <= 0 {
		return nil, errors.New("video id and location id are required")
	}
	var loc models.VideoLocation
	err := common.DB.WithContext(ctx).
		Model(&models.VideoLocation{}).
		Joins("JOIN directory ON directory.id = video_location.directory_id").
		Where("video_location.id = ?", locationID).
		Where("video_location.video_id = ?", videoID).
		Where(activeDirectoryWhereSQL("directory")).
		Preload("DirectoryRef").
		First(&loc).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get active video location: %w", err)
	}
	return &loc, nil
}

// VideoLocationPathExists reports whether any active location row already uses the directory/path pair.
func VideoLocationPathExists(ctx context.Context, directoryID int64, relativePath string) (bool, error) {
	relativePath = cleanRelativePathForDB(relativePath)
	if directoryID <= 0 || relativePath == "" {
		return false, errors.New("directory id and relative path are required")
	}
	var count int64
	if err := common.DB.WithContext(ctx).
		Model(&models.VideoLocation{}).
		Where("directory_id = ? AND relative_path = ?", directoryID, relativePath).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check video location path: %w", err)
	}
	return count > 0, nil
}

// UpdateVideoLocationPath updates the stored path metadata for a location after a filesystem rename.
func UpdateVideoLocationPath(ctx context.Context, locationID int64, relativePath string, modifiedAt time.Time) (*models.VideoLocation, error) {
	relativePath = cleanRelativePathForDB(relativePath)
	if locationID <= 0 || relativePath == "" {
		return nil, errors.New("location id and relative path are required")
	}
	updates := map[string]any{
		"relative_path": relativePath,
		"filename":      filepath.Base(filepath.FromSlash(relativePath)),
		"modified_at":   modifiedAt,
		"updated_at":    gorm.Expr("CURRENT_TIMESTAMP"),
	}

	var loc models.VideoLocation
	err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current models.VideoLocation
		if err := tx.
			Model(&models.VideoLocation{}).
			Where("id = ?", locationID).
			First(&current).Error; err != nil {
			return fmt.Errorf("load current video location: %w", err)
		}

		var activeConflicts int64
		if err := tx.
			Model(&models.VideoLocation{}).
			Where("directory_id = ? AND relative_path = ? AND id <> ?", current.DirectoryID, relativePath, locationID).
			Count(&activeConflicts).Error; err != nil {
			return fmt.Errorf("check active video location path: %w", err)
		}
		if activeConflicts > 0 {
			return ErrVideoLocationPathConflict
		}

		if err := tx.
			Model(&models.VideoLocation{}).
			Where("id = ?", locationID).
			Updates(updates).Error; err != nil {
			return fmt.Errorf("save video location path: %w", err)
		}

		if err := tx.
			Model(&models.VideoLocation{}).
			Where("id = ?", locationID).
			Preload("DirectoryRef").
			Preload("Video").
			Preload("Video.Tags").
			First(&loc).Error; err != nil {
			return fmt.Errorf("load updated video location: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("update video location path: %w", err)
	}
	return &loc, nil
}

func activeVideoLocationSubquery(ctx context.Context) *gorm.DB {
	return common.DB.WithContext(ctx).
		Table("video_location vl").
		Select("1").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Where("vl.video_id = video.id").
		Where("COALESCE(d.is_delete, 0) = 0").
		Where("COALESCE(d.enabled, 1) <> 0")
}

func activeDirectoryWhereSQL(directoryAlias string) string {
	directoryAlias = strings.TrimSpace(directoryAlias)
	if directoryAlias == "" {
		directoryAlias = "directory"
	}
	return fmt.Sprintf(
		"COALESCE(%s.is_delete, 0) = 0 AND COALESCE(%s.enabled, 1) <> 0",
		directoryAlias,
		directoryAlias,
	)
}

func applyDirectoryFilter(q *gorm.DB, locationAlias string, directoryIDs []int64) *gorm.DB {
	if hasZeroID(directoryIDs) {
		return q.Where("1 = 0")
	}
	cleanIDs := uniqueInt64s(directoryIDs)
	if len(cleanIDs) == 0 {
		return q
	}
	locationAlias = strings.TrimSpace(locationAlias)
	if locationAlias == "" {
		locationAlias = "video_location"
	}
	return q.Where(locationAlias+".directory_id IN ?", cleanIDs)
}

func directoryFilterSQL(locationAlias string, directoryIDs []int64) string {
	if hasZeroID(directoryIDs) {
		return " AND 1 = 0"
	}
	cleanIDs := uniqueInt64s(directoryIDs)
	if len(cleanIDs) == 0 {
		return ""
	}
	locationAlias = strings.TrimSpace(locationAlias)
	if locationAlias == "" {
		locationAlias = "video_location"
	}
	parts := make([]string, 0, len(cleanIDs))
	for _, id := range cleanIDs {
		parts = append(parts, fmt.Sprintf("%d", id))
	}
	return " AND " + locationAlias + ".directory_id IN (" + strings.Join(parts, ",") + ")"
}

func hasZeroID(ids []int64) bool {
	for _, id := range ids {
		if id == 0 {
			return true
		}
	}
	return false
}

func preloadActiveLocations(db *gorm.DB) *gorm.DB {
	return preloadActiveLocationsWhere("")(db)
}

func preloadActiveLocationsWhere(extraWhere string) func(*gorm.DB) *gorm.DB {
	extraWhere = strings.TrimSpace(extraWhere)
	return func(db *gorm.DB) *gorm.DB {
		return db.
			Preload("Locations", func(tx *gorm.DB) *gorm.DB {
				tx = tx.
					Joins("JOIN directory ON directory.id = video_location.directory_id").
					Where(activeDirectoryWhereSQL("directory"))
				if extraWhere != "" {
					tx = tx.Where(extraWhere)
				}
				return tx.Order("video_location.id")
			}).
			Preload("Locations.DirectoryRef")
	}
}

// ReconcileAllVideoPaths is kept for older call sites; video path fields are now
// derived from VideoLocation at read time.
func ReconcileAllVideoPaths(ctx context.Context) error {
	_ = ctx
	return nil
}

func cleanRelativePathForDB(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	cleaned := filepath.Clean(filepath.FromSlash(p))
	if cleaned == "." {
		return ""
	}
	return filepath.ToSlash(cleaned)
}
