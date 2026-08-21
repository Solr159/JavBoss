package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"javboss/internal/common"
	"javboss/internal/jav"
	"javboss/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrJavDiscoverySubscriptionExists = errors.New("jav discovery subscription already exists")

// JavDiscoveryItemResult is the API-facing representation of an independently
// stored discovery item.
type JavDiscoveryItemResult struct {
	ID            int64           `json:"id"`
	Code          string          `json:"code"`
	ReleaseUnix   int64           `json:"release_unix"`
	Metadata      json.RawMessage `json:"metadata"`
	Wanted        bool            `json:"wanted"`
	Owned         bool            `json:"owned"`
	Subscriptions []string        `json:"subscriptions"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type ListJavDiscoveryItemsOptions struct {
	WantedOnly     bool
	IncludeOwned   bool
	SubscriptionID int64
	Limit          int
	Offset         int
}

func ListJavDiscoverySubscriptions(ctx context.Context) ([]models.JavDiscoverySubscription, error) {
	if common.DB == nil {
		return nil, errors.New("list jav discovery subscriptions: nil db")
	}
	var subscriptions []models.JavDiscoverySubscription
	if err := common.DB.WithContext(ctx).
		Order("created_at DESC, id DESC").
		Find(&subscriptions).Error; err != nil {
		return nil, fmt.Errorf("list jav discovery subscriptions: %w", err)
	}
	return subscriptions, nil
}

func GetJavDiscoverySubscription(ctx context.Context, id int64) (*models.JavDiscoverySubscription, error) {
	if common.DB == nil {
		return nil, errors.New("get jav discovery subscription: nil db")
	}
	if id <= 0 {
		return nil, errors.New("get jav discovery subscription: invalid id")
	}
	var subscription models.JavDiscoverySubscription
	if err := common.DB.WithContext(ctx).First(&subscription, id).Error; err != nil {
		return nil, fmt.Errorf("get jav discovery subscription: %w", err)
	}
	return &subscription, nil
}

func CountJavDiscoverySubscriptionItemsReleasedBefore(ctx context.Context, subscriptionID int64, releaseUnix int64) (int64, error) {
	if common.DB == nil {
		return 0, errors.New("count historical jav discovery subscription items: nil db")
	}
	if subscriptionID <= 0 {
		return 0, errors.New("count historical jav discovery subscription items: invalid subscription id")
	}
	if releaseUnix <= 0 {
		return 0, errors.New("count historical jav discovery subscription items: invalid release date")
	}
	var count int64
	if err := common.DB.WithContext(ctx).
		Model(&models.JavDiscoverySubscriptionItem{}).
		Joins("JOIN jav_discovery_item AS discovery_item ON discovery_item.id = jav_discovery_subscription_item.jav_discovery_item_id").
		Where("jav_discovery_subscription_id = ?", subscriptionID).
		Where("discovery_item.release_unix > 0 AND discovery_item.release_unix < ?", releaseUnix).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count historical jav discovery subscription items: %w", err)
	}
	return count, nil
}

func CreateJavDiscoverySubscription(ctx context.Context, subscription *models.JavDiscoverySubscription) error {
	if common.DB == nil {
		return errors.New("create jav discovery subscription: nil db")
	}
	if subscription == nil {
		return errors.New("create jav discovery subscription: missing subscription")
	}
	subscription.Kind = strings.TrimSpace(subscription.Kind)
	subscription.Name = strings.TrimSpace(subscription.Name)
	subscription.ReferenceCode = strings.ToUpper(strings.TrimSpace(subscription.ReferenceCode))
	subscription.ProviderLocator = strings.TrimSpace(subscription.ProviderLocator)
	if subscription.Kind == "" || subscription.Name == "" || subscription.ReferenceCode == "" || subscription.ProviderLocator == "" {
		return errors.New("create jav discovery subscription: missing required field")
	}

	var count int64
	if err := common.DB.WithContext(ctx).
		Model(&models.JavDiscoverySubscription{}).
		Where("kind = ? AND provider_locator = ?", subscription.Kind, subscription.ProviderLocator).
		Count(&count).Error; err != nil {
		return fmt.Errorf("check jav discovery subscription: %w", err)
	}
	if count > 0 {
		return ErrJavDiscoverySubscriptionExists
	}
	if err := common.DB.WithContext(ctx).Create(subscription).Error; err != nil {
		return fmt.Errorf("create jav discovery subscription: %w", err)
	}
	return nil
}

func DeleteJavDiscoverySubscription(ctx context.Context, id int64) error {
	if common.DB == nil {
		return errors.New("delete jav discovery subscription: nil db")
	}
	if id <= 0 {
		return errors.New("delete jav discovery subscription: invalid id")
	}
	result := common.DB.WithContext(ctx).Delete(&models.JavDiscoverySubscription{}, id)
	if result.Error != nil {
		return fmt.Errorf("delete jav discovery subscription: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func ListJavDiscoveryItems(ctx context.Context, options ListJavDiscoveryItemsOptions) ([]JavDiscoveryItemResult, int64, error) {
	if common.DB == nil {
		return nil, 0, errors.New("list jav discovery items: nil db")
	}
	if options.Limit <= 0 || options.Limit > 200 {
		options.Limit = 50
	}
	if options.Offset < 0 {
		options.Offset = 0
	}

	base := applyJavDiscoveryItemFilters(
		common.DB.WithContext(ctx).Model(&models.JavDiscoveryItem{}),
		options,
	)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count jav discovery items: %w", err)
	}

	var records []models.JavDiscoveryItem
	query := applyJavDiscoveryItemFilters(common.DB.WithContext(ctx), options)
	if err := query.
		Order("release_unix DESC, code DESC").
		Limit(options.Limit).
		Offset(options.Offset).
		Find(&records).Error; err != nil {
		return nil, 0, fmt.Errorf("list jav discovery items: %w", err)
	}

	itemIDs := make([]int64, 0, len(records))
	for _, record := range records {
		itemIDs = append(itemIDs, record.ID)
	}
	subscriptionsByItem, err := listDiscoveryItemSubscriptions(ctx, itemIDs)
	if err != nil {
		return nil, 0, err
	}
	ownedCodes, err := listOwnedDiscoveryCodes(ctx, records)
	if err != nil {
		return nil, 0, err
	}

	items := make([]JavDiscoveryItemResult, 0, len(records))
	for _, record := range records {
		metadata := json.RawMessage(strings.TrimSpace(record.MetadataJSON))
		if !json.Valid(metadata) {
			metadata = json.RawMessage(`{}`)
		}
		items = append(items, JavDiscoveryItemResult{
			ID:            record.ID,
			Code:          record.Code,
			ReleaseUnix:   record.ReleaseUnix,
			Metadata:      metadata,
			Wanted:        record.Wanted,
			Owned:         ownedCodes[strings.ToUpper(strings.TrimSpace(record.Code))],
			Subscriptions: subscriptionsByItem[record.ID],
			CreatedAt:     record.CreatedAt,
			UpdatedAt:     record.UpdatedAt,
		})
	}
	return items, total, nil
}

func applyJavDiscoveryItemFilters(query *gorm.DB, options ListJavDiscoveryItemsOptions) *gorm.DB {
	if options.WantedOnly {
		query = query.Where("wanted = ?", true)
	}
	if options.SubscriptionID > 0 {
		query = query.Where(
			"id IN (SELECT jav_discovery_item_id FROM jav_discovery_subscription_item WHERE jav_discovery_subscription_id = ?)",
			options.SubscriptionID,
		)
	}
	if !options.IncludeOwned {
		query = query.Where(`NOT EXISTS (
			SELECT 1
			FROM jav AS owned_jav
			JOIN video_location AS owned_location ON owned_location.jav_id = owned_jav.id
			JOIN directory AS owned_directory ON owned_directory.id = owned_location.directory_id
			WHERE UPPER(owned_jav.code) = UPPER(jav_discovery_item.code)
				AND COALESCE(owned_location.is_delete, 0) = 0
				AND COALESCE(owned_directory.is_delete, 0) = 0
		)`)
	}
	return query
}

func listOwnedDiscoveryCodes(ctx context.Context, items []models.JavDiscoveryItem) (map[string]bool, error) {
	result := make(map[string]bool)
	if len(items) == 0 {
		return result, nil
	}
	codes := make([]string, 0, len(items))
	for _, item := range items {
		code := strings.ToUpper(strings.TrimSpace(item.Code))
		if code != "" {
			codes = append(codes, code)
		}
	}
	if len(codes) == 0 {
		return result, nil
	}
	var rows []struct {
		Code string `gorm:"column:code"`
	}
	if err := common.DB.WithContext(ctx).
		Table("jav").
		Select("DISTINCT UPPER(jav.code) AS code").
		Joins("JOIN video_location AS vl ON vl.jav_id = jav.id").
		Joins("JOIN directory AS d ON d.id = vl.directory_id").
		Where("UPPER(jav.code) IN ?", codes).
		Where(activeLocationWhereSQL("vl", "d")).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list owned jav discovery codes: %w", err)
	}
	for _, row := range rows {
		result[strings.ToUpper(strings.TrimSpace(row.Code))] = true
	}
	return result, nil
}

func listDiscoveryItemSubscriptions(ctx context.Context, itemIDs []int64) (map[int64][]string, error) {
	result := make(map[int64][]string)
	if len(itemIDs) == 0 {
		return result, nil
	}
	var rows []struct {
		ItemID int64  `gorm:"column:item_id"`
		Name   string `gorm:"column:name"`
	}
	if err := common.DB.WithContext(ctx).
		Table("jav_discovery_subscription_item AS map").
		Select("map.jav_discovery_item_id AS item_id, subscription.name AS name").
		Joins("JOIN jav_discovery_subscription AS subscription ON subscription.id = map.jav_discovery_subscription_id").
		Where("map.jav_discovery_item_id IN ?", itemIDs).
		Order("subscription.name").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list jav discovery item subscriptions: %w", err)
	}
	for _, row := range rows {
		result[row.ItemID] = append(result[row.ItemID], row.Name)
	}
	return result, nil
}

func SetJavDiscoveryItemWanted(ctx context.Context, id int64, wanted bool) error {
	if common.DB == nil {
		return errors.New("update jav discovery item: nil db")
	}
	result := common.DB.WithContext(ctx).
		Model(&models.JavDiscoveryItem{}).
		Where("id = ?", id).
		Update("wanted", wanted)
	if result.Error != nil {
		return fmt.Errorf("update jav discovery item: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func GetJavDiscoveryItemCoverURL(ctx context.Context, id int64) (string, error) {
	return getJavDiscoveryItemImageURL(ctx, id, false)
}

func GetJavDiscoveryItemThumbnailURL(ctx context.Context, id int64) (string, error) {
	return getJavDiscoveryItemImageURL(ctx, id, true)
}

func getJavDiscoveryItemImageURL(ctx context.Context, id int64, thumbnail bool) (string, error) {
	if common.DB == nil {
		return "", errors.New("get jav discovery item cover: nil db")
	}
	if id <= 0 {
		return "", errors.New("get jav discovery item cover: invalid id")
	}
	var item models.JavDiscoveryItem
	if err := common.DB.WithContext(ctx).
		Select("id", "metadata_json").
		First(&item, id).Error; err != nil {
		return "", fmt.Errorf("get jav discovery item cover: %w", err)
	}
	var metadata struct {
		CoverURL     string `json:"cover_url"`
		ThumbnailURL string `json:"thumbnail_url"`
	}
	if err := json.Unmarshal([]byte(item.MetadataJSON), &metadata); err != nil {
		return "", fmt.Errorf("decode jav discovery item cover metadata: %w", err)
	}
	coverURL := strings.TrimSpace(metadata.CoverURL)
	if thumbnail && strings.TrimSpace(metadata.ThumbnailURL) != "" {
		coverURL = strings.TrimSpace(metadata.ThumbnailURL)
	}
	if coverURL == "" {
		return "", gorm.ErrRecordNotFound
	}
	return coverURL, nil
}

func GetJavDiscoveryItem(ctx context.Context, id int64) (*models.JavDiscoveryItem, error) {
	if common.DB == nil {
		return nil, errors.New("get jav discovery item: nil db")
	}
	if id <= 0 {
		return nil, errors.New("get jav discovery item: invalid id")
	}
	var item models.JavDiscoveryItem
	if err := common.DB.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, fmt.Errorf("get jav discovery item: %w", err)
	}
	return &item, nil
}

func UpdateJavDiscoveryItemDetails(ctx context.Context, id int64, details jav.JavBusDiscoveryItem) (*models.JavDiscoveryItem, error) {
	current, err := GetJavDiscoveryItem(ctx, id)
	if err != nil {
		return nil, err
	}
	var existing jav.JavBusDiscoveryItem
	_ = json.Unmarshal([]byte(current.MetadataJSON), &existing)
	details = mergeJavDiscoveryDetails(existing, details)
	metadata, err := json.Marshal(details)
	if err != nil {
		return nil, fmt.Errorf("marshal jav discovery item details: %w", err)
	}
	magnetLinks := details.MagnetLinks
	if magnetLinks == nil {
		magnetLinks = []jav.JavBusMagnetLink{}
	}
	magnetLinksJSON, err := json.Marshal(magnetLinks)
	if err != nil {
		return nil, fmt.Errorf("marshal jav discovery item magnet links: %w", err)
	}
	updates := map[string]any{
		"metadata_json":     string(metadata),
		"magnet_links_json": string(magnetLinksJSON),
	}
	if details.ReleaseUnix > 0 {
		updates["release_unix"] = details.ReleaseUnix
	}
	if err := common.DB.WithContext(ctx).
		Model(&models.JavDiscoveryItem{}).
		Where("id = ?", id).
		Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update jav discovery item details: %w", err)
	}
	return GetJavDiscoveryItem(ctx, id)
}

func mergeJavDiscoveryDetails(existing, details jav.JavBusDiscoveryItem) jav.JavBusDiscoveryItem {
	if strings.TrimSpace(details.Code) == "" {
		details.Code = existing.Code
	}
	if strings.TrimSpace(details.Title) == "" {
		details.Title = existing.Title
	}
	if details.ReleaseUnix <= 0 {
		details.ReleaseUnix = existing.ReleaseUnix
	}
	if details.DurationMin <= 0 {
		details.DurationMin = existing.DurationMin
	}
	if strings.TrimSpace(details.CoverURL) == "" {
		details.CoverURL = existing.CoverURL
	}
	if strings.TrimSpace(details.ThumbnailURL) == "" {
		details.ThumbnailURL = existing.ThumbnailURL
		if strings.TrimSpace(details.ThumbnailURL) == "" && existing.DetailsFetchedAt == nil {
			details.ThumbnailURL = existing.CoverURL
		}
	}
	if strings.TrimSpace(details.DetailURL) == "" {
		details.DetailURL = existing.DetailURL
	}
	if len(details.Actresses) == 0 {
		details.Actresses = existing.Actresses
	}
	if strings.TrimSpace(details.Studio) == "" {
		details.Studio = existing.Studio
	}
	if strings.TrimSpace(details.Series) == "" {
		details.Series = existing.Series
	}
	if len(details.Tags) == 0 {
		details.Tags = existing.Tags
	}
	if len(details.SampleImages) == 0 {
		details.SampleImages = existing.SampleImages
	}
	if details.IsUncensored == nil {
		details.IsUncensored = existing.IsUncensored
	}
	if strings.TrimSpace(details.Source) == "" {
		details.Source = existing.Source
	}
	if details.DetailsFetchedAt == nil {
		details.DetailsFetchedAt = existing.DetailsFetchedAt
	}
	return details
}

func mergeJavDiscoveryListing(existing, listing jav.JavBusDiscoveryItem) jav.JavBusDiscoveryItem {
	result := existing
	if strings.TrimSpace(result.Code) == "" {
		result.Code = listing.Code
	}
	if strings.TrimSpace(result.Title) == "" {
		result.Title = listing.Title
	}
	if listing.ReleaseUnix > 0 {
		result.ReleaseUnix = listing.ReleaseUnix
	}
	if strings.TrimSpace(result.CoverURL) == "" {
		result.CoverURL = listing.CoverURL
	}
	if strings.TrimSpace(listing.ThumbnailURL) != "" {
		result.ThumbnailURL = listing.ThumbnailURL
	} else if strings.TrimSpace(listing.CoverURL) != "" {
		result.ThumbnailURL = listing.CoverURL
	}
	if strings.TrimSpace(listing.DetailURL) != "" {
		result.DetailURL = listing.DetailURL
	}
	if len(result.Actresses) == 0 {
		result.Actresses = listing.Actresses
	}
	if strings.TrimSpace(result.Source) == "" {
		result.Source = listing.Source
	}
	return result
}

// UpsertJavDiscoveryItems stores listing metadata and associates each item with
// its source subscription without changing a user's wanted selections.
func UpsertJavDiscoveryItems(ctx context.Context, subscriptionID int64, items []jav.JavBusDiscoveryItem) error {
	if common.DB == nil {
		return errors.New("upsert jav discovery items: nil db")
	}
	if subscriptionID <= 0 {
		return errors.New("upsert jav discovery items: invalid subscription id")
	}
	return common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		for _, item := range items {
			code := strings.ToUpper(strings.TrimSpace(item.Code))
			if code == "" {
				continue
			}
			if strings.TrimSpace(item.ThumbnailURL) == "" {
				item.ThumbnailURL = item.CoverURL
			}
			var existing models.JavDiscoveryItem
			if err := tx.Select("metadata_json").
				Where("code = ?", code).
				Take(&existing).Error; err == nil {
				var existingMetadata jav.JavBusDiscoveryItem
				if json.Unmarshal([]byte(existing.MetadataJSON), &existingMetadata) == nil &&
					existingMetadata.DetailsFetchedAt != nil {
					item = mergeJavDiscoveryListing(existingMetadata, item)
				}
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("load existing jav discovery metadata for %s: %w", code, err)
			}
			metadata, err := json.Marshal(item)
			if err != nil {
				return fmt.Errorf("marshal jav discovery metadata for %s: %w", code, err)
			}
			record := models.JavDiscoveryItem{
				Code:         code,
				ReleaseUnix:  item.ReleaseUnix,
				MetadataJSON: string(metadata),
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "code"}},
				DoUpdates: clause.Assignments(map[string]any{
					"release_unix":  item.ReleaseUnix,
					"metadata_json": string(metadata),
					"updated_at":    now,
				}),
			}).Create(&record).Error; err != nil {
				return fmt.Errorf("upsert jav discovery item %s: %w", code, err)
			}
			record.ID = 0
			if err := tx.Select("id").Where("code = ?", code).First(&record).Error; err != nil {
				return fmt.Errorf("find jav discovery item %s: %w", code, err)
			}
			link := models.JavDiscoverySubscriptionItem{
				JavDiscoverySubscriptionID: subscriptionID,
				JavDiscoveryItemID:         record.ID,
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&link).Error; err != nil {
				return fmt.Errorf("associate jav discovery item %s: %w", code, err)
			}
		}
		return nil
	})
}

func MarkJavDiscoverySubscriptionSync(ctx context.Context, id int64, syncErr error) error {
	if common.DB == nil {
		return errors.New("mark jav discovery subscription sync: nil db")
	}
	updates := map[string]any{"last_synced_at": time.Now()}
	if syncErr != nil {
		updates["last_error"] = syncErr.Error()
	} else {
		updates["last_error"] = ""
	}
	if err := common.DB.WithContext(ctx).
		Model(&models.JavDiscoverySubscription{}).
		Where("id = ?", id).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("mark jav discovery subscription sync: %w", err)
	}
	return nil
}
