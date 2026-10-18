package db

import (
	"context"
	"fmt"

	"javboss/internal/models"
)

// SaveJavTitleTranslation saves a separate Chinese title unless either title was edited during translation.
func (store *Store) SaveJavTitleTranslation(ctx context.Context, item *models.Jav, title string) (bool, error) {
	result := store.conn.WithContext(ctx).Model(&models.Jav{}).
		Where("id = ? AND title = ? AND zh_title = ?", item.ID, item.Title, item.ZhTitle).
		Update("zh_title", title)
	if result.Error != nil {
		return false, fmt.Errorf("save jav title translation: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

// ListJavTitleTranslationItems includes all records, regardless of the current page or filters.
func (store *Store) ListJavTitleTranslationItems(ctx context.Context) ([]models.Jav, error) {
	var items []models.Jav
	if err := store.conn.WithContext(ctx).Select("id", "title", "zh_title").Order("id").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list jav titles for translation: %w", err)
	}
	return items, nil
}

// GetJavTitleTranslationItem rechecks a queued title without loading its related metadata.
func (store *Store) GetJavTitleTranslationItem(ctx context.Context, id int64) (*models.Jav, error) {
	var item models.Jav
	if err := store.conn.WithContext(ctx).Select("id", "title", "zh_title").First(&item, id).Error; err != nil {
		return nil, fmt.Errorf("get jav title for translation: %w", err)
	}
	return &item, nil
}
