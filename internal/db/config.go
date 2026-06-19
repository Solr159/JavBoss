package db

import (
	"context"
	"fmt"
	"strings"

	"javboss/internal/common"
	"javboss/internal/models"

	"gorm.io/gorm/clause"
)

// ListConfig returns all config key/value pairs.
func ListConfig(ctx context.Context) (map[string]string, error) {
	var rows []models.Config
	if err := common.DB.WithContext(ctx).Order("key").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list config: %w", err)
	}
	result := make(map[string]string, len(rows))
	for _, row := range rows {
		result[row.Key] = row.Value
	}
	return result, nil
}

// UpsertConfig persists provided key/value pairs, replacing existing values when keys collide.
func UpsertConfig(ctx context.Context, entries map[string]string) error {
	if len(entries) == 0 {
		return nil
	}
	rows := make([]models.Config, 0, len(entries))
	for k, v := range entries {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		rows = append(rows, models.Config{Key: key, Value: v})
	}
	if len(rows) == 0 {
		return nil
	}

	if err := common.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&rows).Error; err != nil {
		return fmt.Errorf("upsert config: %w", err)
	}
	return nil
}
