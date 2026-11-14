package db

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"

	"javboss/internal/models"
)

func TestSubtitleSettingsJSONPartialUpdates(t *testing.T) {
	conn, err := Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := conn.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(1)
	store := NewStore(conn)
	dir := models.Directory{Path: t.TempDir()}
	video := models.Video{Fingerprint: "subtitle-settings"}
	for _, row := range []any{&dir, &video} {
		if err := conn.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	location := models.VideoLocation{VideoID: video.ID, DirectoryID: dir.ID, RelativePath: "video.mp4", Filename: "video.mp4"}
	if err := conn.Create(&location).Error; err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	// Independent UI/MPV writes must merge rather than replace the JSON object.
	var workers sync.WaitGroup
	start := make(chan struct{})
	for _, offsetWriter := range []bool{true, false} {
		workers.Add(1)
		go func(offsetWriter bool) {
			defer workers.Done()
			<-start
			for range 20 {
				var offset *int64
				var track *string
				if offsetWriter {
					value := int64(-1500)
					offset = &value
				} else {
					value := "off"
					track = &value
				}
				if found, err := store.UpdateVideoSubtitleSettings(ctx, video.ID, offset, track); err != nil || !found {
					t.Errorf("save settings: found=%v err=%v", found, err)
				}
			}
		}(offsetWriter)
	}
	close(start)
	workers.Wait()
	if err := store.SaveVideo(ctx, &video); err != nil {
		t.Fatal(err)
	}
	var got models.Video
	if err := conn.First(&got, video.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.SubtitleSettings.OffsetMS != -1500 || got.SubtitleSettings.Track != "off" {
		t.Fatalf("settings lost: %+v", got.SubtitleSettings)
	}
	selected := "local:中文字幕.srt"
	if found, err := store.UpdateVideoSubtitleSettings(ctx, video.ID, nil, &selected); err != nil || !found {
		t.Fatal(found, err)
	}
	var raw string
	if err := conn.Raw("SELECT subtitle_settings FROM video WHERE id = ?", video.ID).Scan(&raw).Error; err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 2 || fields["offset_ms"] != float64(-1500) || fields["track"] != selected {
		t.Fatalf("unexpected JSON: %s", raw)
	}
}
