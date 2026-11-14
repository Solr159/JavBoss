package metadata

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm"

	"javboss/internal/db"
	"javboss/internal/models"
	"javboss/internal/util"
)

func mediaFixture(t *testing.T) (*db.Store, *gorm.DB, *models.Video, string) {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "media.db"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := conn.DB()
	t.Cleanup(func() { sqlDB.Close() })
	store := db.NewStore(conn)
	dir := models.Directory{Path: t.TempDir()}
	if err := conn.Create(&dir).Error; err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir.Path, "video.mp4")
	if err := os.WriteFile(path, []byte("media"), 0600); err != nil {
		t.Fatal(err)
	}
	stat, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	video := models.Video{Fingerprint: "legacy", Size: stat.Size(), DurationSec: 13}
	if err := store.CreateVideo(t.Context(), &video); err != nil {
		t.Fatal(err)
	}
	loc, err := store.UpsertVideoLocation(t.Context(), video.ID, dir.ID, "video.mp4", stat.ModTime())
	if err != nil {
		t.Fatal(err)
	}
	selected, err := store.GetVideoForPlayback(t.Context(), video.ID, loc.ID)
	if err != nil {
		t.Fatal(err)
	}
	return store, conn, selected, path
}

func testInfo() *util.VideoMetadata {
	return &util.VideoMetadata{Version: models.MediaInfoVersion, Codec: "hevc", VideoCodec: "hevc", Container: "mp4", Width: 1920, Height: 1080, DurationSeconds: 12.75, FPS: 23.976}
}

func TestFirstPlaybackBackfillsOnceAcrossConcurrentRequests(t *testing.T) {
	store, conn, video, path := mediaFixture(t)
	var calls atomic.Int32
	probe := func(context.Context, string) (*util.VideoMetadata, error) { calls.Add(1); return testInfo(), nil }
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			info, err := load(t.Context(), store, video, path, probe)
			if err != nil || !reflect.DeepEqual(info, testInfo()) {
				t.Errorf("media: %+v %v", info, err)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("ran %d probes", calls.Load())
	}
	var saved models.Video
	if err := conn.First(&saved, video.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved.MediaInfo, *testInfo()) {
		t.Fatalf("saved: %+v", saved.MediaInfo)
	}
	selected, err := store.GetVideoForLocation(t.Context(), video.ID, video.LocationID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = load(t.Context(), store, selected, path, func(context.Context, string) (*util.VideoMetadata, error) {
		t.Error("populated video reprobed")
		return nil, errors.New("unexpected probe")
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestChangedFilesDoNotOverwriteSharedVideoInfo(t *testing.T) {
	for _, change := range []string{"size", "mtime"} {
		t.Run(change, func(t *testing.T) {
			store, conn, video, path := mediaFixture(t)
			video.MediaInfo = *testInfo()
			if _, err := store.FillVideoMediaInfo(t.Context(), video.ID, video.MediaInfo); err != nil {
				t.Fatal(err)
			}
			// Another copy retains its scan timestamp, independently of this location.
			other := filepath.Join(filepath.Dir(path), "other.mp4")
			if err := os.WriteFile(other, []byte("media"), 0600); err != nil {
				t.Fatal(err)
			}
			otherStat, _ := os.Stat(other)
			otherLoc, err := store.UpsertVideoLocation(t.Context(), video.ID, video.DirectoryID, "other.mp4", otherStat.ModTime())
			if err != nil {
				t.Fatal(err)
			}
			if change == "size" {
				if err := os.WriteFile(path, []byte("new larger media"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				stamp := video.ModifiedAt.Add(time.Second)
				if err := os.Chtimes(path, stamp, stamp); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			changed := *testInfo()
			changed.VideoCodec = "av1"
			info, err := load(t.Context(), store, video, path, func(context.Context, string) (*util.VideoMetadata, error) { calls++; return &changed, nil })
			if err != nil || calls != 1 || info.VideoCodec != "av1" {
				t.Fatalf("changed file: %+v calls=%d err=%v", info, calls, err)
			}
			otherVideo, err := store.GetVideoForLocation(t.Context(), video.ID, otherLoc.ID)
			if err != nil {
				t.Fatal(err)
			}
			otherInfo, err := load(t.Context(), store, otherVideo, other, func(context.Context, string) (*util.VideoMetadata, error) {
				t.Error("unchanged copy reprobed")
				return nil, errors.New("unexpected probe")
			})
			if err != nil || otherInfo.VideoCodec != "hevc" {
				t.Fatalf("other copy: %+v %v", otherInfo, err)
			}
			var persisted models.Video
			if err := conn.First(&persisted, video.ID).Error; err != nil {
				t.Fatal(err)
			}
			if persisted.MediaInfo.VideoCodec != "hevc" {
				t.Fatal("changed copy overwrote shared metadata")
			}
		})
	}
}

func TestFailedOrInterruptedProbeDoesNotPopulateMediaInfo(t *testing.T) {
	for _, reason := range []string{"failed", "cancelled", "changed during probe", "deleted"} {
		t.Run(reason, func(t *testing.T) {
			store, conn, video, path := mediaFixture(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if reason == "deleted" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			_, err := load(ctx, store, video, path, func(context.Context, string) (*util.VideoMetadata, error) {
				switch reason {
				case "failed":
					return nil, errors.New("probe failed")
				case "cancelled":
					cancel()
				case "changed during probe":
					if err := os.WriteFile(path, []byte("replaced media"), 0600); err != nil {
						return nil, err
					}
				case "deleted":
					t.Error("missing file reached probe")
				}
				return testInfo(), nil
			})
			if err == nil {
				t.Fatal("expected failure")
			}
			var persisted models.Video
			if err := conn.First(&persisted, video.ID).Error; err != nil {
				t.Fatal(err)
			}
			if persisted.MediaInfo.Complete() {
				t.Fatal("failed probe marked complete")
			}
		})
	}
}
