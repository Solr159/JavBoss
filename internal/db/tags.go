package db

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"javboss/internal/common"
	"javboss/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TagCount represents a tag with associated video count.
type TagCount struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	CategoryID *int64 `json:"category_id,omitempty"`
	Category   string `json:"category,omitempty"`
	Count      int64  `json:"count"`
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
// By default it includes locations already associated with JAV metadata.
func ListTags(ctx context.Context, directoryIDs []int64, hideJav ...bool) ([]TagCount, error) {
	var tags []TagCount
	hideRecognizedJav := false
	if len(hideJav) > 0 {
		hideRecognizedJav = hideJav[0]
	}
	countWhere := activeLocationWhereSQL("vl", "d")
	if hideRecognizedJav {
		countWhere += " AND vl.jav_id IS NULL"
	}
	query := common.DB.WithContext(ctx).
		Table("tag t").
		Select("t.id, t.name, t.category_id, tc.name AS category, COUNT(DISTINCT CASE WHEN " + countWhere + " THEN vl.id END) AS count").
		Joins("LEFT JOIN video_tag vt ON vt.tag_id = t.id").
		Joins("LEFT JOIN video_location vl ON vl.video_id = vt.video_id").
		Joins("LEFT JOIN directory d ON d.id = vl.directory_id").
		Joins("LEFT JOIN tag_category tc ON tc.id = t.category_id")
	query = applyDirectoryFilter(query, "vl", directoryIDs)
	if err := query.
		Group("t.id, t.name, t.category_id, tc.name").
		Order("t.name").
		Scan(&tags).Error; err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	return tags, nil
}

// ListTagCategories returns every category in its configured display order.
func ListTagCategories(ctx context.Context) ([]models.TagCategory, error) {
	var categories []models.TagCategory
	if err := common.DB.WithContext(ctx).Order("sort_order, id").Find(&categories).Error; err != nil {
		return nil, fmt.Errorf("list tag categories: %w", err)
	}
	return categories, nil
}

// CreateTagCategory creates an empty category that tags can be moved into.
func CreateTagCategory(ctx context.Context, name string) (*models.TagCategory, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("category name cannot be empty")
	}
	category := models.TagCategory{Name: name}
	if err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var maxSortOrder int
		if err := tx.Model(&models.TagCategory{}).
			Select("COALESCE(MAX(sort_order), -1)").
			Scan(&maxSortOrder).Error; err != nil {
			return fmt.Errorf("find last tag category position: %w", err)
		}
		category.SortOrder = maxSortOrder + 1
		return tx.Create(&category).Error
	}); err != nil {
		return nil, fmt.Errorf("create tag category %q: %w", name, err)
	}
	return &category, nil
}

// ReorderTagCategories saves the complete category order. ID 0 reserves a
// sortable position for the virtual default category without storing a row.
func ReorderTagCategories(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return errors.New("category ids are required")
	}
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id < 0 {
			return errors.New("category ids cannot be negative")
		}
		if _, exists := seen[id]; exists {
			return errors.New("category ids must be unique")
		}
		seen[id] = struct{}{}
	}
	if _, hasDefaultCategory := seen[0]; !hasDefaultCategory {
		return errors.New("category order must include the default category")
	}
	return common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var storedIDs []int64
		if err := tx.Model(&models.TagCategory{}).Pluck("id", &storedIDs).Error; err != nil {
			return fmt.Errorf("list tag categories for reorder: %w", err)
		}
		if len(storedIDs)+1 != len(ids) {
			return errors.New("category order must include every category")
		}
		stored := make(map[int64]struct{}, len(storedIDs))
		for _, id := range storedIDs {
			stored[id] = struct{}{}
		}
		for sortOrder, id := range ids {
			if id == 0 {
				continue
			}
			if _, exists := stored[id]; !exists {
				return fmt.Errorf("tag category %d not found", id)
			}
			if err := tx.Model(&models.TagCategory{}).
				Where("id = ?", id).
				Update("sort_order", sortOrder).Error; err != nil {
				return fmt.Errorf("update tag category %d position: %w", id, err)
			}
		}
		return nil
	})
}

// RenameTagCategory changes a category name without changing tag membership.
func RenameTagCategory(ctx context.Context, id int64, name string) error {
	name = strings.TrimSpace(name)
	if id <= 0 {
		return errors.New("category id must be positive")
	}
	if name == "" {
		return errors.New("category name cannot be empty")
	}
	result := common.DB.WithContext(ctx).
		Model(&models.TagCategory{}).
		Where("id = ?", id).
		Update("name", name)
	if result.Error != nil {
		return fmt.Errorf("rename tag category: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteTagCategory removes a category and leaves its tags uncategorized.
func DeleteTagCategory(ctx context.Context, id int64) error {
	if id <= 0 {
		return errors.New("category id must be positive")
	}
	return common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var categories []models.TagCategory
		if err := tx.Order("sort_order, id").Find(&categories).Error; err != nil {
			return fmt.Errorf("list tag categories for delete: %w", err)
		}
		categoryOrder := tagCategoryOrderWithDefault(categories)
		found := false
		nextOrder := make([]int64, 0, len(categoryOrder)-1)
		for _, categoryID := range categoryOrder {
			if categoryID == id {
				found = true
				continue
			}
			nextOrder = append(nextOrder, categoryID)
		}
		if !found {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Model(&models.Tag{}).Where("category_id = ?", id).Update("category_id", nil).Error; err != nil {
			return fmt.Errorf("clear tag category: %w", err)
		}
		result := tx.Delete(&models.TagCategory{}, id)
		if result.Error != nil {
			return fmt.Errorf("delete tag category: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		for sortOrder, categoryID := range nextOrder {
			if categoryID == 0 {
				continue
			}
			if err := tx.Model(&models.TagCategory{}).
				Where("id = ?", categoryID).
				Update("sort_order", sortOrder).Error; err != nil {
				return fmt.Errorf("normalize tag category %d position after delete: %w", categoryID, err)
			}
		}
		return nil
	})
}

func tagCategoryOrderWithDefault(categories []models.TagCategory) []int64 {
	occupiedSortOrders := make(map[int]struct{}, len(categories))
	for _, category := range categories {
		if category.SortOrder >= 0 {
			occupiedSortOrders[category.SortOrder] = struct{}{}
		}
	}
	defaultSortOrder := 0
	for {
		if _, occupied := occupiedSortOrders[defaultSortOrder]; !occupied {
			break
		}
		defaultSortOrder++
	}
	type orderedCategory struct {
		id        int64
		sortOrder int
	}
	ordered := make([]orderedCategory, 0, len(categories)+1)
	for _, category := range categories {
		ordered = append(ordered, orderedCategory{id: category.ID, sortOrder: category.SortOrder})
	}
	ordered = append(ordered, orderedCategory{id: 0, sortOrder: defaultSortOrder})
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].sortOrder != ordered[j].sortOrder {
			return ordered[i].sortOrder < ordered[j].sortOrder
		}
		return ordered[i].id < ordered[j].id
	})
	ids := make([]int64, 0, len(ordered))
	for _, category := range ordered {
		ids = append(ids, category.id)
	}
	return ids
}

// AssignTagsCategory moves multiple tags into one category. A nil category
// leaves the selected tags uncategorized.
func AssignTagsCategory(ctx context.Context, tagIDs []int64, categoryID *int64) error {
	cleanTagIDs := uniqueInt64s(tagIDs)
	if len(cleanTagIDs) == 0 {
		return errors.New("tag ids are required")
	}
	if categoryID != nil {
		if *categoryID <= 0 {
			return errors.New("category id must be positive")
		}
		var count int64
		if err := common.DB.WithContext(ctx).Model(&models.TagCategory{}).Where("id = ?", *categoryID).Count(&count).Error; err != nil {
			return fmt.Errorf("find tag category: %w", err)
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}
	}
	if err := common.DB.WithContext(ctx).
		Model(&models.Tag{}).
		Where("id IN ?", cleanTagIDs).
		Update("category_id", categoryID).Error; err != nil {
		return fmt.Errorf("assign tag category: %w", err)
	}
	return nil
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
