package service

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"javboss/internal/common"
	"javboss/internal/db"
	"javboss/internal/jav"
	"javboss/internal/models"
)

func TestJavMetadataFastZhProvidersExcludeSlowProviders(t *testing.T) {
	got := javFastZhMetadataProviders()
	want := []jav.Provider{jav.ProviderJavBus}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("javFastZhMetadataProviders() = %#v, want %#v", got, want)
	}
}

func TestScanJavSeriesMetadataProviderRoundFallsBackToJavMenu(t *testing.T) {
	var avmooNoUpdateRounds atomic.Uint32
	avmooUpdates := []int64{0, 0, 3, 0, 0}
	avmooIndex := 0
	var calls []string
	avmooScan := func(context.Context) (int64, error) {
		calls = append(calls, "avmoo")
		updated := avmooUpdates[avmooIndex]
		avmooIndex++
		return updated, nil
	}
	javMenuScan := func(context.Context) (int64, error) {
		calls = append(calls, "javmenu")
		return 1, nil
	}

	for range 7 {
		if err := scanJavSeriesMetadataProviderRound(
			context.Background(),
			&avmooNoUpdateRounds,
			avmooScan,
			javMenuScan,
		); err != nil {
			t.Fatalf("scan provider round: %v", err)
		}
	}

	want := []string{"avmoo", "avmoo", "javmenu", "avmoo", "avmoo", "avmoo", "javmenu"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("provider calls = %#v, want %#v", calls, want)
	}
	if got := avmooNoUpdateRounds.Load(); got != 0 {
		t.Fatalf("avmoo no-update rounds = %d, want 0 after JavMenu fallback", got)
	}
}

func TestScanMissingJavLocalSeriesUsesOnlyJavMenu(t *testing.T) {
	gdb, err := db.Open(filepath.Join(t.TempDir(), "missing-series.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	previousDB := common.DB
	common.DB = gdb
	t.Cleanup(func() {
		common.DB = previousDB
		if sqlDB, dbErr := gdb.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	now := time.Unix(1710000000, 0).UTC()
	series := []models.JavSeries{
		{Name: "Existing Local Series"},
		{Name: "English Hint", IsEnglish: true},
	}
	if err := gdb.Create(&series).Error; err != nil {
		t.Fatalf("create series: %v", err)
	}
	localSeries := series[0]
	englishSeries := series[1]
	uncensored := true
	rows := []models.Jav{
		{Code: "NO-HINT-001", FetchedAt: now, CreatedAt: now},
		{Code: "EN-HINT-001", SeriesEnID: &englishSeries.ID, FetchedAt: now.Add(time.Second), CreatedAt: now.Add(time.Second)},
		{Code: "EXISTING-001", SeriesID: &localSeries.ID, FetchedAt: now.Add(2 * time.Second), CreatedAt: now.Add(2 * time.Second)},
		{Code: "UNCENSORED-001", IsUncensored: &uncensored, FetchedAt: now.Add(3 * time.Second), CreatedAt: now.Add(3 * time.Second)},
	}
	if err := gdb.Create(&rows).Error; err != nil {
		t.Fatalf("create jav rows: %v", err)
	}

	cache := &javScannerLookupCache{values: map[string]jav.JavInfo{
		"v2:jav:javmenu:lookup_jav:NO-HINT-001": {Series: "JavMenu Local Series"},
		"v2:jav:javmenu:lookup_jav:EN-HINT-001": {},
	}}
	jav.SetCache(cache)
	t.Cleanup(func() {
		jav.SetCache(nil)
	})

	updated, err := scanMissingJavLocalSeriesWithJavMenu(context.Background())
	if err != nil {
		t.Fatalf("scan missing local series: %v", err)
	}
	if updated != 1 {
		t.Fatalf("updated series = %d, want 1", updated)
	}

	wantKeys := []string{
		"v2:jav:javmenu:lookup_jav:NO-HINT-001",
		"v2:jav:javmenu:lookup_jav:EN-HINT-001",
	}
	sort.Strings(cache.keys)
	sort.Strings(wantKeys)
	if !reflect.DeepEqual(cache.keys, wantKeys) {
		t.Fatalf("lookup cache keys = %#v, want %#v", cache.keys, wantKeys)
	}

	var filled models.Jav
	if err := gdb.Preload("Series").Where("code = ?", "NO-HINT-001").First(&filled).Error; err != nil {
		t.Fatalf("load filled jav: %v", err)
	}
	if filled.Series == nil || filled.Series.Name != "JavMenu Local Series" {
		t.Fatalf("unexpected filled series: %#v", filled.Series)
	}

	var empty models.Jav
	if err := gdb.Where("code = ?", "EN-HINT-001").First(&empty).Error; err != nil {
		t.Fatalf("load empty-series jav: %v", err)
	}
	if empty.SeriesID != nil {
		t.Fatalf("empty JavMenu series should not be persisted: %#v", empty.SeriesID)
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
		return nil, false, nil
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
