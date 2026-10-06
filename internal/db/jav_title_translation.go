package db

import (
	"context"
	"fmt"

	"javboss/internal/common"
	"javboss/internal/models"
)

// SaveJavTitleTranslation saves a separate Chinese title unless either title was edited during translation.
func SaveJavTitleTranslation(ctx context.Context, item *models.Jav, title string) (bool, error) {
	result := common.DB.WithContext(ctx).Model(&models.Jav{}).
		Where("id = ? AND title = ? AND zh_title = ?", item.ID, item.Title, item.ZhTitle).
		Update("zh_title", title)
	if result.Error != nil {
		return false, fmt.Errorf("save jav title translation: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

// ListJavTitleTranslationItems includes all records, regardless of the current page or filters.
func ListJavTitleTranslationItems(ctx context.Context) ([]models.Jav, error) {
	var items []models.Jav
	if err := common.DB.WithContext(ctx).Select("id", "title", "zh_title").Order("id").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list jav titles for translation: %w", err)
	}
	return items, nil
}
