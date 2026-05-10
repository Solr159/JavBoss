package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"pornboss/internal/common"
	"pornboss/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TagCount represents a tag with associated video count.
type TagCount struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// CreateTag inserts a new tag with the provided name.
func CreateTag(ctx context.Context, name string) (*models.Tag, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("tag name cannot be empty")
	}

	tag := models.Tag{Name: name}
	if err := common.DB.WithContext(ctx).Create(&tag).Error; err != nil {
		return nil, fmt.Errorf("create tag %q: %w", name, err)
	}
	return &tag, nil
}

// DeleteTag removes a tag and detaches it from any associated videos.
func DeleteTag(ctx context.Context, id int64) error {
	if id == 0 {
		return errors.New("tag id cannot be zero")
	}

	return common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tag_id = ?", id).Delete(&models.VideoTag{}).Error; err != nil {
			return fmt.Errorf("delete tag relations: %w", err)
		}
		if err := tx.Delete(&models.Tag{}, id).Error; err != nil {
			return fmt.Errorf("delete tag: %w", err)
		}
		return nil
	})
}

// RenameTag updates the tag name.
func RenameTag(ctx context.Context, id int64, newName string) error {
	newName = strings.TrimSpace(newName)
	if id == 0 {
		return errors.New("tag id cannot be zero")
	}
	if newName == "" {
		return errors.New("tag name cannot be empty")
	}

	if err := common.DB.WithContext(ctx).Model(&models.Tag{}).Where("id = ?", id).Update("name", newName).Error; err != nil {
		return fmt.Errorf("rename tag: %w", err)
	}
	return nil
}

// ListTags returns all tags ordered by name with attached active location counts.
// By default it hides locations already associated with JAV metadata.
func ListTags(ctx context.Context, directoryIDs []int64, hideJav ...bool) ([]TagCount, error) {
	var tags []TagCount
	hideRecognizedJav := true
	if len(hideJav) > 0 {
		hideRecognizedJav = hideJav[0]
	}
	countWhere := activeLocationWhereSQL("vl", "d")
	if hideRecognizedJav {
		countWhere += " AND vl.jav_id IS NULL"
	}
	query := common.DB.WithContext(ctx).
		Table("tag t").
		Select("t.id, t.name, COUNT(DISTINCT CASE WHEN " + countWhere + " THEN vl.id END) AS count").
		Joins("LEFT JOIN video_tag vt ON vt.tag_id = t.id").
		Joins("LEFT JOIN video_location vl ON vl.video_id = vt.video_id").
		Joins("LEFT JOIN directory d ON d.id = vl.directory_id")
	query = applyDirectoryFilter(query, "vl", directoryIDs)
	if err := query.
		Group("t.id, t.name").
		Order("t.name").
		Scan(&tags).Error; err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	return tags, nil
}

// AddTagToVideos associates a single tag with multiple videos.
func AddTagToVideos(ctx context.Context, tagID int64, videoIDs []int64) error {
	if tagID == 0 || len(videoIDs) == 0 {
		return nil
	}

	cleanIDs := uniqueInt64s(videoIDs)
	if len(cleanIDs) == 0 {
		return nil
	}

	return common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var tag models.Tag
		if err := tx.First(&tag, tagID).Error; err != nil {
			return fmt.Errorf("find tag: %w", err)
		}

		now := time.Now()
		rows := make([]models.VideoTag, 0, len(cleanIDs))
		for _, vid := range cleanIDs {
			rows = append(rows, models.VideoTag{VideoID: vid, TagID: tag.ID, CreatedAt: now})
		}
		if len(rows) == 0 {
			return nil
		}

		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error; err != nil {
			return fmt.Errorf("insert video tags: %w", err)
		}
		return nil
	})
}

// RemoveTagFromVideos detaches a single tag from multiple videos.
func RemoveTagFromVideos(ctx context.Context, tagID int64, videoIDs []int64) error {
	if tagID == 0 || len(videoIDs) == 0 {
		return nil
	}

	cleanIDs := uniqueInt64s(videoIDs)
	if len(cleanIDs) == 0 {
		return nil
	}

	return common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var tag models.Tag
		if err := tx.First(&tag, tagID).Error; err != nil {
			return fmt.Errorf("find tag: %w", err)
		}

		if err := tx.Where("video_id IN ? AND tag_id = ?", cleanIDs, tagID).Delete(&models.VideoTag{}).Error; err != nil {
			return fmt.Errorf("delete video tags: %w", err)
		}
		return nil
	})
}

// ReplaceTagsForVideos replaces the full tag list for the provided videos.
func ReplaceTagsForVideos(ctx context.Context, videoIDs, tagIDs []int64) error {
	cleanVideoIDs := uniqueInt64s(videoIDs)
	if len(cleanVideoIDs) == 0 {
		return nil
	}
	cleanTagIDs := uniqueInt64s(tagIDs)

	var tags []models.Tag
	if len(cleanTagIDs) > 0 {
		if err := common.DB.WithContext(ctx).Where("id IN ?", cleanTagIDs).Find(&tags).Error; err != nil {
			return fmt.Errorf("find tags: %w", err)
		}
		if len(tags) != len(cleanTagIDs) {
			return errors.New("invalid tag_id")
		}
	}

	return common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("video_id IN ?", cleanVideoIDs).Delete(&models.VideoTag{}).Error; err != nil {
			return fmt.Errorf("delete video tags: %w", err)
		}
		if len(cleanTagIDs) == 0 {
			return nil
		}

		now := time.Now()
		rows := make([]models.VideoTag, 0, len(cleanVideoIDs)*len(cleanTagIDs))
		for _, vid := range cleanVideoIDs {
			for _, tid := range cleanTagIDs {
				rows = append(rows, models.VideoTag{VideoID: vid, TagID: tid, CreatedAt: now})
			}
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error; err != nil {
			return fmt.Errorf("insert video tags: %w", err)
		}
		return nil
	})
}

// DeleteTags removes multiple tags and detaches them from videos.
func DeleteTags(ctx context.Context, ids []int64) error {
	cleanIDs := uniqueInt64s(ids)
	if len(cleanIDs) == 0 {
		return nil
	}

	return common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tag_id IN ?", cleanIDs).Delete(&models.VideoTag{}).Error; err != nil {
			return fmt.Errorf("delete tag relations: %w", err)
		}
		if err := tx.Where("id IN ?", cleanIDs).Delete(&models.Tag{}).Error; err != nil {
			return fmt.Errorf("delete tags: %w", err)
		}
		return nil
	})
}
