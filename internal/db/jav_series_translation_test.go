package db

import (
	"context"
	"testing"

	"javboss/internal/models"
)

func TestSeriesTranslationAppearsInSummariesAndSearch(t *testing.T) {
	deps := newTestDependencies()
	database := openTestDB(deps, t)
	create := func(value any) {
		t.Helper()
		if err := database.Create(value).Error; err != nil {
			t.Fatal(err)
		}
	}
	studio := models.JavStudio{Name: "Studio"}
	create(&studio)
	series := models.JavSeries{Name: "雨の日", ZhName: "雨天系列", StudioID: &studio.ID}
	create(&series)
	work := models.Jav{Code: "SERIES-001", SeriesID: &series.ID, StudioID: &studio.ID}
	create(&work)
	directory := models.Directory{Path: t.TempDir()}
	create(&directory)
	video := models.Video{Fingerprint: "series-translation"}
	create(&video)
	create(&models.VideoLocation{VideoID: video.ID, DirectoryID: directory.ID, RelativePath: "series.mp4", Filename: "series.mp4", JavID: &work.ID})
	ctx := context.Background()
	for _, search := range []string{series.Name, series.ZhName} {
		items, total, err := deps.store.ListJavSeries(ctx, search, 10, 0, nil)
		if err != nil || total != 1 || len(items) != 1 || items[0].Name != series.Name || items[0].ZhName != series.ZhName {
			t.Fatalf("search %q: items=%+v total=%d err=%v", search, items, total, err)
		}
	}
	summary, err := deps.store.GetJavSeriesSummary(ctx, series.ID, nil)
	if err != nil || summary.ZhName != series.ZhName {
		t.Fatalf("summary=%+v err=%v", summary, err)
	}
	studios, _, err := deps.store.ListJavStudios(ctx, "", 10, 0, nil)
	if err != nil || len(studios) != 1 || len(studios[0].Series) != 1 || studios[0].Series[0].ZhName != series.ZhName {
		t.Fatalf("studio series=%+v err=%v", studios, err)
	}
}
