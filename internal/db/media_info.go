package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"javboss/internal/models"
)

// FillVideoMediaInfo fills missing/older probe data without replacing an already
// populated record or writing a stale copy of counters, subtitles or other fields.
func (store *Store) FillVideoMediaInfo(ctx context.Context, videoID int64, info models.MediaInfo) (bool, error) {
	if videoID <= 0 || !info.Complete() {
		return false, errors.New("video id and complete media info are required")
	}
	raw, err := json.Marshal(info)
	if err != nil {
		return false, fmt.Errorf("encode video media info: %w", err)
	}
	result := store.conn.WithContext(ctx).Model(&models.Video{}).
		Where("id = ? AND COALESCE(json_extract(media_info, '$.version'), 0) < ?", videoID, models.MediaInfoVersion).
		UpdateColumn("media_info", string(raw))
	if result.Error != nil {
		return false, fmt.Errorf("save video media info: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}
