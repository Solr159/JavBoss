package enrichment

import (
	"context"
	"sync"
	"testing"
	"time"

	"javboss/internal/jav"
	"javboss/internal/models"
)

func TestSlowUncensoredProviderDoesNotBlockCensoredEnrichment(t *testing.T) {
	for _, task := range []struct {
		name       string
		censored   func(context.Context) error
		uncensored func(context.Context) error
		filled     func(models.Jav) bool
	}{
		{"studio", EnrichCensoredStudios, EnrichUncensoredStudios, func(row models.Jav) bool { return row.StudioID != nil }},
		{"series", EnrichCensoredSeries, EnrichUncensoredSeries, func(row models.Jav) bool { return row.SeriesID != nil }},
		{"idol", EnrichCensoredIdols, EnrichUncensoredIdols, func(row models.Jav) bool { return len(row.Idols) != 0 }},
	} {
		t.Run(task.name, func(t *testing.T) {
			gdb := openEnrichmentTestDB(t)
			uncensored, censored := true, false
			rows := []models.Jav{
				{Code: "UNC-001", IsUncensored: &uncensored},
				{Code: "CEN-001", IsUncensored: &censored},
				{Code: "UNKNOWN-001"},
			}
			if err := gdb.Create(&rows).Error; err != nil {
				t.Fatal(err)
			}
			started, release := make(chan struct{}), make(chan struct{})
			var startedOnce sync.Once
			cache := &metadataLookupCache{
				values: map[string]jav.JavInfo{
					"v3:jav:avsox:lookup_jav:UNC-001":         {Studio: "Uncensored Studio", Series: "Uncensored Series", Actors: []string{"Uncensored Idol"}},
					"v5:jav:javdb-api:lookup_jav:CEN-001":     {Studio: "Censored Studio", Series: "Censored Series", Actors: []string{"Censored Idol"}},
					"v5:jav:javdb-api:lookup_jav:UNKNOWN-001": {Studio: "Unknown Studio", Series: "Unknown Series", Actors: []string{"Unknown Idol"}},
				},
				beforeGet: func(key string) {
					if key == "v3:jav:avsox:lookup_jav:UNC-001" {
						startedOnce.Do(func() { close(started) })
						<-release
					}
				},
			}
			jav.SetCache(cache)
			t.Cleanup(func() { jav.SetCache(nil) })
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			uncensoredDone := make(chan error, 1)
			censoredDone := make(chan error, 1)
			var workers sync.WaitGroup
			// Always unblock and join workers before restoring global DB/cache state.
			defer func() {
				close(release)
				cancel()
				workers.Wait()
			}()
			workers.Add(1)
			go func() {
				defer workers.Done()
				uncensoredDone <- task.uncensored(ctx)
			}()
			select {
			case <-started:
			case err := <-uncensoredDone:
				t.Fatalf("uncensored enrichment finished before AVSOX lookup: %v", err)
			case <-ctx.Done():
				t.Fatal("AVSOX lookup did not start")
			}
			workers.Add(1)
			go func() {
				defer workers.Done()
				censoredDone <- task.censored(ctx)
			}()
			select {
			case err := <-censoredDone:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal("censored enrichment blocked behind AVSOX")
			}
			for i, row := range rows {
				var got models.Jav
				if err := gdb.Preload("Idols").First(&got, row.ID).Error; err != nil {
					t.Fatal(err)
				}
				if task.filled(got) != (i != 0) {
					t.Fatalf("unexpected enrichment while AVSOX is blocked: %+v", got)
				}
			}
		})
	}
}
