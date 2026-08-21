package service

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"javboss/internal/common"
	"javboss/internal/db"
	"javboss/internal/jav"
	"javboss/internal/models"
)

func TestJavDiscoverySyncExcludesWorksReleasedBeforeSubscription(t *testing.T) {
	subscription := openJavDiscoveryServiceTestDB(t)
	createdDateUnix := javDiscoverySubscriptionDateUnix(subscription.CreatedAt)
	createdDate := time.Unix(createdDateUnix, 0)

	previousFetch := fetchJavBusActressWorks
	fetchJavBusActressWorks = func(
		_ context.Context,
		_, _ string,
		options jav.JavBusActressWorksOptions,
	) ([]jav.JavBusDiscoveryItem, error) {
		if options.Offset != 0 ||
			options.Limit != javDiscoveryLatestScanLimit ||
			options.ReleasedNotBefore != createdDateUnix ||
			options.ReleasedBefore != 0 {
			t.Errorf("latest sync options = %#v", options)
		}
		return []jav.JavBusDiscoveryItem{
			{Code: "NEW-001", ReleaseUnix: createdDate.Unix(), Source: "javbus"},
			{Code: "OLD-001", ReleaseUnix: createdDate.Add(-24 * time.Hour).Unix(), Source: "javbus"},
		}, nil
	}
	t.Cleanup(func() { fetchJavBusActressWorks = previousFetch })

	if err := syncJavDiscoverySubscription(context.Background(), subscription); err != nil {
		t.Fatalf("sync subscription: %v", err)
	}
	items, count, err := db.ListJavDiscoveryItems(context.Background(), db.ListJavDiscoveryItemsOptions{
		IncludeOwned:   true,
		SubscriptionID: subscription.ID,
	})
	if err != nil {
		t.Fatalf("list synced items: %v", err)
	}
	if count != 1 || len(items) != 1 || items[0].Code != "NEW-001" {
		t.Fatalf("synced items = %#v total=%d, want only NEW-001", items, count)
	}
}

func TestLoadMoreJavDiscoveryHistoryLoadsTenAfterExistingItems(t *testing.T) {
	subscription := openJavDiscoveryServiceTestDB(t)
	createdDateUnix := javDiscoverySubscriptionDateUnix(subscription.CreatedAt)
	if err := db.UpsertJavDiscoveryItems(context.Background(), subscription.ID, []jav.JavBusDiscoveryItem{
		{Code: "HISTORY-001", ReleaseUnix: createdDateUnix - 86400, Source: "javbus"},
		{Code: "HISTORY-002", ReleaseUnix: createdDateUnix - 2*86400, Source: "javbus"},
	}); err != nil {
		t.Fatalf("seed subscription items: %v", err)
	}

	previousFetch := fetchJavBusActressWorks
	fetchJavBusActressWorks = func(
		_ context.Context,
		_, _ string,
		options jav.JavBusActressWorksOptions,
	) ([]jav.JavBusDiscoveryItem, error) {
		if options.Offset != 2 ||
			options.Limit != javDiscoveryHistoryBatchSize ||
			options.ReleasedBefore != createdDateUnix ||
			options.ReleasedNotBefore != 0 {
			t.Errorf("history options = %#v", options)
		}
		items := make([]jav.JavBusDiscoveryItem, 0, javDiscoveryHistoryBatchSize)
		for index := 0; index < javDiscoveryHistoryBatchSize; index++ {
			items = append(items, jav.JavBusDiscoveryItem{
				Code:        fmt.Sprintf("HISTORY-%03d", index+3),
				ReleaseUnix: createdDateUnix - int64(index+3)*86400,
				Source:      "javbus",
			})
		}
		return items, nil
	}
	t.Cleanup(func() { fetchJavBusActressWorks = previousFetch })

	loaded, err := LoadMoreJavDiscoveryHistory(context.Background(), subscription.ID)
	if err != nil {
		t.Fatalf("load more history: %v", err)
	}
	if loaded != javDiscoveryHistoryBatchSize {
		t.Fatalf("loaded = %d, want %d", loaded, javDiscoveryHistoryBatchSize)
	}
}

func openJavDiscoveryServiceTestDB(t *testing.T) models.JavDiscoverySubscription {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "discovery.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	previousDB := common.DB
	common.DB = database
	t.Cleanup(func() {
		common.DB = previousDB
		if sqlDB, dbErr := database.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	subscription := models.JavDiscoverySubscription{
		Kind:            "idol",
		Name:            "葵つかさ",
		ReferenceCode:   "ABP-001",
		ProviderLocator: "/star/star-key",
	}
	if err := db.CreateJavDiscoverySubscription(context.Background(), &subscription); err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	return subscription
}
