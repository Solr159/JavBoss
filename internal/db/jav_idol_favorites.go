package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"pornboss/internal/common"
	"pornboss/internal/jav"
	"pornboss/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// JavIdolFavoriteGroupSummary represents an idol favorite group with a visible idol count.
type JavIdolFavoriteGroupSummary struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Count     int64  `json:"count"`
	SortOrder int    `json:"sort_order"`
}

func buildIdolFavoriteCountQuery(ctx context.Context) *gorm.DB {
	return common.DB.WithContext(ctx).
		Table("jav_idol_favorite_map").
		Select("jav_idol_id, COUNT(DISTINCT jav_idol_favorite_group_id) AS favorite_count").
		Group("jav_idol_id")
}

// ListJavIdolFavoriteGroups returns user-created idol favorite groups.
func ListJavIdolFavoriteGroups(ctx context.Context, directoryIDs []int64) ([]JavIdolFavoriteGroupSummary, error) {
	var groups []JavIdolFavoriteGroupSummary
	isEnglish := jav.CurrentMetadataLanguageIsEnglish()
	soloIdols := buildVisibleSoloIdolCoverQuery(ctx, directoryIDs, isEnglish)
	if err := common.DB.WithContext(ctx).
		Table("jav_idol_favorite_group jifg").
		Select("jifg.id, jifg.name, jifg.sort_order, COUNT(DISTINCT CASE WHEN solo_idols.cover_code IS NOT NULL AND COALESCE(ji.is_english, 0) = ? THEN jifm.jav_idol_id END) AS count", isEnglish).
		Joins("LEFT JOIN jav_idol_favorite_map jifm ON jifm.jav_idol_favorite_group_id = jifg.id").
		Joins("LEFT JOIN jav_idol ji ON ji.id = jifm.jav_idol_id").
		Joins("LEFT JOIN (?) solo_idols ON solo_idols.jav_idol_id = jifm.jav_idol_id", soloIdols).
		Group("jifg.id, jifg.name, jifg.sort_order").
		Order("jifg.sort_order ASC, jifg.name ASC, jifg.id ASC").
		Scan(&groups).Error; err != nil {
		return nil, fmt.Errorf("list jav idol favorite groups: %w", err)
	}
	return groups, nil
}

// CreateJavIdolFavoriteGroup creates a favorite group or returns the existing group with the same name.
func CreateJavIdolFavoriteGroup(ctx context.Context, name string) (*models.JavIdolFavoriteGroup, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("favorite group name cannot be empty")
	}

	var group models.JavIdolFavoriteGroup
	err := common.DB.WithContext(ctx).Where("name = ?", name).First(&group).Error
	if err == nil {
		return &group, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("find jav idol favorite group %q: %w", name, err)
	}
	nextOrder, err := nextJavIdolFavoriteGroupSortOrder(ctx)
	if err != nil {
		return nil, err
	}
	group = models.JavIdolFavoriteGroup{Name: name, SortOrder: nextOrder}
	if err := common.DB.WithContext(ctx).Create(&group).Error; err != nil {
		return nil, fmt.Errorf("create jav idol favorite group %q: %w", name, err)
	}
	return &group, nil
}

func nextJavIdolFavoriteGroupSortOrder(ctx context.Context) (int, error) {
	var maxOrder int
	if err := common.DB.WithContext(ctx).
		Model(&models.JavIdolFavoriteGroup{}).
		Select("COALESCE(MAX(sort_order), 0)").
		Scan(&maxOrder).Error; err != nil {
		return 0, fmt.Errorf("get next jav idol favorite group order: %w", err)
	}
	return maxOrder + 1, nil
}

// RenameJavIdolFavoriteGroup renames a favorite group.
func RenameJavIdolFavoriteGroup(ctx context.Context, id int64, name string) error {
	name = strings.TrimSpace(name)
	if id <= 0 {
		return errors.New("favorite group id must be positive")
	}
	if name == "" {
		return errors.New("favorite group name cannot be empty")
	}
	if err := common.DB.WithContext(ctx).
		Model(&models.JavIdolFavoriteGroup{}).
		Where("id = ?", id).
		Update("name", name).Error; err != nil {
		return fmt.Errorf("rename jav idol favorite group: %w", err)
	}
	return nil
}

// DeleteJavIdolFavoriteGroup deletes a favorite group and its memberships.
func DeleteJavIdolFavoriteGroup(ctx context.Context, id int64) error {
	if id <= 0 {
		return errors.New("favorite group id must be positive")
	}
	res := common.DB.WithContext(ctx).Delete(&models.JavIdolFavoriteGroup{}, id)
	if res.Error != nil {
		return fmt.Errorf("delete jav idol favorite group: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ReorderJavIdolFavoriteGroups persists the display order of favorite groups.
func ReorderJavIdolFavoriteGroups(ctx context.Context, groupIDs []int64) error {
	cleanIDs := uniqueInt64s(groupIDs)
	if len(cleanIDs) == 0 {
		return errors.New("group_ids required")
	}
	return common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.JavIdolFavoriteGroup{}).Where("id IN ?", cleanIDs).Count(&count).Error; err != nil {
			return fmt.Errorf("find jav idol favorite groups: %w", err)
		}
		if count != int64(len(cleanIDs)) {
			return errors.New("invalid favorite_group_id")
		}
		for index, id := range cleanIDs {
			if err := tx.Model(&models.JavIdolFavoriteGroup{}).
				Where("id = ?", id).
				Update("sort_order", index+1).Error; err != nil {
				return fmt.Errorf("update jav idol favorite group order: %w", err)
			}
		}
		return nil
	})
}

// ListJavIdolFavoriteGroupIdols returns visible idols in a favorite group order.
func ListJavIdolFavoriteGroupIdols(ctx context.Context, groupID int64, directoryIDs []int64) ([]JavIdolSummary, error) {
	if groupID <= 0 {
		return nil, errors.New("favorite group id must be positive")
	}
	items, _, err := ListJavIdols(ctx, "", "favorite", 100000, 0, directoryIDs, groupID)
	return items, err
}

// ReorderJavIdolFavoriteGroupIdols persists idol order inside a favorite group.
func ReorderJavIdolFavoriteGroupIdols(ctx context.Context, groupID int64, idolIDs []int64) error {
	if groupID <= 0 {
		return errors.New("favorite group id must be positive")
	}
	cleanIDs := uniqueInt64s(idolIDs)
	if len(cleanIDs) == 0 {
		return errors.New("idol_ids required")
	}
	return common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.JavIdolFavoriteGroup{}).Where("id = ?", groupID).Count(&count).Error; err != nil {
			return fmt.Errorf("find jav idol favorite group: %w", err)
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Model(&models.JavIdolFavoriteMap{}).
			Where("jav_idol_favorite_group_id = ?", groupID).
			Where("jav_idol_id IN ?", cleanIDs).
			Count(&count).Error; err != nil {
			return fmt.Errorf("find jav idol favorite maps: %w", err)
		}
		if count != int64(len(cleanIDs)) {
			return errors.New("invalid idol_id")
		}
		for index, idolID := range cleanIDs {
			if err := tx.Model(&models.JavIdolFavoriteMap{}).
				Where("jav_idol_favorite_group_id = ? AND jav_idol_id = ?", groupID, idolID).
				Update("sort_order", index+1).Error; err != nil {
				return fmt.Errorf("update jav idol favorite map order: %w", err)
			}
		}
		return nil
	})
}

// RemoveJavIdolFavoriteGroupIdols removes idols from a favorite group.
func RemoveJavIdolFavoriteGroupIdols(ctx context.Context, groupID int64, idolIDs []int64) error {
	if groupID <= 0 {
		return errors.New("favorite group id must be positive")
	}
	cleanIDs := uniqueInt64s(idolIDs)
	if len(cleanIDs) == 0 {
		return errors.New("idol_ids required")
	}
	if err := common.DB.WithContext(ctx).
		Where("jav_idol_favorite_group_id = ? AND jav_idol_id IN ?", groupID, cleanIDs).
		Delete(&models.JavIdolFavoriteMap{}).Error; err != nil {
		return fmt.Errorf("remove jav idol favorite maps: %w", err)
	}
	return nil
}

func nextJavIdolFavoriteMapSortOrderTx(tx *gorm.DB, groupID int64) (int, error) {
	var maxOrder int
	if err := tx.Model(&models.JavIdolFavoriteMap{}).
		Where("jav_idol_favorite_group_id = ?", groupID).
		Select("COALESCE(MAX(sort_order), 0)").
		Scan(&maxOrder).Error; err != nil {
		return 0, fmt.Errorf("get next jav idol favorite map order: %w", err)
	}
	return maxOrder + 1, nil
}

// ListJavIdolFavoriteGroupIDs returns favorite group IDs containing the idol.
func ListJavIdolFavoriteGroupIDs(ctx context.Context, idolID int64) ([]int64, error) {
	if idolID <= 0 {
		return nil, errors.New("idol id must be positive")
	}
	var ids []int64
	if err := common.DB.WithContext(ctx).
		Table("jav_idol_favorite_map").
		Where("jav_idol_id = ?", idolID).
		Order("jav_idol_favorite_group_id").
		Pluck("jav_idol_favorite_group_id", &ids).Error; err != nil {
		return nil, fmt.Errorf("list jav idol favorite group ids: %w", err)
	}
	return ids, nil
}

// ReplaceJavIdolFavoriteGroups replaces all favorite group memberships for one idol.
func ReplaceJavIdolFavoriteGroups(ctx context.Context, idolID int64, groupIDs []int64) error {
	if idolID <= 0 {
		return errors.New("idol id must be positive")
	}
	cleanGroupIDs := uniqueInt64s(groupIDs)

	return common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var idolCount int64
		if err := tx.Model(&models.JavIdol{}).Where("id = ?", idolID).Count(&idolCount).Error; err != nil {
			return fmt.Errorf("find jav idol: %w", err)
		}
		if idolCount == 0 {
			return gorm.ErrRecordNotFound
		}

		if len(cleanGroupIDs) > 0 {
			var groupCount int64
			if err := tx.Model(&models.JavIdolFavoriteGroup{}).Where("id IN ?", cleanGroupIDs).Count(&groupCount).Error; err != nil {
				return fmt.Errorf("find jav idol favorite groups: %w", err)
			}
			if groupCount != int64(len(cleanGroupIDs)) {
				return errors.New("invalid favorite_group_id")
			}
		}

		var existing []models.JavIdolFavoriteMap
		if err := tx.Where("jav_idol_id = ?", idolID).Find(&existing).Error; err != nil {
			return fmt.Errorf("find jav idol favorite maps: %w", err)
		}
		existingOrders := make(map[int64]int, len(existing))
		for _, row := range existing {
			if row.SortOrder > 0 {
				existingOrders[row.JavIdolFavoriteGroupID] = row.SortOrder
			}
		}

		if err := tx.Where("jav_idol_id = ?", idolID).Delete(&models.JavIdolFavoriteMap{}).Error; err != nil {
			return fmt.Errorf("delete jav idol favorite maps: %w", err)
		}
		if len(cleanGroupIDs) == 0 {
			return nil
		}

		now := time.Now()
		rows := make([]models.JavIdolFavoriteMap, 0, len(cleanGroupIDs))
		for _, groupID := range cleanGroupIDs {
			sortOrder := existingOrders[groupID]
			if sortOrder <= 0 {
				nextOrder, err := nextJavIdolFavoriteMapSortOrderTx(tx, groupID)
				if err != nil {
					return err
				}
				sortOrder = nextOrder
			}
			rows = append(rows, models.JavIdolFavoriteMap{
				JavIdolFavoriteGroupID: groupID,
				JavIdolID:              idolID,
				SortOrder:              sortOrder,
				CreatedAt:              now,
			})
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error; err != nil {
			return fmt.Errorf("insert jav idol favorite maps: %w", err)
		}
		return nil
	})
}
