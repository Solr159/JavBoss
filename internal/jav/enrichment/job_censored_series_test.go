package enrichment

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"javboss/internal/jav/scrape"
	"javboss/internal/models"
)

func TestEnrichCensoredSeriesJavMenuFallback(t *testing.T) {
	deps := newTestDependencies()
	uncensored, censored := true, false
	for _, tc := range []struct {
		name     string
		state    *bool
		api      *scrape.JavInfo
		existing bool
		want     string
		wantKeys []string
	}{
		{"API not found", &censored, nil, false, "Menu Series", []string{"v5:jav:javdb-api:lookup_jav:SER-001", "v2:jav:javmenu:lookup_jav:SER-001"}},
		{"API series empty", &censored, &scrape.JavInfo{Series: "  "}, false, "Menu Series", []string{"v5:jav:javdb-api:lookup_jav:SER-001", "v2:jav:javmenu:lookup_jav:SER-001"}},
		{"API preferred", &censored, &scrape.JavInfo{Series: "API Series"}, false, "API Series", []string{"v5:jav:javdb-api:lookup_jav:SER-001"}},
		{"unknown censor state", nil, nil, false, "Menu Series", []string{"v5:jav:javdb-api:lookup_jav:SER-001", "v2:jav:javmenu:lookup_jav:SER-001"}},
		{"uncensored excluded", &uncensored, nil, false, "", nil},
		{"existing series preserved", &censored, nil, true, "Existing Series", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gdb := openEnrichmentTestDB(deps, t)
			studio := models.JavStudio{Name: "Existing Studio"}
			if err := gdb.Create(&studio).Error; err != nil {
				t.Fatal(err)
			}
			row := models.Jav{Code: "SER-001", Title: "Original", StudioID: &studio.ID, IsUncensored: tc.state}
			if tc.existing {
				series := models.JavSeries{Name: "Existing Series"}
				if err := gdb.Create(&series).Error; err != nil {
					t.Fatal(err)
				}
				row.SeriesID = &series.ID
			}
			if err := gdb.Create(&row).Error; err != nil {
				t.Fatal(err)
			}
			cache := &metadataLookupCache{values: map[string]scrape.JavInfo{
				"v2:jav:javmenu:lookup_jav:SER-001": {Series: " Menu Series ", Studio: "Unwanted Studio", Title: "Unwanted", Actors: []string{"Unwanted Idol"}},
			}}
			if tc.api != nil {
				cache.values["v5:jav:javdb-api:lookup_jav:SER-001"] = *tc.api
			}
			scrape.SetCache(cache)
			t.Cleanup(func() { scrape.SetCache(nil) })
			if err := EnrichCensoredSeries(context.Background()); err != nil {
				t.Fatal(err)
			}
			var got models.Jav
			if err := gdb.Preload("Series").Preload("Idols").First(&got, row.ID).Error; err != nil {
				t.Fatal(err)
			}
			if tc.want == "" {
				if got.SeriesID != nil {
					t.Fatalf("unexpected series: %+v", got.Series)
				}
			} else if got.Series == nil || got.Series.Name != tc.want {
				t.Fatalf("series=%+v, want %q", got.Series, tc.want)
			}
			if got.Title != row.Title || !reflect.DeepEqual(got.StudioID, row.StudioID) || !reflect.DeepEqual(got.IsUncensored, row.IsUncensored) || len(got.Idols) != 0 {
				t.Fatalf("unrelated metadata changed: %+v", got)
			}
			if !reflect.DeepEqual(cache.keys, tc.wantKeys) {
				t.Fatalf("lookups=%v, want %v", cache.keys, tc.wantKeys)
			}
		})
	}
}

func TestEnrichCensoredSeriesCompositeProvider(t *testing.T) {
	deps := newTestDependencies()
	for _, tc := range []struct {
		name       string
		info       map[scrape.Provider]*scrape.JavInfo
		errs       map[scrape.Provider]error
		wantSeries string
		wantCalls  int
	}{
		{name: "both series empty", wantSeries: "Avmoo Series", wantCalls: 4},
		{name: "API preferred", info: map[scrape.Provider]*scrape.JavInfo{scrape.ProviderJavDBAPI: {Series: "API Series"}}, wantSeries: "API Series", wantCalls: 1},
		{name: "menu preferred", info: map[scrape.Provider]*scrape.JavInfo{scrape.ProviderJavMenu: {Series: "Menu Series"}}, wantSeries: "Menu Series", wantCalls: 2},
		{name: "both not found", errs: map[scrape.Provider]error{scrape.ProviderJavDBAPI: scrape.ErrNotFound, scrape.ProviderJavMenu: scrape.ErrNotFound}, wantSeries: "Avmoo Series", wantCalls: 4},
		{name: "API failed", errs: map[scrape.Provider]error{scrape.ProviderJavDBAPI: errors.New("request failed")}, wantSeries: "Avmoo Series", wantCalls: 4},
		{name: "menu failed", errs: map[scrape.Provider]error{scrape.ProviderJavMenu: errors.New("request failed")}, wantSeries: "Avmoo Series", wantCalls: 4},
		{name: "both returned nil", info: map[scrape.Provider]*scrape.JavInfo{scrape.ProviderJavDBAPI: nil, scrape.ProviderJavMenu: nil}, wantSeries: "Avmoo Series", wantCalls: 4},
		{name: "probe series empty", info: map[scrape.Provider]*scrape.JavInfo{scrape.ProviderJavDatabase: {Series: "  "}}, wantCalls: 3},
		{name: "probe nil", info: map[scrape.Provider]*scrape.JavInfo{scrape.ProviderJavDatabase: nil}, wantCalls: 3},
		{name: "probe not found", errs: map[scrape.Provider]error{scrape.ProviderJavDatabase: scrape.ErrNotFound}, wantCalls: 3},
		{name: "probe failed", errs: map[scrape.Provider]error{scrape.ProviderJavDatabase: errors.New("request failed")}, wantCalls: 3},
		{name: "Avmoo series empty", info: map[scrape.Provider]*scrape.JavInfo{scrape.ProviderAvmoo: {Series: "  "}}, wantCalls: 4},
		{name: "Avmoo nil", info: map[scrape.Provider]*scrape.JavInfo{scrape.ProviderAvmoo: nil}, wantCalls: 4},
		{name: "Avmoo not found", errs: map[scrape.Provider]error{scrape.ProviderAvmoo: scrape.ErrNotFound}, wantCalls: 4},
		{name: "Avmoo failed", errs: map[scrape.Provider]error{scrape.ProviderAvmoo: errors.New("request failed")}, wantCalls: 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gdb := openEnrichmentTestDB(deps, t)
			censored := false
			row := models.Jav{Code: "SER-001", Title: "Original", IsUncensored: &censored}
			if err := gdb.Create(&row).Error; err != nil {
				t.Fatal(err)
			}
			results := map[scrape.Provider]*scrape.JavInfo{
				scrape.ProviderJavDBAPI:    {},
				scrape.ProviderJavMenu:     {Series: "  "},
				scrape.ProviderJavDatabase: {Series: "English Probe Series"},
				scrape.ProviderAvmoo:       {Series: " Avmoo Series ", Studio: "Unwanted Studio", Title: "Unwanted Title", Actors: []string{"Unwanted Idol"}},
			}
			for provider, info := range tc.info {
				results[provider] = info
			}
			var calls []scrape.Provider
			err := enrichCensoredSeries(context.Background(), func(_ context.Context, code string, provider scrape.Provider) (*scrape.JavInfo, error) {
				if code != row.Code {
					t.Fatalf("code=%s, want %s", code, row.Code)
				}
				calls = append(calls, provider)
				info, ok := results[provider]
				if !ok {
					t.Fatalf("unexpected provider: %v", provider)
				}
				return info, tc.errs[provider]
			})
			if err != nil {
				t.Fatal(err)
			}
			wantProviders := []scrape.Provider{scrape.ProviderJavDBAPI, scrape.ProviderJavMenu, scrape.ProviderJavDatabase, scrape.ProviderAvmoo}
			if !reflect.DeepEqual(calls, wantProviders[:tc.wantCalls]) {
				t.Fatalf("providers=%v, want %v", calls, wantProviders[:tc.wantCalls])
			}
			var got models.Jav
			if err := gdb.Preload("Series").Preload("Idols").First(&got, row.ID).Error; err != nil {
				t.Fatal(err)
			}
			if tc.wantSeries == "" {
				if got.SeriesID != nil {
					t.Fatalf("unexpected series: %+v", got.Series)
				}
			} else if got.Series == nil || got.Series.Name != tc.wantSeries {
				t.Fatalf("series=%+v, want %s", got.Series, tc.wantSeries)
			}
			if got.Title != row.Title || got.StudioID != nil || got.SeriesEnID != nil || len(got.Idols) != 0 || !reflect.DeepEqual(got.IsUncensored, row.IsUncensored) {
				t.Fatalf("unrelated metadata changed: %+v", got)
			}
			var probeSeriesCount int64
			if err := gdb.Model(&models.JavSeries{}).Where("name = ?", "English Probe Series").Count(&probeSeriesCount).Error; err != nil {
				t.Fatal(err)
			}
			if probeSeriesCount != 0 {
				t.Fatal("English probe series was persisted")
			}
		})
	}
}

func TestEnrichCensoredSeriesStopsAfterCanceledProbe(t *testing.T) {
	deps := newTestDependencies()
	gdb := openEnrichmentTestDB(deps, t)
	row := models.Jav{Code: "SER-001"}
	if err := gdb.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls []scrape.Provider
	err := enrichCensoredSeries(ctx, func(_ context.Context, _ string, provider scrape.Provider) (*scrape.JavInfo, error) {
		calls = append(calls, provider)
		if provider == scrape.ProviderJavDatabase {
			cancel()
			return &scrape.JavInfo{Series: "English Probe Series"}, nil
		}
		return nil, scrape.ErrNotFound
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v, want cancellation", err)
	}
	if !reflect.DeepEqual(calls, []scrape.Provider{scrape.ProviderJavDBAPI, scrape.ProviderJavMenu, scrape.ProviderJavDatabase}) {
		t.Fatalf("unexpected calls after cancellation: %v", calls)
	}
}
