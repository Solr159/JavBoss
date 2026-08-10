package db

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"javboss/internal/common"
	"javboss/internal/models"
)

type WesternEntitySummary struct {
	Name          string `json:"name"`
	WorkCount     int64  `json:"work_count"`
	SampleVideoID int64  `json:"sample_video_id"`
	CoverURL      string `json:"cover_url"`
}

// ListWesternEntities aggregates performer, studio, and tag values from the
// active Western metadata records. Series intentionally returns no rows until
// the provider exposes a real series field.
func ListWesternEntities(ctx context.Context, kind, search string, limit, offset int) ([]WesternEntitySummary, int64, error) {
	if limit <= 0 {
		limit = 25
	}
	if offset < 0 {
		offset = 0
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "series" {
		return []WesternEntitySummary{}, 0, nil
	}
	if kind != "performers" && kind != "studios" && kind != "tags" {
		return nil, 0, fmt.Errorf("unsupported Western entity kind %q", kind)
	}
	var rows []models.WesternMetadata
	query := common.DB.WithContext(ctx).Model(&models.WesternMetadata{}).
		Where("EXISTS (SELECT 1 FROM video_location vl JOIN directory d ON d.id = vl.directory_id WHERE vl.video_id = western_metadata.video_id AND COALESCE(vl.is_delete, 0) = 0 AND COALESCE(d.is_delete, 0) = 0)")
	if err := query.Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("list Western metadata: %w", err)
	}
	needle := strings.ToLower(strings.TrimSpace(search))
	values := map[string]WesternEntitySummary{}
	add := func(value string, row models.WesternMetadata) {
		value = strings.TrimSpace(value)
		if value == "" || (needle != "" && !strings.Contains(strings.ToLower(value), needle)) {
			return
		}
		item := values[value]
		item.Name = value
		item.WorkCount++
		if item.SampleVideoID == 0 {
			item.SampleVideoID = row.VideoID
			item.CoverURL = row.CoverURL
		}
		values[value] = item
	}
	for _, row := range rows {
		switch kind {
		case "performers":
			for _, value := range row.Performers {
				add(value, row)
			}
		case "studios":
			add(row.Studio, row)
		case "tags":
			for _, value := range append([]string(row.Genres), row.Labels...) {
				add(value, row)
			}
		}
	}
	items := make([]WesternEntitySummary, 0, len(values))
	for _, item := range values {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].WorkCount != items[j].WorkCount {
			return items[i].WorkCount > items[j].WorkCount
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	total := int64(len(items))
	if offset >= len(items) {
		return []WesternEntitySummary{}, total, nil
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end], total, nil
}
