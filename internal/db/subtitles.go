package db

import (
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"

	"javboss/internal/models"
)

func (store *Store) UpdateVideoSubtitleOffset(ctx context.Context, videoID, offsetMS int64) (bool, error) {
	return store.UpdateVideoSubtitleSettings(ctx, videoID, &offsetMS, nil)
}

func (store *Store) UpdateVideoSubtitleSettings(ctx context.Context, videoID int64, offsetMS *int64, track *string) (bool, error) {
	if offsetMS != nil && (*offsetMS < -3600000 || *offsetMS > 3600000) {
		return false, fmt.Errorf("invalid subtitle offset")
	}
	if track != nil && len(*track) > 1024 {
		return false, fmt.Errorf("invalid subtitle track")
	}
	values := map[string]any{}
	if track != nil {
		values["track"] = *track
	}
	if offsetMS != nil {
		values["offset_ms"] = *offsetMS
	}
	if len(values) == 0 {
		return false, fmt.Errorf("empty subtitle settings")
	}
	data, err := json.Marshal(values)
	if err != nil {
		return false, fmt.Errorf("encode subtitle settings: %w", err)
	}
	// Merge in SQL so independent offset and track saves cannot overwrite each other.
	result := store.conn.WithContext(ctx).Model(&models.Video{}).
		Where("id = ?", videoID).
		Where("EXISTS (?)", store.activeVideoLocationSubquery(ctx)).
		Update("subtitle_settings", gorm.Expr("json_patch(subtitle_settings, ?)", string(data)))
	if result.Error != nil {
		return false, fmt.Errorf("update video subtitle settings: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}
