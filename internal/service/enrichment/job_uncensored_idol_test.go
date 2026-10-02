package enrichment

import (
	"context"
	"reflect"
	"testing"

	"javboss/internal/jav"
	"javboss/internal/models"
)

func TestEnrichUncensoredIdolsAvsoxFallback(t *testing.T) {
	uncensored, censored := true, false
	for _, tc := range []struct {
		name       string
		state      *bool
		emptyAPI   bool
		wantFilled bool
	}{
		{name: "uncensored API not found", state: &uncensored, wantFilled: true},
		{name: "uncensored API field empty", state: &uncensored, emptyAPI: true, wantFilled: true},
		{name: "censored", state: &censored},
		{name: "unknown censor state"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gdb := openEnrichmentTestDB(t)
			row := models.Jav{Code: "UNC-001", Title: "Original", IsUncensored: tc.state}
			if err := gdb.Create(&row).Error; err != nil {
				t.Fatal(err)
			}
			cache := &metadataLookupCache{values: map[string]jav.JavInfo{
				"v3:jav:avsox:lookup_jav:UNC-001": {Studio: "Unwanted Studio", Series: "AVSOX Series", Actors: []string{"AVSOX Idol"}, Title: "Unwanted title"},
			}}
			if tc.emptyAPI {
				cache.values["v5:jav:javdb-api:lookup_jav:UNC-001"] = jav.JavInfo{}
			}
			jav.SetCache(cache)
			t.Cleanup(func() { jav.SetCache(nil) })
			if err := EnrichUncensoredIdols(context.Background()); err != nil {
				t.Fatal(err)
			}
			var got models.Jav
			if err := gdb.Preload("Series").Preload("Idols").First(&got, row.ID).Error; err != nil {
				t.Fatal(err)
			}
			if tc.wantFilled {
				if len(got.Idols) != 1 || got.Idols[0].Name != "AVSOX Idol" {
					t.Fatalf("field not filled: %+v", got)
				}
			} else if len(got.Idols) != 0 {
				t.Fatalf("AVSOX used for non-uncensored JAV: %+v", got)
			}
			if got.StudioID != nil || got.Title != row.Title || got.SeriesID != nil || !reflect.DeepEqual(got.IsUncensored, tc.state) {
				t.Fatalf("unrelated metadata changed: %+v", got)
			}
			var wantKeys []string
			if tc.wantFilled {
				wantKeys = []string{"v5:jav:javdb-api:lookup_jav:UNC-001", "v3:jav:avsox:lookup_jav:UNC-001"}
			}
			if !reflect.DeepEqual(cache.keys, wantKeys) {
				t.Fatalf("lookups=%v, want %v", cache.keys, wantKeys)
			}
			if tc.wantFilled {
				cache.keys = nil
				if err := EnrichUncensoredIdols(context.Background()); err != nil {
					t.Fatal(err)
				}
				if len(cache.keys) != 0 {
					t.Fatalf("filled field looked up again: %v", cache.keys)
				}
			}
		})
	}
}
