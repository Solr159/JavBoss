package service

import (
	"context"
	"path/filepath"
	"reflect"
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

func TestShouldScanMissingJavLocalSeriesWithJavMenu(t *testing.T) {
	tests := []struct {
		round uint64
		want  bool
	}{
		{round: 0, want: false},
		{round: 1, want: false},
		{round: 4, want: false},
		{round: 5, want: true},
		{round: 6, want: false},
		{round: 10, want: true},
	}
	for _, test := range tests {
		if got := shouldScanMissingJavLocalSeriesWithJavMenu(test.round); got != test.want {
			t.Errorf("round %d: got %t, want %t", test.round, got, test.want)
		}
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

	calls := map[string]jav.Provider{}
	lookup := func(code string, provider jav.Provider) (*jav.JavInfo, error) {
		calls[code] = provider
		switch code {
		case "NO-HINT-001":
			return &jav.JavInfo{Series: "JavMenu Local Series"}, nil
		case "EN-HINT-001":
			return &jav.JavInfo{}, nil
		default:
			t.Fatalf("unexpected lookup for code %q", code)
			return nil, nil
		}
	}
	if err := scanMissingJavLocalSeriesWithLookup(context.Background(), lookup); err != nil {
		t.Fatalf("scan missing local series: %v", err)
	}

	if len(calls) != 2 {
		t.Fatalf("lookup calls = %#v, want two missing-series candidates", calls)
	}
	for code, provider := range calls {
		if provider != jav.ProviderJavMenu {
			t.Fatalf("lookup provider for %s = %s, want javmenu", code, provider.String())
		}
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
