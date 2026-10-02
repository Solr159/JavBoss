package db

import (
	"context"
	"testing"

	"javboss/internal/jav/metadata"
	"javboss/internal/models"
)

func TestAutomaticJavMetadataDefersStudioAndSeriesToEnrichment(t *testing.T) {
	for _, provider := range []metadata.Provider{
		metadata.ProviderJavBus, metadata.ProviderJavDB, metadata.ProviderJavDBAPI,
		metadata.ProviderAvmoo, metadata.ProviderAvsox, metadata.ProviderJavMenu,
		metadata.ProviderUnknown,
	} {
		t.Run(provider.String(), func(t *testing.T) {
			gdb := openTestDB(t)
			ctx := context.Background()
			info := &metadata.JavInfo{Code: "STUDIO-001", Title: "Title", Studio: "Scraped Studio", Series: "Scraped Series", Provider: provider}
			rec, err := SaveJavInfo(ctx, info)
			if err != nil {
				t.Fatal(err)
			}
			if rec.StudioID != nil || rec.SeriesID != nil {
				t.Fatal("automatic save filled studio or series")
			}
			var count int64
			if err := gdb.Model(&models.JavStudio{}).Count(&count).Error; err != nil || count != 0 {
				t.Fatalf("automatic save created studio: count=%d err=%v", count, err)
			}
			if err := gdb.Model(&models.JavSeries{}).Count(&count).Error; err != nil || count != 0 {
				t.Fatalf("automatic save created series: count=%d err=%v", count, err)
			}
			if updated, err := UpdateJavSeriesIfMissing(ctx, rec.ID, "Background Series"); err != nil || !updated {
				t.Fatalf("fill series: updated=%v err=%v", updated, err)
			}
			if updated, err := UpdateJavStudioIfMissing(ctx, rec.ID, "Scanner Studio"); err != nil || !updated {
				t.Fatalf("fill studio: updated=%v err=%v", updated, err)
			}
			info.Studio = "Replacement Studio"
			info.Series = "Replacement Series"
			if _, err := SaveJavInfo(ctx, info); err != nil {
				t.Fatal(err)
			}
			assertJavStudio(t, gdb, info.Code, "Scanner Studio")
			assertJavSeries(t, gdb, info.Code, "Background Series")
			if err := gdb.Model(&models.JavSeries{}).Count(&count).Error; err != nil || count != 1 {
				t.Fatalf("refresh created unwanted series: count=%d err=%v", count, err)
			}
			if err := gdb.Model(&models.JavStudio{}).Count(&count).Error; err != nil || count != 1 {
				t.Fatalf("refresh created unwanted studio: count=%d err=%v", count, err)
			}
		})
	}
}

func TestManualJavMetadataSavesAndUpdatesStudioAndSeries(t *testing.T) {
	for _, provider := range []metadata.Provider{metadata.ProviderUser, metadata.ProviderManualScrape} {
		t.Run(provider.String(), func(t *testing.T) {
			gdb := openTestDB(t)
			info := &metadata.JavInfo{Code: "STUDIO-001", Title: "Title", Studio: "Manual Studio", Series: "Manual Series", Provider: provider}
			if _, err := SaveJavInfo(context.Background(), info); err != nil {
				t.Fatal(err)
			}
			assertJavStudio(t, gdb, info.Code, "Manual Studio")
			assertJavSeries(t, gdb, info.Code, "Manual Series")
			info.Studio = "Edited Studio"
			info.Series = "Edited Series"
			if _, err := SaveJavInfo(context.Background(), info); err != nil {
				t.Fatal(err)
			}
			assertJavStudio(t, gdb, info.Code, "Edited Studio")
			assertJavSeries(t, gdb, info.Code, "Edited Series")
		})
	}
}
