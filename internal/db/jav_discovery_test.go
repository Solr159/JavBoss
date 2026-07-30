package db

import (
	"context"
	"testing"

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
