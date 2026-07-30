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
	Subscriptions []string        `json:"subscriptions"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
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
	subscription.ProviderKey = strings.TrimSpace(subscription.ProviderKey)
	if subscription.Kind == "" || subscription.Name == "" || subscription.ReferenceCode == "" || subscription.ProviderKey == "" {
		return errors.New("create jav discovery subscription: missing required field")
	}

	var count int64
	if err := common.DB.WithContext(ctx).
		Model(&models.JavDiscoverySubscription{}).
		Where("kind = ? AND provider_key = ?", subscription.Kind, subscription.ProviderKey).
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

func ListJavDiscoveryItems(ctx context.Context, wantedOnly bool, limit, offset int) ([]JavDiscoveryItemResult, int64, error) {
	if common.DB == nil {
		return nil, 0, errors.New("list jav discovery items: nil db")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	base := common.DB.WithContext(ctx).Model(&models.JavDiscoveryItem{})
	if wantedOnly {
		base = base.Where("wanted = ?", true)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count jav discovery items: %w", err)
	}

	var records []models.JavDiscoveryItem
	query := common.DB.WithContext(ctx)
	if wantedOnly {
		query = query.Where("wanted = ?", true)
	}
	if err := query.
		Order("release_unix DESC, code DESC").
		Limit(limit).
		Offset(offset).
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
			Subscriptions: subscriptionsByItem[record.ID],
			CreatedAt:     record.CreatedAt,
			UpdatedAt:     record.UpdatedAt,
		})
	}
	return items, total, nil
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
