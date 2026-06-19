package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"javboss/internal/common"
	"javboss/internal/models"

	"gorm.io/gorm"
)

func normalizeDirectoryPath(p string) (string, error) {
	if strings.TrimSpace(p) == "" {
		return "", errors.New("directory path cannot be empty")
	}
	cleaned := filepath.Clean(p)
	if !filepath.IsAbs(cleaned) {
		abs, err := filepath.Abs(cleaned)
		if err != nil {
			return "", fmt.Errorf("resolve directory: %w", err)
		}
		cleaned = abs
	}
	if info, err := os.Stat(cleaned); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("directory %q does not exist", cleaned)
		}
		return "", fmt.Errorf("stat directory: %w", err)
	} else if !info.IsDir() {
		return "", fmt.Errorf("path %q is not a directory", cleaned)
	}
	return cleaned, nil
}

// ListDirectories returns all directories regardless of status.
func ListDirectories(ctx context.Context) ([]models.Directory, error) {
	var dirs []models.Directory
	if err := common.DB.WithContext(ctx).Order("id").Find(&dirs).Error; err != nil {
		return nil, fmt.Errorf("list directories: %w", err)
	}
	return dirs, nil
}

// ListActiveDirectories returns directories that are not marked as deleted.
func ListActiveDirectories(ctx context.Context) ([]models.Directory, error) {
	var dirs []models.Directory
	if err := common.DB.WithContext(ctx).
		Where("COALESCE(is_delete, 0) = 0").
		Order("id").
		Find(&dirs).Error; err != nil {
		return nil, fmt.Errorf("list active directories: %w", err)
	}
	return dirs, nil
}

// GetDirectory fetches a single directory by id.
func GetDirectory(ctx context.Context, id int64) (*models.Directory, error) {
	if id == 0 {
		return nil, errors.New("directory id cannot be zero")
	}
	var dir models.Directory
	if err := common.DB.WithContext(ctx).First(&dir, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get directory: %w", err)
	}
	return &dir, nil
}

// CreateDirectory registers a new directory.
func CreateDirectory(ctx context.Context, path string) (*models.Directory, error) {
	normalized, err := normalizeDirectoryPath(path)
	if err != nil {
		return nil, err
	}

	var existing models.Directory
	err = common.DB.WithContext(ctx).Where("path = ?", normalized).First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	// If directory exists but was soft-deleted, restore it instead of inserting a new row.
	if err == nil {
		if existing.IsDelete {
			dir, updErr := updateDirectoryWithVisibility(ctx, existing.ID, func(tx *gorm.DB, dir *models.Directory) error {
				dir.Path = normalized
				dir.IsDelete = false
				dir.Missing = false
				return nil
			})
			if updErr != nil {
				return nil, fmt.Errorf("restore directory: %w", updErr)
			}
			return dir, nil
		}
		return nil, fmt.Errorf("directory %q already exists", normalized)
	}

	dir := models.Directory{
		Path: normalized,
	}
	if err := common.DB.WithContext(ctx).Create(&dir).Error; err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}
	return &dir, nil
}

// UpdateDirectory updates a directory's attributes. Pass nil to leave a field untouched.
func UpdateDirectory(ctx context.Context, id int64, path *string, isDelete *bool) (*models.Directory, error) {
	var normalizedPath *string
	if path != nil {
		normalized, err := normalizeDirectoryPath(*path)
		if err != nil {
			return nil, err
		}
		normalizedPath = &normalized
	}

	return updateDirectoryWithVisibility(ctx, id, func(tx *gorm.DB, dir *models.Directory) error {
		if normalizedPath != nil && dir.Path != *normalizedPath {
			var other models.Directory
			if err := tx.Where("path = ?", *normalizedPath).First(&other).Error; err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return fmt.Errorf("lookup conflicting directory: %w", err)
				}
			} else if other.ID != dir.ID {
				if other.IsDelete {
					// Restore the soft-deleted record (other) and mark current dir as deleted instead.
					if err := tx.Model(&models.Directory{}).
						Where("id = ?", other.ID).
						Updates(map[string]any{"is_delete": false, "missing": false}).Error; err != nil {
						return fmt.Errorf("restore deleted directory: %w", err)
					}
					dir.IsDelete = true
					// Keep dir.Path unchanged to avoid uniqueness conflict; caller attempted to reuse other's path.
					normalizedPath = nil
				} else {
					return fmt.Errorf("directory %q already exists", *normalizedPath)
				}
			}
			if normalizedPath != nil {
				dir.Path = *normalizedPath
				if err := hideVideoLocationsByDirectoryID(tx, dir.ID); err != nil {
					return err
				}
			}
			dir.Missing = false
		}
		if isDelete != nil {
			dir.IsDelete = *isDelete
		}
		return nil
	})
}

// SetDirectoryMissingAndHideVideos updates the directory missing flag and hides/unhides its videos atomically.
func SetDirectoryMissingAndHideVideos(ctx context.Context, id int64, missing bool) error {
	_, err := updateDirectoryWithVisibility(ctx, id, func(tx *gorm.DB, dir *models.Directory) error {
		dir.Missing = missing
		return nil
	})
	return err
}

func hideVideoLocationsByDirectoryID(tx *gorm.DB, directoryID int64) error {
	if directoryID <= 0 {
		return errors.New("directory id cannot be zero")
	}
	if err := tx.
		Model(&models.VideoLocation{}).
		Where("directory_id = ?", directoryID).
		Where("COALESCE(is_delete, 0) = 0").
		Update("is_delete", true).Error; err != nil {
		return fmt.Errorf("hide video locations for directory: %w", err)
	}
	return nil
}

// SetDirectoryDeletedAndHideVideos toggles deletion flag and hides/unhides its videos.
func SetDirectoryDeletedAndHideVideos(ctx context.Context, id int64, deleted bool) (*models.Directory, error) {
	return updateDirectoryWithVisibility(ctx, id, func(tx *gorm.DB, dir *models.Directory) error {
		dir.IsDelete = deleted
		return nil
	})
}

func updateDirectoryWithVisibility(ctx context.Context, id int64, mutate func(tx *gorm.DB, dir *models.Directory) error) (*models.Directory, error) {
	if id == 0 {
		return nil, errors.New("directory id cannot be zero")
	}

	var dir models.Directory
	err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&dir, id).Error; err != nil {
			return err
		}

		if mutate != nil {
			if err := mutate(tx, &dir); err != nil {
				return err
			}
		}

		if err := tx.Save(&dir).Error; err != nil {
			return fmt.Errorf("update directory: %w", err)
		}

		return nil
	})

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &dir, nil
}

// DirectoriesByIDs returns directories matching the provided ids.
func DirectoriesByIDs(ctx context.Context, ids []int64) ([]models.Directory, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var dirs []models.Directory
	if err := common.DB.WithContext(ctx).Where("id IN ?", ids).Order("id").Find(&dirs).Error; err != nil {
		return nil, fmt.Errorf("list directories by ids: %w", err)
	}
	return dirs, nil
}
