package db

import (
	"context"
	"fmt"

	"javboss/internal/common"
	"javboss/internal/models"
)

// GetJavSeriesRecord loads one series row for name translation.
func GetJavSeriesRecord(ctx context.Context, seriesID int64) (*models.JavSeries, error) {
	var item models.JavSeries
	if err := common.DB.WithContext(ctx).Where("id = ?", seriesID).First(&item).Error; err != nil {
		return nil, fmt.Errorf("get jav series: %w", err)
	}
	return &item, nil
}

// SaveJavSeriesNameTranslation saves a separate Chinese series name unless the name was edited during translation.
func SaveJavSeriesNameTranslation(ctx context.Context, item *models.JavSeries, name string) (bool, error) {
	result := common.DB.WithContext(ctx).Model(&models.JavSeries{}).
		Where("id = ? AND name = ? AND zh_name = ?", item.ID, item.Name, item.ZhName).
		Update("zh_name", name)
	if result.Error != nil {
		return false, fmt.Errorf("save jav series name translation: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}
