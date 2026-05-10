package db

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"pornboss/internal/common"
	"pornboss/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ListVideos returns paginated active video locations as video-like rows.
// By default it hides locations already associated with JAV metadata.
func ListVideos(ctx context.Context, limit, offset int, tagNames []string, search, sort string, seed *int64, directoryIDs []int64, hideJav ...bool) ([]models.Video, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	hideRecognizedJav := true
	if len(hideJav) > 0 {
		hideRecognizedJav = hideJav[0]
	}

	orderClause := "video.created_at DESC, video_location.id DESC" // default: newest first
	var orderExpr clause.Expr
	useExpr := false
	switch strings.ToLower(strings.TrimSpace(sort)) {
	case "filename", "filename_asc":
		orderClause = "video_location.filename COLLATE NOCASE, video_location.id"
	case "filename_desc":
		orderClause = "video_location.filename COLLATE NOCASE DESC, video_location.id DESC"
	case "duration", "duration_desc":
		orderClause = "video.duration_sec DESC, video.created_at DESC, video_location.id DESC"
	case "duration_asc":
		orderClause = "video.duration_sec ASC, video.created_at ASC, video_location.id ASC"
	case "play_count", "play_count_desc":
		orderClause = "COALESCE(video.play_count, 0) DESC, video.created_at DESC, video_location.id DESC"
	case "play_count_asc":
		orderClause = "COALESCE(video.play_count, 0) ASC, video.created_at ASC, video_location.id ASC"
	case "recent_asc":
		orderClause = "video.created_at ASC, video_location.id ASC"
	case "random":
		if seed != nil && *seed > 0 {
			orderExpr = clause.Expr{
				SQL:  "stable_random_rank(video_location.id, ?), video_location.id",
				Vars: []any{*seed},
			}
			useExpr = true
		} else {
			orderClause = "RANDOM()"
		}
	case "recent", "":
		// keep default
	default:
		// unknown value fallback to default
		orderClause = "video.created_at DESC, video_location.id DESC"
	}

	query := common.DB.WithContext(ctx).
		Model(&models.VideoLocation{}).
		Joins("JOIN directory ON directory.id = video_location.directory_id").
		Joins("JOIN video ON video.id = video_location.video_id").
		Where(activeLocationWhereSQL("video_location", "directory")).
		Preload("DirectoryRef").
		Preload("Video").
		Preload("Video.Tags").
		Limit(limit).
		Offset(offset)
	if hideRecognizedJav {
		query = query.Where("video_location.jav_id IS NULL")
	}
	query = applyDirectoryFilter(query, "video_location", directoryIDs)
	if useExpr {
		query = query.Order(clause.OrderBy{Expression: orderExpr})
	} else {
		query = query.Order(orderClause)
	}

	cleanedSearch := strings.TrimSpace(search)
	if cleanedSearch != "" {
		like := fmt.Sprintf("%%%s%%", cleanedSearch)
		query = query.Where("video_location.filename LIKE ? COLLATE NOCASE", like)
	}

	cleanedTags := normalizeTagNames(tagNames)
	if len(cleanedTags) > 0 {
		query = query.
			Joins("JOIN video_tag ON video_tag.video_id = video_location.video_id").
			Joins("JOIN tag ON tag.id = video_tag.tag_id").
			Where("tag.name IN ?", cleanedTags).
			Group("video_location.id").
			Having("COUNT(DISTINCT tag.name) = ?", len(cleanedTags))
	}

	var locations []models.VideoLocation
	if err := query.Find(&locations).Error; err != nil {
		return nil, fmt.Errorf("list videos: %w", err)
	}
	videos := make([]models.Video, 0, len(locations))
	for _, loc := range locations {
		if loc.Video.ID == 0 {
			continue
		}
		videos = append(videos, videoFromLocation(loc))
	}
	return videos, nil
}

// CountVideos returns the total number of active locations that match optional filters.
// By default it hides locations already associated with JAV metadata.
func CountVideos(ctx context.Context, tagNames []string, search string, directoryIDs []int64, hideJav ...bool) (int64, error) {
	cleanedTags := normalizeTagNames(tagNames)
	cleanedSearch := strings.TrimSpace(search)
	hideRecognizedJav := true
	if len(hideJav) > 0 {
		hideRecognizedJav = hideJav[0]
	}
	like := ""
	if cleanedSearch != "" {
		like = fmt.Sprintf("%%%s%%", cleanedSearch)
	}

	// Base query counts locations; when filtering by tags, group by location id and
	// ensure all requested video tags are matched (intersection semantics).
	if len(cleanedTags) == 0 {
		base := common.DB.WithContext(ctx).
			Model(&models.VideoLocation{}).
			Joins("JOIN directory ON directory.id = video_location.directory_id").
			Where(activeLocationWhereSQL("video_location", "directory"))
		if hideRecognizedJav {
			base = base.Where("video_location.jav_id IS NULL")
		}
		base = applyDirectoryFilter(base, "video_location", directoryIDs)
		if like != "" {
			base = base.Where("video_location.filename LIKE ? COLLATE NOCASE", like)
		}
		var count int64
		if err := base.Count(&count).Error; err != nil {
			return 0, fmt.Errorf("count videos: %w", err)
		}
		return count, nil
	}

	// Build subquery selecting matching location ids then count outer rows.
	sub := common.DB.WithContext(ctx).
		Model(&models.VideoLocation{}).
		Joins("JOIN directory ON directory.id = video_location.directory_id").
		Where(activeLocationWhereSQL("video_location", "directory")).
		Select("video_location.id").
		Joins("JOIN video_tag ON video_tag.video_id = video_location.video_id").
		Joins("JOIN tag ON tag.id = video_tag.tag_id").
		Where("tag.name IN ?", cleanedTags)
	if hideRecognizedJav {
		sub = sub.Where("video_location.jav_id IS NULL")
	}
	sub = applyDirectoryFilter(sub, "video_location", directoryIDs)

	if like != "" {
		sub = sub.Where("video_location.filename LIKE ? COLLATE NOCASE", like)
	}

	sub = sub.Group("video_location.id").
		Having("COUNT(DISTINCT tag.name) = ?", len(cleanedTags))

	var count int64
	if err := common.DB.Table("(?) as m", sub).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count videos (filtered): %w", err)
	}
	return count, nil
}

func videoFromLocation(loc models.VideoLocation) models.Video {
	video := loc.Video
	applyLocationFields(&video, loc)
	video.Locations = []models.VideoLocation{{
		ID:           loc.ID,
		VideoID:      loc.VideoID,
		DirectoryID:  loc.DirectoryID,
		RelativePath: loc.RelativePath,
		Filename:     loc.Filename,
		ModifiedAt:   loc.ModifiedAt,
		JavID:        loc.JavID,
		IsDelete:     loc.IsDelete,
		CreatedAt:    loc.CreatedAt,
		UpdatedAt:    loc.UpdatedAt,
		DirectoryRef: loc.DirectoryRef,
	}}
	return video
}

func applyPrimaryLocationFields(video *models.Video) {
	if video == nil || len(video.Locations) == 0 {
		return
	}
	applyLocationFields(video, video.Locations[0])
}

func applyLocationFields(video *models.Video, loc models.VideoLocation) {
	if video == nil {
		return
	}
	video.LocationID = loc.ID
	video.DirectoryID = loc.DirectoryID
	video.Path = loc.RelativePath
	video.Filename = loc.Filename
	if video.Filename == "" {
		video.Filename = filepath.Base(filepath.FromSlash(loc.RelativePath))
	}
	video.ModifiedAt = loc.ModifiedAt
	video.JavID = loc.JavID
	video.DirectoryRef = loc.DirectoryRef
}

// GetVideo fetches a single video by identifier.
func GetVideo(ctx context.Context, id int64) (*models.Video, error) {
	var video models.Video
	if err := common.DB.WithContext(ctx).
		Model(&models.Video{}).
		Where("EXISTS (?)", activeVideoLocationSubquery(ctx)).
		Preload("Tags").
		Scopes(preloadActiveLocations).
		First(&video, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get video %d: %w", id, err)
	}
	applyPrimaryLocationFields(&video)
	return &video, nil
}

// AllVideos returns every video row; used for sync bookkeeping.
func AllVideos(ctx context.Context) ([]models.Video, error) {
	var videos []models.Video
	if err := common.DB.WithContext(ctx).Find(&videos).Error; err != nil {
		return nil, fmt.Errorf("load videos: %w", err)
	}
	return videos, nil
}

// GetVideoByFingerprint returns a video by its globally unique content fingerprint.
func GetVideoByFingerprint(ctx context.Context, fingerprint string) (*models.Video, error) {
	fingerprint = strings.TrimSpace(fingerprint)
	if fingerprint == "" {
		return nil, nil
	}
	var video models.Video
	err := common.DB.WithContext(ctx).
		Where("fingerprint = ?", fingerprint).
		First(&video).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get video by fingerprint: %w", err)
	}
	return &video, nil
}

// SaveVideo inserts or updates a video based on its primary key.
func SaveVideo(ctx context.Context, video *models.Video) error {
	if video == nil {
		return errors.New("video is nil")
	}
	if err := common.DB.WithContext(ctx).Save(video).Error; err != nil {
		return fmt.Errorf("save video %d: %w", video.ID, err)
	}
	return nil
}

// CreateVideo inserts a new video record.
func CreateVideo(ctx context.Context, video *models.Video) error {
	if video == nil {
		return errors.New("video is nil")
	}
	if err := common.DB.WithContext(ctx).Create(video).Error; err != nil {
		return fmt.Errorf("create video %q: %w", video.Fingerprint, err)
	}
	return nil
}

// DeleteByIDs removes videos by their identifiers.
func DeleteByIDs(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	if err := common.DB.WithContext(ctx).Where("id IN ?", ids).Delete(&models.Video{}).Error; err != nil {
		return fmt.Errorf("delete videos: %w", err)
	}
	return nil
}

// IncrementVideoPlayCount increments the play count for a video if it has an active location.
func IncrementVideoPlayCount(ctx context.Context, id int64) error {
	if id <= 0 {
		return errors.New("video id cannot be zero")
	}
	if err := common.DB.WithContext(ctx).
		Model(&models.Video{}).
		Where("id = ?", id).
		Where("EXISTS (?)", activeVideoLocationSubquery(ctx)).
		UpdateColumn("play_count", gorm.Expr("COALESCE(play_count, 0) + 1")).Error; err != nil {
		return fmt.Errorf("increment play count: %w", err)
	}
	return nil
}

// IncrementVideoPlayCountByPath increments the play count for a video located at a directory path + relative path.
func IncrementVideoPlayCountByPath(ctx context.Context, dirPath, relPath string) error {
	videoID, err := GetVideoIDByPath(ctx, dirPath, relPath)
	if err != nil {
		return err
	}
	if videoID == 0 {
		return nil
	}

	return IncrementVideoPlayCount(ctx, videoID)
}
