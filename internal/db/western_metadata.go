package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"javboss/internal/common"
	"javboss/internal/models"
	"javboss/internal/western"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func GetWesternMetadata(ctx context.Context, videoID int64) (*models.WesternMetadata, error) {
	if videoID <= 0 {
		return nil, errors.New("video id cannot be zero")
	}
	var metadata models.WesternMetadata
	if err := common.DB.WithContext(ctx).First(&metadata, "video_id = ?", videoID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get western metadata: %w", err)
	}
	return &metadata, nil
}

func SaveWesternMetadata(ctx context.Context, videoID int64, input western.Metadata) (*models.WesternMetadata, error) {
	if videoID <= 0 {
		return nil, errors.New("video id cannot be zero")
	}
	input.Source = strings.ToLower(strings.TrimSpace(input.Source))
	if input.Source == "" {
		return nil, errors.New("metadata source is required")
	}
	contentType := strings.ToLower(strings.TrimSpace(input.ContentType))
	if contentType != "movie" {
		contentType = "scene"
	}
	matchStatus := strings.ToLower(strings.TrimSpace(input.MatchStatus))
	if matchStatus != "unmatched" {
		matchStatus = "matched"
	}
	now := time.Now()
	metadata := models.WesternMetadata{
		VideoID:       videoID,
		Title:         strings.TrimSpace(input.Title),
		OriginalTitle: strings.TrimSpace(input.OriginalTitle),
		ContentType:   contentType,
		MatchStatus:   matchStatus,
		Studio:        strings.TrimSpace(input.Studio),
		Description:   strings.TrimSpace(input.Description),
		ReleaseDate:   strings.TrimSpace(input.ReleaseDate),
		Source:        input.Source,
		SourceID:      strings.TrimSpace(input.SourceID),
		SourceURL:     strings.TrimSpace(input.SourceURL),
		CoverURL:      strings.TrimSpace(input.CoverURL),
		Performers:    models.StringList(normalizeWesternValues(input.Performers)),
		Genres:        models.StringList(normalizeWesternValues(input.Genres)),
		Labels:        models.StringList(normalizeWesternValues(input.Labels)),
		FetchedAt:     now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if metadata.Title == "" && metadata.SourceID == "" {
		return nil, errors.New("metadata title or source id is required")
	}
	if err := common.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "video_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"title", "original_title", "content_type", "match_status", "studio", "description",
			"release_date", "source", "source_id", "source_url", "cover_url",
			"performers", "genres", "labels", "fetched_at", "updated_at",
		}),
	}).Create(&metadata).Error; err != nil {
		return nil, fmt.Errorf("save western metadata: %w", err)
	}
	return GetWesternMetadata(ctx, videoID)
}

func DeleteWesternMetadata(ctx context.Context, videoID int64) error {
	if videoID <= 0 {
		return errors.New("video id cannot be zero")
	}
	if err := common.DB.WithContext(ctx).Delete(&models.WesternMetadata{}, "video_id = ?", videoID).Error; err != nil {
		return fmt.Errorf("delete western metadata: %w", err)
	}
	return nil
}

func normalizeWesternValues(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if value == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}
