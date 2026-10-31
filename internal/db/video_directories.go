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
	DirectoryID   int64    `json:"directory_id"`
	DirectoryPath string   `json:"directory_path"`
	Path          string   `json:"path"`
	Videos        []string `json:"videos"`
	VideoCount    int      `json:"video_count"`
}

const directoryVideoPreviewLimit = 20

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
		Where(activeDirectoryWhereSQL("directory")).
		Order("video_location.directory_id, video_location.relative_path").Scan(&locations).Error; err != nil {
		return nil, fmt.Errorf("list video directories: %w", err)
	}
	items := make([]VideoDirectory, 0)
	type directoryKey struct {
		id   int64
		path string
	}
	directories := make(map[directoryKey]*VideoDirectory)
	for _, location := range locations {
		for dir := path.Dir(location.RelativePath); ; dir = path.Dir(dir) {
			key := directoryKey{location.DirectoryID, dir}
			if directories[key] != nil {
				break
			}
			directories[key] = &VideoDirectory{
				DirectoryID:   location.DirectoryID,
				DirectoryPath: location.DirectoryPath,
				Path:          dir,
				Videos:        []string{},
			}
			if dir == "." || dir == "/" {
				break
			}
		}
		// Count only files directly in this directory, keeping the response bounded.
		directory := directories[directoryKey{location.DirectoryID, path.Dir(location.RelativePath)}]
		directory.VideoCount++
		if len(directory.Videos) < directoryVideoPreviewLimit {
			directory.Videos = append(directory.Videos, path.Base(location.RelativePath))
		}
	}
	for _, directory := range directories {
		items = append(items, *directory)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].DirectoryPath != items[j].DirectoryPath {
			return items[i].DirectoryPath < items[j].DirectoryPath
		}
		return items[i].Path < items[j].Path
	})
	return items, nil
}
