package db

import (
	"context"
	"math"
	"testing"

	"javboss/internal/jav/scrape/metadata"
	"javboss/internal/models"
)

func TestJavCoverCropPersistence(t *testing.T) {
	deps := newTestDependencies()
	database := openTestDB(deps, t)
	ctx := context.Background()
	info := &metadata.JavInfo{Code: "CROP-001", Title: "Cover", Provider: metadata.ProviderJavBus}
	item, err := deps.store.SaveJavInfo(ctx, info)
	if err != nil {
		t.Fatal(err)
	}
	if item.CoverCropPosition != 1 {
		t.Fatalf("default position = %v", item.CoverCropPosition)
	}
	for _, position := range []float64{0.4, 1, 0} {
		updated, err := deps.store.UpdateJav(ctx, item.ID, JavUpdateInput{CoverCropPosition: &position}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if updated.CoverCropPosition != position || updated.Title != info.Title {
			t.Fatalf("unexpected update: %#v", updated)
		}
		if _, err := deps.store.SaveJavInfo(ctx, info); err != nil {
			t.Fatal(err)
		}
		var stored models.Jav
		if err := database.First(&stored, item.ID).Error; err != nil {
			t.Fatal(err)
		}
		if stored.CoverCropPosition != position {
			t.Fatalf("scrape changed crop: %v, want %v", stored.CoverCropPosition, position)
		}
	}
	for _, position := range []float64{-0.01, 1.01, math.NaN(), math.Inf(1)} {
		if _, err := deps.store.UpdateJav(ctx, item.ID, JavUpdateInput{CoverCropPosition: &position}, nil); err == nil {
			t.Fatalf("accepted invalid position %v", position)
		}
	}
	updatedTitle := "Updated title"
	updated, err := deps.store.UpdateJav(ctx, item.ID, JavUpdateInput{Title: &updatedTitle}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if updated.CoverCropPosition != 0 {
		t.Fatalf("metadata update changed crop: %v", updated.CoverCropPosition)
	}
}
