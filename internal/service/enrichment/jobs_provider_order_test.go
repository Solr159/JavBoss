package enrichment

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"javboss/internal/jav"
	"javboss/internal/models"
)

func TestEnrichmentTriesFallbackBeforeMovingToNextJav(t *testing.T) {
	for _, task := range []struct {
		name       string
		uncensored bool
		run        func(context.Context) error
		fallback   string
		result     jav.JavInfo
	}{
		{"censored series", false, EnrichCensoredSeries, "v2:jav:javmenu:lookup_jav:", jav.JavInfo{Series: "Fallback Series"}},
		{"uncensored series", true, EnrichUncensoredSeries, "v3:jav:avsox:lookup_jav:", jav.JavInfo{Series: "Fallback Series"}},
		{"uncensored idols", true, EnrichUncensoredIdols, "v3:jav:avsox:lookup_jav:", jav.JavInfo{Actors: []string{"Fallback Idol"}}},
	} {
		t.Run(task.name, func(t *testing.T) {
			gdb := openEnrichmentTestDB(t)
			cache := &metadataLookupCache{values: map[string]jav.JavInfo{}}
			for _, code := range []string{"ORDER-001", "ORDER-002"} {
				row := models.Jav{Code: code, IsUncensored: &task.uncensored}
				if err := gdb.Create(&row).Error; err != nil {
					t.Fatal(err)
				}
				cache.values["v5:jav:javdb-api:lookup_jav:"+code] = jav.JavInfo{Series: "  ", Actors: []string{"", "  "}}
				cache.values[task.fallback+code] = task.result
			}
			jav.SetCache(cache)
			t.Cleanup(func() { jav.SetCache(nil) })
			if err := task.run(context.Background()); err != nil {
				t.Fatal(err)
			}
			if len(cache.keys) != 4 {
				t.Fatalf("lookups=%v, want two providers per JAV", cache.keys)
			}
			// Candidate order is shuffled; each JAV's fallback must immediately
			// follow its primary lookup, before any lookup for the next JAV.
			for i := 0; i < len(cache.keys); i += 2 {
				code := strings.TrimPrefix(cache.keys[i], "v5:jav:javdb-api:lookup_jav:")
				want := []string{"v5:jav:javdb-api:lookup_jav:" + code, task.fallback + code}
				if !reflect.DeepEqual(cache.keys[i:i+2], want) {
					t.Fatalf("provider lookups are not grouped per JAV: %v", cache.keys)
				}
				var got models.Jav
				if err := gdb.Preload("Series").Preload("Idols").Where("code = ?", code).First(&got).Error; err != nil {
					t.Fatal(err)
				}
				if task.result.Series != "" {
					if got.Series == nil || got.Series.Name != task.result.Series {
						t.Fatalf("series not filled: %+v", got.Series)
					}
				} else if len(got.Idols) != 1 || got.Idols[0].Name != task.result.Actors[0] {
					t.Fatalf("idols not filled: %+v", got.Idols)
				}
			}
		})
	}
}
