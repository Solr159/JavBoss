package db

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"javboss/internal/jav"
	"javboss/internal/models"
)

func TestJavDiscoveryWantedIsSubsetAndSurvivesRefresh(t *testing.T) {
	openTestDB(t)
	ctx := context.Background()
	subscription := models.JavDiscoverySubscription{
		Kind:          "idol",
		Name:          "葵つかさ",
		ReferenceCode: "ABP-001",
		ProviderKey:   "star-key",
	}
	if err := CreateJavDiscoverySubscription(ctx, &subscription); err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	first := jav.JavBusDiscoveryItem{
		Code:        "ABP-001",
		Title:       "First title",
		ReleaseUnix: 100,
		Source:      "javbus",
	}
	if err := UpsertJavDiscoveryItems(ctx, subscription.ID, []jav.JavBusDiscoveryItem{first}); err != nil {
		t.Fatalf("upsert items: %v", err)
	}
	all, total, err := ListJavDiscoveryItems(ctx, false, 10, 0)
	if err != nil {
		t.Fatalf("list all items: %v", err)
	}
	if total != 1 || len(all) != 1 || all[0].Wanted {
		t.Fatalf("unexpected discovered items: total=%d items=%#v", total, all)
	}
	if len(all[0].Subscriptions) != 1 || all[0].Subscriptions[0] != subscription.Name {
		t.Fatalf("subscriptions = %#v", all[0].Subscriptions)
	}
	if err := SetJavDiscoveryItemWanted(ctx, all[0].ID, true); err != nil {
		t.Fatalf("mark wanted: %v", err)
	}

	first.Title = "Refreshed title"
	first.ReleaseUnix = 200
	if err := UpsertJavDiscoveryItems(ctx, subscription.ID, []jav.JavBusDiscoveryItem{first}); err != nil {
		t.Fatalf("refresh items: %v", err)
	}
	wanted, wantedTotal, err := ListJavDiscoveryItems(ctx, true, 10, 0)
	if err != nil {
		t.Fatalf("list wanted items: %v", err)
	}
	if wantedTotal != 1 || len(wanted) != 1 || !wanted[0].Wanted {
		t.Fatalf("wanted is not a subset after refresh: total=%d items=%#v", wantedTotal, wanted)
	}
	if wanted[0].ReleaseUnix != 200 {
		t.Fatalf("release unix = %d, want refreshed value", wanted[0].ReleaseUnix)
	}
}

func TestJavDiscoveryDetailsSurviveListingRefresh(t *testing.T) {
	openTestDB(t)
	ctx := context.Background()
	subscription := models.JavDiscoverySubscription{
		Kind:          "idol",
		Name:          "葵つかさ",
		ReferenceCode: "ABP-001",
		ProviderKey:   "star-key",
	}
	if err := CreateJavDiscoverySubscription(ctx, &subscription); err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	listing := jav.JavBusDiscoveryItem{
		Code:        "ABP-001",
		Title:       "Listing title",
		ReleaseUnix: 100,
		CoverURL:    "https://example.com/listing.jpg",
		DetailURL:   "https://www.javbus.com/ABP-001",
		Source:      "javbus",
	}
	if err := UpsertJavDiscoveryItems(ctx, subscription.ID, []jav.JavBusDiscoveryItem{listing}); err != nil {
		t.Fatalf("upsert listing: %v", err)
	}
	items, _, err := ListJavDiscoveryItems(ctx, false, 10, 0)
	if err != nil || len(items) != 1 {
		t.Fatalf("list items: items=%#v err=%v", items, err)
	}
	fetchedAt := time.Now().UTC()
	uncensored := false
	details := jav.JavBusDiscoveryItem{
		Code:             "ABP-001",
		Title:            "Complete detail title",
		ReleaseUnix:      100,
		DurationMin:      120,
		CoverURL:         "https://example.com/detail.jpg",
		DetailURL:        "https://www.javbus.com/ABP-001",
		Actresses:        []string{"葵つかさ"},
		Studio:           "Prestige",
		Series:           "Absolute",
		Tags:             []string{"高清", "单体作品"},
		IsUncensored:     &uncensored,
		Source:           "javbus",
		DetailsFetchedAt: &fetchedAt,
	}
	if _, err := UpdateJavDiscoveryItemDetails(ctx, items[0].ID, details); err != nil {
		t.Fatalf("update details: %v", err)
	}

	listing.Title = "Refreshed listing title"
	listing.ReleaseUnix = 200
	if err := UpsertJavDiscoveryItems(ctx, subscription.ID, []jav.JavBusDiscoveryItem{listing}); err != nil {
		t.Fatalf("refresh listing: %v", err)
	}
	record, err := GetJavDiscoveryItem(ctx, items[0].ID)
	if err != nil {
		t.Fatalf("get refreshed item: %v", err)
	}
	var metadata jav.JavBusDiscoveryItem
	if err := json.Unmarshal([]byte(record.MetadataJSON), &metadata); err != nil {
		t.Fatalf("unmarshal refreshed metadata: %v", err)
	}
	if record.ReleaseUnix != 200 || metadata.ReleaseUnix != 200 {
		t.Fatalf("release unix = record:%d metadata:%d, want 200", record.ReleaseUnix, metadata.ReleaseUnix)
	}
	if metadata.Title != details.Title ||
		metadata.Studio != details.Studio ||
		metadata.Series != details.Series ||
		len(metadata.Tags) != len(details.Tags) ||
		metadata.DetailsFetchedAt == nil {
		t.Fatalf("full details were overwritten by listing refresh: %#v", metadata)
	}
	coverURL, err := GetJavDiscoveryItemCoverURL(ctx, items[0].ID)
	if err != nil || coverURL != details.CoverURL {
		t.Fatalf("detail cover URL = %q, err=%v", coverURL, err)
	}
	thumbnailURL, err := GetJavDiscoveryItemThumbnailURL(ctx, items[0].ID)
	if err != nil || thumbnailURL != listing.CoverURL {
		t.Fatalf("listing thumbnail URL = %q, err=%v", thumbnailURL, err)
	}
}
