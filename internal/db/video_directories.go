package db

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"javboss/internal/models"

	"gorm.io/gorm"
)

// VideoFilter narrows the active locations returned by ListVideos and CountVideos.
type VideoFilter struct {
	HideJav      bool
	Subdirectory string
}

func applyVideoSubdirectoryFilter(query *gorm.DB, filters []VideoFilter) *gorm.DB {
	if len(filters) == 0 || filters[0].Subdirectory == "" || filters[0].Subdirectory == "." {
		return query
	}
	prefix := strings.TrimRight(filters[0].Subdirectory, "/") + "/"
	// Compare literal, case-sensitive prefixes including the separator. SQL LIKE
	// would treat % and _ in directory names as wildcards and fold ASCII case.
	return query.Where("substr(video_location.relative_path, 1, length(?)) = ? COLLATE BINARY", prefix, prefix)
}

type VideoDirectory struct {
	DirectoryID   int64  `json:"directory_id"`
	DirectoryPath string `json:"directory_path"`
	Path          string `json:"path"`
}

// ListVideoDirectories includes ancestors of indexed video locations, scoped to
// enabled, non-deleted roots. It does not walk or expose unindexed filesystem paths.
func (store *Store) ListVideoDirectories(ctx context.Context) ([]VideoDirectory, error) {
	var locations []struct {
		DirectoryID   int64
		DirectoryPath string
		RelativePath  string
	}
	if err := store.conn.WithContext(ctx).Model(&models.VideoLocation{}).
		Select("video_location.directory_id, directory.path AS directory_path, video_location.relative_path").
		Joins("JOIN directory ON directory.id = video_location.directory_id").
		Where(activeDirectoryWhereSQL("directory")).Scan(&locations).Error; err != nil {
		return nil, fmt.Errorf("list video directories: %w", err)
	}
	items := make([]VideoDirectory, 0)
	seen := make(map[VideoDirectory]bool)
	for _, location := range locations {
		for dir := path.Dir(location.RelativePath); ; dir = path.Dir(dir) {
			item := VideoDirectory{location.DirectoryID, location.DirectoryPath, dir}
			if seen[item] {
				break
			}
			seen[item] = true
			items = append(items, item)
			if dir == "." || dir == "/" {
				break
			}
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].DirectoryPath != items[j].DirectoryPath {
			return items[i].DirectoryPath < items[j].DirectoryPath
		}
		return items[i].Path < items[j].Path
	})
	return items, nil
}
