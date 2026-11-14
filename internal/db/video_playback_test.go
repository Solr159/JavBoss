package db

import (
	"reflect"
	"testing"
	"time"

	"javboss/internal/models"

	"gorm.io/gorm"
)

func TestGetVideoForPlayback(t *testing.T) {
	deps := newTestDependencies()
	conn := openTestDB(deps, t)
	ctx := t.Context()
	info := models.MediaInfo{Version: models.MediaInfoVersion, Container: "mp4", VideoCodec: "h264", DurationSeconds: 12.25}
	video := models.Video{Fingerprint: "playback", Size: 1234, DurationSec: 12, MediaInfo: info}
	other := models.Video{Fingerprint: "other"}
	for _, v := range []*models.Video{&video, &other} {
		if err := conn.Create(v).Error; err != nil {
			t.Fatal(err)
		}
	}
	stamp := time.Unix(1700000000, 123456789).UTC()
	var dirs []models.Directory
	var locations []*models.VideoLocation
	for i, name := range []string{"deleted", "disabled", "first", "second"} {
		dir := models.Directory{Path: "/media/" + name}
		if err := conn.Create(&dir).Error; err != nil {
			t.Fatal(err)
		}
		loc, err := deps.store.UpsertVideoLocation(ctx, video.ID, dir.ID, name+".mp4", stamp.Add(time.Duration(i)*time.Second))
		if err != nil {
			t.Fatal(err)
		}
		dirs = append(dirs, dir)
		locations = append(locations, loc)
	}
	if err := conn.Model(&dirs[0]).UpdateColumn("is_delete", true).Error; err != nil {
		t.Fatal(err)
	}
	if err := conn.Model(&dirs[1]).UpdateColumn("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	otherLoc, err := deps.store.UpsertVideoLocation(ctx, other.ID, dirs[2].ID, "other.mp4", stamp)
	if err != nil {
		t.Fatal(err)
	}
	// Unrelated JSON must not be fetched or decoded on the playback path.
	if err := conn.Model(&video).UpdateColumn("subtitle_settings", "invalid json").Error; err != nil {
		t.Fatal(err)
	}
	if err := conn.Model(&dirs[2]).UpdateColumn("last_scan_summary", "invalid json").Error; err != nil {
		t.Fatal(err)
	}
	queries := 0
	if err := conn.Callback().Query().After("gorm:query").Register("test:count_playback_queries", func(*gorm.DB) { queries++ }); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		videoID    int64
		locationID int64
		want       int // Index in locations, or -1 for no match.
	}{
		{"primary skips inactive directories", video.ID, 0, 2},
		{"selected first", video.ID, locations[2].ID, 2},
		{"selected second", video.ID, locations[3].ID, 3},
		{"deleted directory", video.ID, locations[0].ID, -1},
		{"disabled directory", video.ID, locations[1].ID, -1},
		{"wrong video", other.ID, locations[2].ID, -1},
		{"another video's location", video.ID, otherLoc.ID, -1},
		{"missing video", other.ID + 100, 0, -1},
		{"missing location", video.ID, otherLoc.ID + 100, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			queries = 0
			got, err := deps.store.GetVideoForPlayback(ctx, tc.videoID, tc.locationID)
			if err != nil {
				t.Fatal(err)
			}
			if queries != 1 {
				t.Fatalf("queries = %d, want 1", queries)
			}
			if tc.want < 0 {
				if got != nil {
					t.Fatalf("unexpected video: %+v", got)
				}
				return
			}
			loc, dir := locations[tc.want], dirs[tc.want]
			want := &models.Video{
				ID: video.ID, LocationID: loc.ID, DirectoryID: dir.ID,
				Path: loc.RelativePath, Size: video.Size, DurationSec: video.DurationSec,
				ModifiedAt: loc.ModifiedAt, MediaInfo: info,
				DirectoryRef: models.Directory{ID: dir.ID, Path: dir.Path},
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %+v, want %+v", got, want)
			}
		})
	}
	// Re-read directory visibility on every request, without retaining stale state.
	if err := conn.Model(&dirs[2]).UpdateColumn("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	got, err := deps.store.GetVideoForPlayback(ctx, video.ID, 0)
	if err != nil || got == nil || got.LocationID != locations[3].ID {
		t.Fatalf("primary after disabling directory: %+v, %v", got, err)
	}
}
