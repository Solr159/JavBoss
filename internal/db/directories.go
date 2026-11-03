package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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

// normalizeDirectoryScanScope validates requested subdirectory selections against
// the directory root. Each entry must be a relative path inside root; absolute
// paths, parent traversal, and missing or non-directory targets are rejected so a
// broken scope can never widen a scan outside the chosen directory.
func normalizeDirectoryScanScope(root string, include []string) (models.DirectoryScanScope, error) {
	if len(include) == 0 {
		return models.DirectoryScanScope{}, nil
	}
	cleanedRoot := filepath.Clean(root)
	seen := make(map[string]struct{}, len(include))
	normalized := make([]string, 0, len(include))
	for _, raw := range include {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if strings.ContainsRune(value, 0) {
			return models.DirectoryScanScope{}, fmt.Errorf("invalid scan scope %q", raw)
		}
		relativeToRoot := filepath.FromSlash(value)
		if filepath.IsAbs(relativeToRoot) || filepath.VolumeName(relativeToRoot) != "" {
			return models.DirectoryScanScope{}, fmt.Errorf("scan scope %q must be relative to the directory", raw)
		}
		relative := filepath.Clean(relativeToRoot)
		if relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return models.DirectoryScanScope{}, fmt.Errorf("scan scope %q must stay inside the directory", raw)
		}
		target := filepath.Join(cleanedRoot, relative)
		info, err := os.Stat(target)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return models.DirectoryScanScope{}, fmt.Errorf("scan scope %q does not exist", raw)
			}
			return models.DirectoryScanScope{}, fmt.Errorf("stat scan scope %q: %w", raw, err)
		}
		if !info.IsDir() {
			return models.DirectoryScanScope{}, fmt.Errorf("scan scope %q is not a directory", raw)
		}
		key := strings.ToLower(filepath.ToSlash(relative))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, filepath.ToSlash(relative))
	}
	sort.Slice(normalized, func(i, j int) bool {
		return strings.ToLower(normalized[i]) < strings.ToLower(normalized[j])
	})
	return models.DirectoryScanScope{Include: dropNestedScanScopes(normalized)}, nil
}

// dropNestedScanScopes removes selections already covered by an ancestor selection,
// so overlapping picks do not scan the same files twice.
func dropNestedScanScopes(include []string) []string {
	if len(include) < 2 {
		return include
	}
	kept := make([]string, 0, len(include))
	for _, candidate := range include {
		nested := false
		for _, other := range include {
			if other == candidate {
				continue
			}
			if strings.HasPrefix(strings.ToLower(candidate), strings.ToLower(other)+"/") {
				nested = true
				break
			}
		}
		if !nested {
			kept = append(kept, candidate)
		}
	}
	return kept
}

// ListDirectories returns all directories regardless of status.
func (store *Store) ListDirectories(ctx context.Context) ([]models.Directory, error) {
	var dirs []models.Directory
	if err := store.conn.WithContext(ctx).
		Model(&models.Directory{}).
		Select(`directory.*,
			COUNT(video_location.id) AS scanned_video_count,
			COALESCE(SUM(CASE WHEN video_location.jav_id IS NOT NULL THEN 1 ELSE 0 END), 0) AS scraped_video_count`).
		Joins(`LEFT JOIN video_location
			ON video_location.directory_id = directory.id`).
		Group("directory.id").
		Order("directory.id").
		Find(&dirs).Error; err != nil {
		return nil, fmt.Errorf("list directories: %w", err)
	}
	return dirs, nil
}

// ListActiveDirectories returns directories that are not marked as deleted.
func (store *Store) ListActiveDirectories(ctx context.Context) ([]models.Directory, error) {
	var dirs []models.Directory
	if err := store.conn.WithContext(ctx).
		Where("COALESCE(is_delete, 0) = 0").
		Order("id").
		Find(&dirs).Error; err != nil {
		return nil, fmt.Errorf("list active directories: %w", err)
	}
	return dirs, nil
}

// GetDirectory fetches a single directory by id.
func (store *Store) GetDirectory(ctx context.Context, id int64) (*models.Directory, error) {
	if id == 0 {
		return nil, errors.New("directory id cannot be zero")
	}
	var dir models.Directory
	if err := store.conn.WithContext(ctx).First(&dir, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get directory: %w", err)
	}
	return &dir, nil
}

// CreateDirectory registers a new directory.
func (store *Store) CreateDirectory(ctx context.Context, path string) (*models.Directory, error) {
	return store.CreateDirectoryWithScope(ctx, path, nil)
}

// CreateDirectoryWithScope registers a new directory restricted to the given
// subdirectories. A nil or empty include list keeps the whole directory in scope.
func (store *Store) CreateDirectoryWithScope(ctx context.Context, path string, include []string) (*models.Directory, error) {
	normalized, err := normalizeDirectoryPath(path)
	if err != nil {
		return nil, err
	}
	scope, err := normalizeDirectoryScanScope(normalized, include)
	if err != nil {
		return nil, err
	}

	var existing models.Directory
	err = store.conn.WithContext(ctx).Where("path = ?", normalized).First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	// If directory exists but was soft-deleted, restore it instead of inserting a new row.
	if err == nil {
		if existing.IsDelete {
			dir, updErr := store.updateDirectoryWithVisibility(ctx, existing.ID, func(tx *gorm.DB, dir *models.Directory) error {
				dir.Path = normalized
				dir.IsDelete = false
				dir.Missing = false
				dir.Enabled = true
				dir.ScanScope = scope
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
		Path:      normalized,
		ScanScope: scope,
	}
	if err := store.conn.WithContext(ctx).Create(&dir).Error; err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}
	return &dir, nil
}

// UpdateDirectory updates a directory's attributes. Pass nil to leave a field untouched.
func (store *Store) UpdateDirectory(ctx context.Context, id int64, path *string, isDelete *bool, enabled *bool) (*models.Directory, error) {
	return store.UpdateDirectoryWithScope(ctx, id, path, isDelete, enabled, nil)
}

// UpdateDirectoryWithScope updates a directory like UpdateDirectory and replaces the
// scanned subdirectory selection when include is non-nil. A non-nil empty slice
// restores the default of scanning every subdirectory below the directory path.
func (store *Store) UpdateDirectoryWithScope(
	ctx context.Context,
	id int64,
	path *string,
	isDelete *bool,
	enabled *bool,
	include *[]string,
) (*models.Directory, error) {
	var normalizedPath *string
	if path != nil {
		normalized, err := normalizeDirectoryPath(*path)
		if err != nil {
			return nil, err
		}
		normalizedPath = &normalized
	}

	return store.updateDirectoryWithVisibility(ctx, id, func(tx *gorm.DB, dir *models.Directory) error {
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
				if err := deleteVideoLocationsByDirectoryID(tx, dir.ID); err != nil {
					return err
				}
				if include == nil {
					// The previous selection described the old root; keep it only when it
					// still resolves inside the new root, otherwise fall back to the whole directory.
					if scope, err := normalizeDirectoryScanScope(dir.Path, dir.ScanScope.Includes()); err == nil {
						dir.ScanScope = scope
					} else {
						dir.ScanScope = models.DirectoryScanScope{}
					}
				}
			}
			dir.Missing = false
		}
		if include != nil {
			scope, err := normalizeDirectoryScanScope(dir.Path, *include)
			if err != nil {
				return err
			}
			dir.ScanScope = scope
		}
		if isDelete != nil {
			dir.IsDelete = *isDelete
		}
		if enabled != nil {
			dir.Enabled = *enabled
		}
		return nil
	})
}

// UpdateDirectoryScanSettings updates only automatic-scan fields. The targeted update is safe to
// run while a scan is active because it cannot overwrite scan state or the latest scan summary.
func (store *Store) UpdateDirectoryScanSettings(
	ctx context.Context,
	id int64,
	autoScanEnabled *bool,
	autoScanIntervalMinutes *int,
) (*models.Directory, error) {
	if id <= 0 {
		return nil, errors.New("directory id cannot be zero")
	}
	updates := map[string]any{}
	if autoScanEnabled != nil {
		updates["auto_scan_enabled"] = *autoScanEnabled
	}
	if autoScanIntervalMinutes != nil {
		if *autoScanIntervalMinutes <= 0 {
			return nil, errors.New("automatic scan interval must be positive")
		}
		updates["auto_scan_interval_minutes"] = *autoScanIntervalMinutes
	}
	if len(updates) > 0 {
		result := store.conn.WithContext(ctx).
			Model(&models.Directory{}).
			Where("id = ?", id).
			Updates(updates)
		if result.Error != nil {
			return nil, fmt.Errorf("update directory scan settings: %w", result.Error)
		}
	}
	return store.GetDirectory(ctx, id)
}

// SetDirectoryMissing updates whether a directory is temporarily unavailable.
// Missing directories and their videos remain visible until explicitly deleted.
func (store *Store) SetDirectoryMissing(ctx context.Context, id int64, missing bool) error {
	if id <= 0 {
		return errors.New("directory id cannot be zero")
	}
	if err := store.conn.WithContext(ctx).
		Model(&models.Directory{}).
		Where("id = ?", id).
		Update("missing", missing).Error; err != nil {
		return fmt.Errorf("update directory missing status: %w", err)
	}
	return nil
}

// UpdateDirectoryLastScanSummary stores the latest successfully completed scan result.
func (store *Store) UpdateDirectoryLastScanSummary(
	ctx context.Context,
	id int64,
	summary models.DirectoryScanSummary,
) error {
	if id <= 0 {
		return errors.New("directory id must be positive")
	}
	result := store.conn.WithContext(ctx).
		Model(&models.Directory{}).
		Where("id = ?", id).
		UpdateColumn("last_scan_summary", summary)
	if result.Error != nil {
		return fmt.Errorf("update directory last scan summary: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.New("directory not found")
	}
	return nil
}

func deleteVideoLocationsByDirectoryID(tx *gorm.DB, directoryID int64) error {
	if directoryID <= 0 {
		return errors.New("directory id cannot be zero")
	}
	if err := tx.
		Model(&models.VideoLocation{}).
		Where("directory_id = ?", directoryID).
		Delete(&models.VideoLocation{}).Error; err != nil {
		return fmt.Errorf("delete video locations for directory: %w", err)
	}
	return nil
}

// SetDirectoryDeletedAndHideVideos toggles deletion flag and hides/unhides its videos.
func (store *Store) SetDirectoryDeletedAndHideVideos(ctx context.Context, id int64, deleted bool) (*models.Directory, error) {
	return store.updateDirectoryWithVisibility(ctx, id, func(tx *gorm.DB, dir *models.Directory) error {
		dir.IsDelete = deleted
		return nil
	})
}

func (store *Store) updateDirectoryWithVisibility(ctx context.Context, id int64, mutate func(tx *gorm.DB, dir *models.Directory) error) (*models.Directory, error) {
	if id == 0 {
		return nil, errors.New("directory id cannot be zero")
	}

	var dir models.Directory
	err := store.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
func (store *Store) DirectoriesByIDs(ctx context.Context, ids []int64) ([]models.Directory, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var dirs []models.Directory
	if err := store.conn.WithContext(ctx).Where("id IN ?", ids).Order("id").Find(&dirs).Error; err != nil {
		return nil, fmt.Errorf("list directories by ids: %w", err)
	}
	return dirs, nil
}
