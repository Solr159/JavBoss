package service

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"javboss/internal/common"
	"javboss/internal/db"
	"javboss/internal/jav"
	"javboss/internal/models"
)

func TestScanJavSeriesAndIdolMetadata(t *testing.T) {
	gdb, err := db.Open(filepath.Join(t.TempDir(), "series-idols.db"))
	if err != nil {
		t.Fatal(err)
	}
	previousDB := common.DB
	common.DB = gdb
	t.Cleanup(func() {
		common.DB = previousDB
		if sqlDB, err := gdb.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	studio := models.JavStudio{Name: "Existing Studio"}
	series := models.JavSeries{Name: "Existing Series"}
	english := models.JavSeries{Name: "English Hint", IsEnglish: true}
	idol := models.JavIdol{Name: "Existing Idol"}
	for _, record := range []any{&studio, &series, &english, &idol} {
		if err := gdb.Create(record).Error; err != nil {
			t.Fatal(err)
		}
	}
	uncensored, censored := true, false
	cases := []struct {
		code                            string
		state                           *bool
		hasSeries, hasIdols, hasEnglish bool
		info                            *jav.JavInfo
		wantSeries, wantIdol            string
		lookup                          bool
	}{
		{code: "UNKNOWN-001", info: &jav.JavInfo{Series: " API Series ", Actors: []string{"API Idol", "API Idol"}}, wantSeries: "API Series", wantIdol: "API Idol", lookup: true},
		{code: "CENSORED-001", state: &censored, hasIdols: true, info: &jav.JavInfo{Series: "API Series", Actors: []string{"Replacement Idol"}}, wantSeries: "API Series", wantIdol: "Existing Idol", lookup: true},
		{code: "UNCENSORED-001", state: &uncensored, hasSeries: true, info: &jav.JavInfo{Series: "Replacement Series", Actors: []string{"API Idol"}}, wantSeries: "Existing Series", wantIdol: "API Idol", lookup: true},
		{code: "ENGLISH-001", hasEnglish: true, info: &jav.JavInfo{Series: "API Series", Actors: []string{"API Idol"}}, wantSeries: "API Series", wantIdol: "API Idol", lookup: true},
		{code: "COMPLETE-001", hasSeries: true, hasIdols: true, wantSeries: "Existing Series", wantIdol: "Existing Idol"},
		{code: "EMPTY-001", info: &jav.JavInfo{Series: "  "}, lookup: true},
		{code: "NOTFOUND-001", lookup: true},
		{code: ""},
		{code: "   "},
	}
	cache := &javScannerLookupCache{values: map[string]jav.JavInfo{}}
	var wantKeys []string
	var rows []models.Jav
	for _, tc := range cases {
		row := models.Jav{Code: tc.code, Title: "Original title", ZhTitle: "Original Chinese title", StudioID: &studio.ID, IsUncensored: tc.state}
		if tc.hasSeries {
			row.SeriesID = &series.ID
		}
		if tc.hasEnglish {
			row.SeriesEnID = &english.ID
		}
		if err := gdb.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
		if tc.hasIdols {
			if err := gdb.Create(&models.JavIdolMap{JavID: row.ID, JavIdolID: idol.ID}).Error; err != nil {
				t.Fatal(err)
			}
		}
		rows = append(rows, row)
		key := "v5:jav:javdb-api:lookup_jav:" + tc.code
		if tc.lookup {
			wantKeys = append(wantKeys, key)
		}
		if tc.info != nil {
			info := *tc.info
			info.Title, info.Studio, info.IsUncensored = "Replacement title", "Replacement Studio", &uncensored
			cache.values[key] = info
		}
	}
	jav.SetCache(cache)
	t.Cleanup(func() { jav.SetCache(nil) })
	if err := ScanJavSeriesAndIdolMetadata(context.Background()); err != nil {
		t.Fatal(err)
	}
	sort.Strings(cache.keys)
	sort.Strings(wantKeys)
	if !reflect.DeepEqual(cache.keys, wantKeys) {
		t.Fatalf("lookups=%v, want %v", cache.keys, wantKeys)
	}
	for i, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			var got models.Jav
			if err := gdb.Preload("Series").Preload("Idols").First(&got, rows[i].ID).Error; err != nil {
				t.Fatal(err)
			}
			if tc.wantSeries == "" {
				if got.SeriesID != nil {
					t.Fatalf("unexpected series: %+v", got.Series)
				}
			} else if got.Series == nil || got.Series.Name != tc.wantSeries || got.Series.IsEnglish {
				t.Fatalf("series=%+v, want %s", got.Series, tc.wantSeries)
			}
			if tc.wantIdol == "" {
				if len(got.Idols) != 0 {
					t.Fatalf("unexpected idols: %+v", got.Idols)
				}
			} else if len(got.Idols) != 1 || got.Idols[0].Name != tc.wantIdol {
				t.Fatalf("idols=%+v, want %s", got.Idols, tc.wantIdol)
			}
			if got.Title != rows[i].Title || got.ZhTitle != rows[i].ZhTitle || !reflect.DeepEqual(got.StudioID, rows[i].StudioID) || !reflect.DeepEqual(got.SeriesEnID, rows[i].SeriesEnID) || !reflect.DeepEqual(got.IsUncensored, tc.state) {
				t.Fatalf("unrelated metadata changed: %+v", got)
			}
		})
	}
	cache.keys = nil
	if err := ScanJavSeriesAndIdolMetadata(context.Background()); err != nil {
		t.Fatal(err)
	}
	sort.Strings(cache.keys)
	if want := []string{"v5:jav:javdb-api:lookup_jav:EMPTY-001", "v5:jav:javdb-api:lookup_jav:NOTFOUND-001"}; !reflect.DeepEqual(cache.keys, want) {
		t.Fatalf("second scan lookups=%v, want %v", cache.keys, want)
	}
}

func TestScanUncensoredJavMetadataStillFillsStudioSeriesAndIdols(t *testing.T) {
	gdb, err := db.Open(filepath.Join(t.TempDir(), "avsox.db"))
	if err != nil {
		t.Fatal(err)
	}
	previousDB := common.DB
	common.DB = gdb
	t.Cleanup(func() {
		common.DB = previousDB
		if sqlDB, err := gdb.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	uncensored := true
	row := models.Jav{Code: "UNC-001", IsUncensored: &uncensored}
	if err := gdb.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	cache := &javScannerLookupCache{values: map[string]jav.JavInfo{
		"v3:jav:avsox:lookup_jav:UNC-001": {Studio: "AVSOX Studio", Series: "AVSOX Series", Actors: []string{"AVSOX Idol"}},
	}}
	jav.SetCache(cache)
	t.Cleanup(func() { jav.SetCache(nil) })
	if err := ScanUncensoredJavMetadata(context.Background()); err != nil {
		t.Fatal(err)
	}
	var got models.Jav
	if err := gdb.Preload("Studio").Preload("Series").Preload("Idols").First(&got, row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Studio == nil || got.Studio.Name != "AVSOX Studio" || got.Series == nil || got.Series.Name != "AVSOX Series" || len(got.Idols) != 1 || got.Idols[0].Name != "AVSOX Idol" {
		t.Fatalf("AVSOX fields not filled: %+v", got)
	}
	if !reflect.DeepEqual(cache.keys, []string{"v3:jav:avsox:lookup_jav:UNC-001"}) {
		t.Fatalf("lookups=%v", cache.keys)
	}
}

type javScannerLookupCache struct {
	values map[string]jav.JavInfo
	keys   []string
}

func (c *javScannerLookupCache) Get(key string, _ time.Time) ([]byte, bool, error) {
	c.keys = append(c.keys, key)
	info, ok := c.values[key]
	if !ok {
		return []byte(`{"status":"not_found"}`), true, nil
	}
	raw, err := json.Marshal(struct {
		Status string      `json:"status"`
		Data   jav.JavInfo `json:"data"`
	}{
		Status: "hit",
		Data:   info,
	})
	return raw, true, err
}

func (c *javScannerLookupCache) Set(string, []byte, time.Time) error {
	return nil
}

func TestScanJavDatabasePromotesStudioWithoutBackfillingSeriesOrCensorState(t *testing.T) {
	gdb, err := db.Open(filepath.Join(t.TempDir(), "english-studio.db"))
	if err != nil {
		t.Fatal(err)
	}
	previousDB := common.DB
	common.DB = gdb
	t.Cleanup(func() {
		common.DB = previousDB
		if sqlDB, err := gdb.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	ctx := context.Background()
	rec, err := db.SaveJavInfo(ctx, &jav.JavInfo{Code: "STUDIO-001", Title: "Original title", Studio: "元の片商", Provider: jav.ProviderJavDBAPI})
	if err != nil {
		t.Fatal(err)
	}
	cache := &javScannerLookupCache{values: map[string]jav.JavInfo{
		"v4:jav:javdatabase:lookup_jav:STUDIO-001": {Studio: "English Studio", Series: "Other Series"},
	}}
	jav.SetCache(cache)
	t.Cleanup(func() { jav.SetCache(nil) })
	if err := ScanJavMetadata(ctx); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cache.keys, []string{"v4:jav:javdatabase:lookup_jav:STUDIO-001"}) {
		t.Fatalf("requests=%v", cache.keys)
	}
	var stored models.Jav
	if err := gdb.Preload("Studio").Preload("SeriesEn").First(&stored, rec.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Studio.Name != "English Studio" || *stored.StudioID != *rec.StudioID || stored.Title != "Original title" || stored.SeriesEnID != nil || stored.IsUncensored != nil {
		t.Fatalf("unexpected metadata: %+v", stored)
	}
	var alias models.JavStudioAlias
	if err := gdb.Where("alias = ?", "元の片商").First(&alias).Error; err != nil {
		t.Fatal(err)
	}
	if alias.JavStudioID != *stored.StudioID {
		t.Fatal("wrong alias owner")
	}
	cache.keys = nil
	if err := ScanJavMetadata(ctx); err != nil {
		t.Fatal(err)
	}
	if len(cache.keys) != 0 {
		t.Fatalf("promoted studio selected again: %v", cache.keys)
	}
}
