package db

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"javboss/internal/models"
)

func TestVideoMediaInfoPersistsWithoutOverwritingOtherFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "media.db")
	conn, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := conn.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	store := NewStore(conn)
	video := models.Video{Fingerprint: "media", DurationSec: 13, WatchedMS: 4000, PlayCount: 2, SubtitleSettings: models.SubtitleSettings{Track: "off"}}
	if err := store.CreateVideo(t.Context(), &video); err != nil {
		t.Fatal(err)
	}
	var raw string
	if err := conn.Raw("SELECT media_info FROM video WHERE id = ?", video.ID).Scan(&raw).Error; err != nil {
		t.Fatal(err)
	}
	var old models.MediaInfo
	if err := json.Unmarshal([]byte(raw), &old); err != nil || old.Complete() {
		t.Fatalf("legacy state: %s %v", raw, err)
	}
	info := models.MediaInfo{Version: models.MediaInfoVersion, VideoCodec: "hevc", Container: "mp4", Width: 1920, Height: 1080, DurationSeconds: 12.75}
	if saved, err := store.FillVideoMediaInfo(t.Context(), video.ID, info); err != nil || !saved {
		t.Fatal(saved, err)
	}
	changed := info
	changed.VideoCodec = "h264"
	if saved, err := store.FillVideoMediaInfo(t.Context(), video.ID, changed); err != nil || saved {
		t.Fatalf("overwrote established data: %v %v", saved, err)
	}
	if err := store.SaveVideo(t.Context(), &video); err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	reopenedSQL, _ := reopened.DB()
	defer reopenedSQL.Close()
	var got models.Video
	if err := reopened.First(&got, video.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.MediaInfo, info) || got.WatchedMS != 4000 || got.PlayCount != 2 || got.SubtitleSettings.Track != "off" || got.DurationSec != 13 {
		t.Fatalf("persisted video: %+v", got)
	}
	// Unrelated API listings must not expand every row with the internal probe payload.
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	if _, present := payload["media_info"]; present {
		t.Fatal("internal media info leaked into list payload")
	}
}
