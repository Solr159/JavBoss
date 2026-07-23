package db

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"javboss/internal/jav"
	"javboss/internal/models"
)

func TestEnglishJavMetadataCannotBePersisted(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	for _, provider := range []jav.Provider{jav.ProviderJavDatabase, jav.ProviderThePornDB} {
		if _, err := SaveJavInfo(ctx, &jav.JavInfo{
			Code:     "EN-ONLY-001",
			Title:    "English title",
			Series:   "English series",
			Actors:   []string{"English Idol"},
			Tags:     []string{"English Tag"},
			Provider: provider,
		}); err == nil {
			t.Fatalf("SaveJavInfo accepted provider %s", provider.String())
		}
	}

	var count int64
	if err := db.Model(&models.Jav{}).Where("code = ?", "EN-ONLY-001").Count(&count).Error; err != nil {
		t.Fatalf("count English JAV rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("English JAV row was persisted: count=%d", count)
	}
}

func TestInternalEnglishSeriesGuidesLocalizedSeriesLookup(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()

	studio := models.JavStudio{Name: "Studio"}
	if err := gdb.Create(&studio).Error; err != nil {
		t.Fatalf("create studio: %v", err)
	}
	javRec := models.Jav{Code: "SERIES-001", Title: "Localized title", StudioID: &studio.ID}
	if err := gdb.Create(&javRec).Error; err != nil {
		t.Fatalf("create jav: %v", err)
	}

	updated, err := UpdateJavEnglishSeriesIfMissing(ctx, javRec.ID, "English Series")
	if err != nil {
		t.Fatalf("UpdateJavEnglishSeriesIfMissing: %v", err)
	}
	if !updated {
		t.Fatal("expected internal English series to be stored")
	}

	candidates, err := ListJavsMissingLocalSeriesWithEnglishSeries(ctx)
	if err != nil {
		t.Fatalf("ListJavsMissingLocalSeriesWithEnglishSeries: %v", err)
	}
	if len(candidates) != 1 || candidates[0].ID != javRec.ID || candidates[0].SeriesEnID == nil {
		t.Fatalf("unexpected localized-series candidates: %#v", candidates)
	}
	englishSeriesID := *candidates[0].SeriesEnID
	if _, err := UpdateJav(ctx, javRec.ID, JavUpdateInput{SeriesID: &englishSeriesID}, nil); err == nil {
		t.Fatal("frontend edit accepted an internal English series")
	}

	if err := UpdateJavSeries(ctx, javRec.ID, "中文系列"); err != nil {
		t.Fatalf("UpdateJavSeries: %v", err)
	}
	var stored models.Jav
	if err := gdb.Preload("Series").Preload("SeriesEn").First(&stored, javRec.ID).Error; err != nil {
		t.Fatalf("load jav with series: %v", err)
	}
	if stored.Series == nil || stored.Series.Name != "中文系列" || stored.Series.IsEnglish {
		t.Fatalf("unexpected localized series: %#v", stored.Series)
	}
	if stored.SeriesEn == nil || stored.SeriesEn.Name != "English Series" || !stored.SeriesEn.IsEnglish {
		t.Fatalf("unexpected internal English series: %#v", stored.SeriesEn)
	}

	payload, err := json.Marshal(stored)
	if err != nil {
		t.Fatalf("marshal jav: %v", err)
	}
	for _, hidden := range []string{"series_en", "is_english", "English Series"} {
		if strings.Contains(string(payload), hidden) {
			t.Fatalf("internal English series leaked into API JSON: %s", payload)
		}
	}
}
