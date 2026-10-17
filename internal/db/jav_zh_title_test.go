package db

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"javboss/internal/jav/metadata"
	"javboss/internal/models"
)

func TestSaveJavInfoIgnoresScrapedChineseTitles(t *testing.T) {
	for _, provider := range []metadata.Provider{metadata.ProviderJavDBAPI, metadata.ProviderJavDB, metadata.ProviderJavBus} {
		t.Run(provider.String(), func(t *testing.T) {
			gdb := openTestDB(t)
			ctx := context.Background()
			info := &metadata.JavInfo{Code: "ABC-001", Title: "日本語の原題", ZhTitle: "站点中文译名", Provider: provider}
			item, err := SaveJavInfo(ctx, info)
			if err != nil {
				t.Fatal(err)
			}
			if item.ZhTitle != "" {
				t.Fatalf("scraped Chinese title persisted: %q", item.ZhTitle)
			}
			if saved, err := SaveJavTitleTranslation(ctx, item, "AI 中文标题"); err != nil || !saved {
				t.Fatalf("save translation: saved=%v err=%v", saved, err)
			}
			manual := "人工校对的中文标题"
			for _, want := range []string{"AI 中文标题", manual} {
				if want == manual {
					if _, err := UpdateJav(ctx, item.ID, JavUpdateInput{ZhTitle: &manual}, nil); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := SaveJavInfo(ctx, info); err != nil {
					t.Fatal(err)
				}
				var stored models.Jav
				if err := gdb.First(&stored, item.ID).Error; err != nil {
					t.Fatal(err)
				}
				if stored.Title != info.Title || stored.ZhTitle != want {
					t.Fatalf("scrape replaced existing titles: %q, %q", stored.Title, stored.ZhTitle)
				}
				payload, err := json.Marshal(stored)
				if err != nil {
					t.Fatal(err)
				}
				var fields map[string]any
				if err := json.Unmarshal(payload, &fields); err != nil {
					t.Fatal(err)
				}
				if fields["title"] != stored.Title || fields["zh_title"] != want {
					t.Fatalf("API title fields: %s", payload)
				}
			}
			info.Title = "更新した原題"
			updated, err := SaveJavInfo(ctx, info)
			if err != nil {
				t.Fatal(err)
			}
			if updated.Title != info.Title || updated.ZhTitle != "" {
				t.Fatalf("source change retained stale translation: %+v", updated)
			}
		})
	}
}

func TestSearchJavMatchesChineseTitlesAndKeepsFilters(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	dir := models.Directory{Path: "/media/title-search"}
	video := models.Video{Fingerprint: "title-search"}
	item := models.Jav{Code: "ABC-001", Title: "Rainy day", ZhTitle: "雨天相遇"}
	unlinked := models.Jav{Code: "ABC-002", Title: "Unlinked", ZhTitle: item.ZhTitle}
	for _, record := range []any{&dir, &video, &item, &unlinked} {
		if err := gdb.Create(record).Error; err != nil {
			t.Fatal(err)
		}
	}
	location, err := UpsertVideoLocation(ctx, video.ID, dir.ID, "title.mp4", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := gdb.Model(location).Update("jav_id", item.ID).Error; err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		search string
		dirs   []int64
		want   int64
	}{
		{"ABC-001", nil, 1},
		{"Rainy", nil, 1},
		{"雨天", nil, 1},
		{"雨天", []int64{dir.ID + 1}, 0},
		{"missing", nil, 0},
	} {
		items, total, err := SearchJav(ctx, nil, nil, tc.search, "recent", 25, 0, nil, tc.dirs)
		if err != nil {
			t.Fatal(err)
		}
		if total != tc.want || int64(len(items)) != tc.want {
			t.Fatalf("search=%q dirs=%v: total=%d items=%d want=%d", tc.search, tc.dirs, total, len(items), tc.want)
		}
	}
}
