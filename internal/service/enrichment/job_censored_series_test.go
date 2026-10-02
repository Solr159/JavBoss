package enrichment

import (
	"context"
	"reflect"
	"testing"

	"javboss/internal/jav"
	"javboss/internal/models"
)

func TestEnrichCensoredSeriesJavMenuFallback(t *testing.T) {
	uncensored, censored := true, false
	for _, tc := range []struct {
		name     string
		state    *bool
		api      *jav.JavInfo
		existing bool
		want     string
		wantKeys []string
	}{
		{"API not found", &censored, nil, false, "Menu Series", []string{"v5:jav:javdb-api:lookup_jav:SER-001", "v2:jav:javmenu:lookup_jav:SER-001"}},
		{"API series empty", &censored, &jav.JavInfo{Series: "  "}, false, "Menu Series", []string{"v5:jav:javdb-api:lookup_jav:SER-001", "v2:jav:javmenu:lookup_jav:SER-001"}},
		{"API preferred", &censored, &jav.JavInfo{Series: "API Series"}, false, "API Series", []string{"v5:jav:javdb-api:lookup_jav:SER-001"}},
		{"unknown censor state", nil, nil, false, "Menu Series", []string{"v5:jav:javdb-api:lookup_jav:SER-001", "v2:jav:javmenu:lookup_jav:SER-001"}},
		{"uncensored excluded", &uncensored, nil, false, "", nil},
		{"existing series preserved", &censored, nil, true, "Existing Series", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gdb := openEnrichmentTestDB(t)
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
			cache := &metadataLookupCache{values: map[string]jav.JavInfo{
				"v2:jav:javmenu:lookup_jav:SER-001": {Series: " Menu Series ", Studio: "Unwanted Studio", Title: "Unwanted", Actors: []string{"Unwanted Idol"}},
			}}
			if tc.api != nil {
				cache.values["v5:jav:javdb-api:lookup_jav:SER-001"] = *tc.api
			}
			jav.SetCache(cache)
			t.Cleanup(func() { jav.SetCache(nil) })
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
