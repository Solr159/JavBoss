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
		Kind:            "idol",
		Name:            "葵つかさ",
		ReferenceCode:   "ABP-001",
		ProviderLocator: "/star/star-key",
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
	all, total, err := ListJavDiscoveryItems(ctx, ListJavDiscoveryItemsOptions{Limit: 10})
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
	wanted, wantedTotal, err := ListJavDiscoveryItems(ctx, ListJavDiscoveryItemsOptions{
		WantedOnly: true,
		Limit:      10,
	})
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

func TestListJavDiscoveryItemsFiltersBySubscription(t *testing.T) {
	openTestDB(t)
	ctx := context.Background()
	subscriptions := []models.JavDiscoverySubscription{
		{Kind: "idol", Name: "葵つかさ", ReferenceCode: "ABP-001", ProviderLocator: "/star/star-a"},
		{Kind: "idol", Name: "相沢みなみ", ReferenceCode: "IPX-001", ProviderLocator: "/star/star-b"},
	}
	for index := range subscriptions {
		if err := CreateJavDiscoverySubscription(ctx, &subscriptions[index]); err != nil {
			t.Fatalf("create subscription %d: %v", index, err)
		}
	}
	if err := UpsertJavDiscoveryItems(ctx, subscriptions[0].ID, []jav.JavBusDiscoveryItem{
		{Code: "ABP-001", Title: "First actress", Source: "javbus"},
		{Code: "SHARED-001", Title: "Shared work", Source: "javbus"},
	}); err != nil {
		t.Fatalf("upsert first subscription: %v", err)
	}
	if err := UpsertJavDiscoveryItems(ctx, subscriptions[1].ID, []jav.JavBusDiscoveryItem{
		{Code: "IPX-001", Title: "Second actress", Source: "javbus"},
		{Code: "SHARED-001", Title: "Shared work", Source: "javbus"},
	}); err != nil {
		t.Fatalf("upsert second subscription: %v", err)
	}

	all, total, err := ListJavDiscoveryItems(ctx, ListJavDiscoveryItemsOptions{Limit: 10})
	if err != nil {
		t.Fatalf("list all discovery items: %v", err)
	}
	if total != 3 || len(all) != 3 {
		t.Fatalf("all items: total=%d items=%#v", total, all)
	}

	first, firstTotal, err := ListJavDiscoveryItems(ctx, ListJavDiscoveryItemsOptions{
		SubscriptionID: subscriptions[0].ID,
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("filter first subscription: %v", err)
	}
	assertDiscoveryCodes(t, first, firstTotal, "ABP-001", "SHARED-001")

	second, secondTotal, err := ListJavDiscoveryItems(ctx, ListJavDiscoveryItemsOptions{
		SubscriptionID: subscriptions[1].ID,
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("filter second subscription: %v", err)
	}
	assertDiscoveryCodes(t, second, secondTotal, "IPX-001", "SHARED-001")
}

func assertDiscoveryCodes(t *testing.T, items []JavDiscoveryItemResult, total int64, want ...string) {
	t.Helper()
	if total != int64(len(want)) || len(items) != len(want) {
		t.Fatalf("total=%d items=%#v, want codes=%#v", total, items, want)
	}
	got := make(map[string]bool, len(items))
	for _, item := range items {
		got[item.Code] = true
	}
	for _, code := range want {
		if !got[code] {
			t.Fatalf("codes=%#v, missing %s", got, code)
		}
	}
}

func TestJavDiscoveryDetailsSurviveListingRefresh(t *testing.T) {
	openTestDB(t)
	ctx := context.Background()
	subscription := models.JavDiscoverySubscription{
		Kind:            "idol",
		Name:            "葵つかさ",
		ReferenceCode:   "ABP-001",
		ProviderLocator: "/star/star-key",
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
	items, _, err := ListJavDiscoveryItems(ctx, ListJavDiscoveryItemsOptions{Limit: 10})
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
		MagnetLinks: []jav.JavBusMagnetLink{{
			Name: "ABP-001-HD", URL: "magnet:?xt=urn:btih:ABC123&dn=ABP-001-HD", Size: "4.2GB",
		}},
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
	var magnetLinks []jav.JavBusMagnetLink
	if err := json.Unmarshal([]byte(record.MagnetLinksJSON), &magnetLinks); err != nil {
		t.Fatalf("unmarshal magnet links: %v", err)
	}
	if len(magnetLinks) != 1 || magnetLinks[0].URL != details.MagnetLinks[0].URL {
		t.Fatalf("magnet links were not preserved: %#v", magnetLinks)
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
