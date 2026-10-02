package enrichment

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"javboss/internal/jav"
	"javboss/internal/models"
)

func TestEnrichSeriesOnlyFillsMissingField(t *testing.T) {
	for _, task := range []struct {
		name       string
		uncensored bool
		run        func(context.Context) error
	}{
		{"censored", false, EnrichCensoredSeries},
		{"uncensored", true, EnrichUncensoredSeries},
	} {
		t.Run(task.name, func(t *testing.T) {

			gdb := openEnrichmentTestDB(t)
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
				{code: "UNCENSORED-API-001", state: &uncensored, info: &jav.JavInfo{Series: "API Series", Actors: []string{"API Idol"}}, wantSeries: "API Series", wantIdol: "API Idol", lookup: true},
				{code: "COMPLETE-001", hasSeries: true, hasIdols: true, wantSeries: "Existing Series", wantIdol: "Existing Idol"},
				{code: "EMPTY-001", info: &jav.JavInfo{Series: "  "}, lookup: true},
				{code: "NOTFOUND-001", lookup: true},
				{code: ""},
				{code: "   "},
			}
			cache := &metadataLookupCache{values: map[string]jav.JavInfo{}}
			var wantKeys []string
			var rows []models.Jav
			for _, tc := range cases {
				eligible := (tc.state != nil && *tc.state) == task.uncensored
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

				cases[len(rows)].wantIdol = ""
				if tc.hasIdols {
					cases[len(rows)].wantIdol = "Existing Idol"
				}
				if !eligible {
					cases[len(rows)].wantSeries = ""
					if tc.hasSeries {
						cases[len(rows)].wantSeries = "Existing Series"
					}
				}
				rows = append(rows, row)
				key := "v5:jav:javdb-api:lookup_jav:" + tc.code
				if eligible && tc.lookup && !tc.hasSeries {
					wantKeys = append(wantKeys, key)
				}
				if tc.info != nil {
					info := *tc.info
					info.Title, info.Studio, info.IsUncensored = "Replacement title", "Replacement Studio", &uncensored
					cache.values[key] = info
				}
			}
			if !task.uncensored {
				wantKeys = append(wantKeys, "v2:jav:javmenu:lookup_jav:EMPTY-001", "v2:jav:javmenu:lookup_jav:NOTFOUND-001")
			}
			jav.SetCache(cache)
			t.Cleanup(func() { jav.SetCache(nil) })
			if err := task.run(context.Background()); err != nil {
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
			if err := task.run(context.Background()); err != nil {
				t.Fatal(err)
			}
			sort.Strings(cache.keys)
			var want []string
			if !task.uncensored {
				want = []string{"v2:jav:javmenu:lookup_jav:EMPTY-001", "v2:jav:javmenu:lookup_jav:NOTFOUND-001", "v5:jav:javdb-api:lookup_jav:EMPTY-001", "v5:jav:javdb-api:lookup_jav:NOTFOUND-001"}
			}
			if !reflect.DeepEqual(cache.keys, want) {
				t.Fatalf("second scan lookups=%v, want %v", cache.keys, want)
			}

		})
	}
}
