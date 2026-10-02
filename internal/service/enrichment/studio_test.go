package enrichment

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"javboss/internal/db"
	"javboss/internal/jav"
	"javboss/internal/models"
)

func TestEnrichStudiosProviderOrder(t *testing.T) {
	uncensored, censored := true, false
	for _, tc := range []struct {
		name    string
		state   *bool
		studios map[jav.Provider]string
		errors  map[jav.Provider]error
		want    string
		retry   bool
	}{
		{
			name: "censored prefers javdatabase", state: &censored,
			studios: map[jav.Provider]string{jav.ProviderJavDatabase: " First Studio ", jav.ProviderJavDBAPI: "Second Studio"},
			want:    "First Studio",
		},
		{
			name: "censored falls back on whitespace", state: &censored,
			studios: map[jav.Provider]string{jav.ProviderJavDatabase: "  ", jav.ProviderJavDBAPI: "S1 NO.1 STYLE"},
			want:    "S1 NO.1 STYLE",
		},
		{
			name: "censored falls back on not found", state: &censored,
			errors:  map[jav.Provider]error{jav.ProviderJavDatabase: jav.ErrNotFound},
			studios: map[jav.Provider]string{jav.ProviderJavDBAPI: "日本語の片商"},
			want:    "日本語の片商", retry: true,
		},
		{
			name: "censored never falls back to javbus", state: &censored,
			studios: map[jav.Provider]string{jav.ProviderJavBus: "Unwanted Studio"},
		},
		{
			name:    "unknown state uses censored order",
			studios: map[jav.Provider]string{jav.ProviderJavDatabase: "English Studio"},
			want:    "English Studio",
		},
		{
			name: "uncensored prefers avsox", state: &uncensored,
			studios: map[jav.Provider]string{jav.ProviderAvsox: "AVSOX Studio", jav.ProviderJavDBAPI: "Second Studio", jav.ProviderJavBus: "Third Studio"},
			want:    "AVSOX Studio",
		},
		{
			name: "uncensored falls back on provider failure", state: &uncensored,
			errors:  map[jav.Provider]error{jav.ProviderAvsox: errors.New("provider unavailable")},
			studios: map[jav.Provider]string{jav.ProviderJavDBAPI: "API Studio"},
			want:    "API Studio",
		},
		{
			name: "uncensored javbus last resort", state: &uncensored,
			studios: map[jav.Provider]string{jav.ProviderAvsox: "", jav.ProviderJavBus: "JavBus Studio"},
			want:    "JavBus Studio",
		},
		{
			name: "uncensored all missing", state: &uncensored,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantCalls := []jav.Provider{jav.ProviderJavDatabase, jav.ProviderJavDBAPI}
			if tc.state != nil && *tc.state {
				wantCalls = []jav.Provider{jav.ProviderAvsox, jav.ProviderJavDBAPI, jav.ProviderJavBus}
			}
			run := enrichCensoredStudios
			if tc.state != nil && *tc.state {
				run = enrichUncensoredStudios
			}
			gdb := openEnrichmentTestDB(t)
			series := models.JavSeries{Name: "Existing Series"}
			if err := gdb.Create(&series).Error; err != nil {
				t.Fatal(err)
			}
			row := models.Jav{Code: "STUDIO-001", Title: "Original title", ZhTitle: "Original Chinese title", SeriesID: &series.ID, IsUncensored: tc.state}
			if err := gdb.Create(&row).Error; err != nil {
				t.Fatal(err)
			}
			var calls []jav.Provider
			var callsMu sync.Mutex
			lookup := func(_ context.Context, code string, provider jav.Provider) (*jav.JavInfo, error) {
				callsMu.Lock()
				calls = append(calls, provider)
				callsMu.Unlock()
				if code != row.Code {
					t.Errorf("lookup code=%q", code)
				}
				if err := tc.errors[provider]; err != nil {
					return nil, err
				}
				studio, ok := tc.studios[provider]
				if !ok {
					return nil, nil
				}
				return &jav.JavInfo{Studio: studio, Title: "Unwanted title", Series: "Unwanted Series", Actors: []string{"Unwanted Idol"}, IsUncensored: &uncensored}, nil
			}
			if err := run(context.Background(), lookup); err != nil {
				t.Fatal(err)
			}
			slices.Sort(calls)
			slices.Sort(wantCalls)
			if !reflect.DeepEqual(calls, wantCalls) {
				t.Fatalf("calls=%v, want %v", calls, wantCalls)
			}
			var got models.Jav
			if err := gdb.Preload("Studio").Preload("Series").Preload("Idols").First(&got, row.ID).Error; err != nil {
				t.Fatal(err)
			}
			if tc.want == "" {
				if got.StudioID != nil {
					t.Fatalf("unexpected studio: %+v", got.Studio)
				}
			} else if got.Studio == nil || got.Studio.Name != tc.want || got.Series.StudioID == nil || *got.Series.StudioID != *got.StudioID {
				t.Fatalf("studio not filled or series not linked: %+v", got)
			}
			if got.Title != row.Title || got.ZhTitle != row.ZhTitle || !reflect.DeepEqual(got.IsUncensored, row.IsUncensored) || len(got.Idols) != 0 || got.Series.Name != series.Name {
				t.Fatalf("unrelated metadata changed: %+v", got)
			}
			calls = nil
			if err := run(context.Background(), lookup); err != nil {
				t.Fatal(err)
			}
			if tc.want == "" || tc.retry {
				slices.Sort(calls)
				if !reflect.DeepEqual(calls, wantCalls) {
					t.Fatalf("missing or non-English studio not retried: calls=%v", calls)
				}
			} else if len(calls) != 0 {
				t.Fatalf("English studio scanned again: calls=%v", calls)
			}
		})
	}
}

func TestEnrichStudiosPreservesConcurrentManualEdit(t *testing.T) {
	gdb := openEnrichmentTestDB(t)
	row := models.Jav{Code: "STUDIO-001"}
	if err := gdb.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	err := enrichCensoredStudios(context.Background(), func(ctx context.Context, _ string, provider jav.Provider) (*jav.JavInfo, error) {
		if provider == jav.ProviderJavDatabase {
			if err := db.UpdateJavStudio(ctx, row.ID, "Manual Studio"); err != nil {
				t.Error(err)
				return nil, err
			}
		}
		return &jav.JavInfo{Studio: "Automatic Studio"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := gdb.Preload("Studio").First(&row, row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if row.Studio == nil || row.Studio.Name != "Manual Studio" {
		t.Fatalf("manual studio overwritten: %+v", row)
	}
}

func TestEnrichStudiosStopsOnCancellation(t *testing.T) {
	gdb := openEnrichmentTestDB(t)
	row := models.Jav{Code: "STUDIO-001"}
	if err := gdb.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	err := enrichCensoredStudios(ctx, func(context.Context, string, jav.Provider) (*jav.JavInfo, error) {
		calls.Add(1)
		cancel()
		return &jav.JavInfo{Studio: "Canceled Studio"}, nil
	})
	if !errors.Is(err, context.Canceled) || calls.Load() != 2 {
		t.Fatalf("cancellation: err=%v calls=%d", err, calls.Load())
	}
	if err := gdb.First(&row, row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if row.StudioID != nil {
		t.Fatal("canceled lookup filled studio")
	}
}

func TestEnrichStudiosQueriesProvidersConcurrentlyAndMergesNames(t *testing.T) {
	gdb := openEnrichmentTestDB(t)
	studio := models.JavStudio{Name: "元の片商"}
	if err := gdb.Create(&studio).Error; err != nil {
		t.Fatal(err)
	}
	uncensored := true
	row := models.Jav{Code: "STUDIO-001", StudioID: &studio.ID, IsUncensored: &uncensored}
	if err := gdb.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var started atomic.Int32
	ready := make(chan struct{})
	names := map[jav.Provider]string{
		jav.ProviderAvsox:    "元の片商",
		jav.ProviderJavDBAPI: "English Studio",
		jav.ProviderJavBus:   "Another English Name",
	}
	err := enrichUncensoredStudios(ctx, func(ctx context.Context, _ string, provider jav.Provider) (*jav.JavInfo, error) {
		if started.Add(1) == 3 {
			close(ready)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ready:
			return &jav.JavInfo{Studio: names[provider]}, nil
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := gdb.Preload("Studio").First(&row, row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if row.Studio.Name != "English Studio" || *row.StudioID != studio.ID {
		t.Fatalf("unexpected canonical studio: %+v", row.Studio)
	}
	var aliases []string
	if err := gdb.Model(&models.JavStudioAlias{}).Where("jav_studio_id = ?", studio.ID).Pluck("alias", &aliases).Error; err != nil {
		t.Fatal(err)
	}
	sort.Strings(aliases)
	if !reflect.DeepEqual(aliases, []string{"Another English Name", "元の片商"}) {
		t.Fatalf("aliases=%v", aliases)
	}
}
