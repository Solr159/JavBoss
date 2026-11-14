package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"javboss/internal/db"
	"javboss/internal/models"
	"javboss/internal/video/preview"
	"javboss/internal/video/stream"
)

func TestChangedVideoRejectsPlaybackAndCachedPreview(t *testing.T) {
	root := t.TempDir()
	conn, err := db.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := conn.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	deps := newTestDependencies()
	deps.database = conn
	deps.bindDatabase(t)
	dir := models.Directory{Path: root}
	video := models.Video{Fingerprint: "file-check", Size: 5, DurationSec: 12, MediaInfo: models.MediaInfo{
		Version: models.MediaInfoVersion, Container: "mp4", VideoCodec: "h264", Width: 640, Height: 360, DurationSeconds: 12,
	}}
	for _, row := range []any{&dir, &video} {
		if err := conn.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	stamp := time.Unix(1700000000, 123456789)
	var locations []*models.VideoLocation
	var paths []string
	for _, name := range []string{"first.mp4", "copy.mp4"} {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte("media"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
		stat, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		loc, err := db.Default().UpsertVideoLocation(t.Context(), video.ID, dir.ID, name, stat.ModTime())
		if err != nil {
			t.Fatal(err)
		}
		locations = append(locations, loc)
		paths = append(paths, path)
	}
	previewDir := filepath.Join(root, "previews")
	if err := os.MkdirAll(previewDir, 0700); err != nil {
		t.Fatal(err)
	}
	previewPath := filepath.Join(previewDir, fmt.Sprintf("%d_0.jpg", video.ID))
	if err := os.WriteFile(previewPath, []byte("cached preview"), 0600); err != nil {
		t.Fatal(err)
	}
	preview.Start(t.Context(), previewDir)
	streamDir := filepath.Join(root, "streams")
	stream.Start(t.Context(), streamDir)
	segmentDir := filepath.Join(streamDir, stream.StreamTypeHLS.FileDir(streamCacheKey(video.ID, locations[0].ID), 0))
	if err := os.MkdirAll(segmentDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(segmentDir, "0.ts"), []byte("old segment"), 0600); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.GET("/videos/:id/streams", getVideoStreams)
	router.GET("/videos/:id/stream", streamVideo)
	router.GET("/videos/:id/stream.m3u8", streamHLSManifest)
	router.GET("/videos/:id/stream.m3u8/:segment", streamHLSSegment)
	router.GET("/videos/:id/preview", getVideoPreview)
	url := func(suffix string, locationID int64) string {
		return fmt.Sprintf("/videos/%d/%s?location_id=%d&second=0", video.ID, suffix, locationID)
	}
	request := func(path string, status int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s: got %d %s, want %d", path, w.Code, w.Body.String(), status)
		}
		if status == http.StatusConflict && !strings.Contains(w.Body.String(), "Rescan its directory") {
			t.Fatalf("missing rescan guidance: %s", w.Body.String())
		}
		return w
	}
	var info playbackInfo
	if err := json.Unmarshal(request(url("streams", locations[0].ID), http.StatusOK).Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	directURL := info.Sources[0].Src
	request(directURL, http.StatusOK)
	request(url("preview", locations[0].ID), http.StatusOK)
	request(url("stream.m3u8/0.ts", locations[0].ID), http.StatusOK)
	for _, change := range []string{"size", "mtime"} {
		t.Run(change, func(t *testing.T) {
			if change == "size" {
				if err := os.WriteFile(paths[0], []byte("larger media"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(paths[0], stamp, stamp); err != nil {
					t.Fatal(err)
				}
			} else {
				changed := stamp.Add(time.Second)
				if err := os.Chtimes(paths[0], changed, changed); err != nil {
					t.Fatal(err)
				}
			}
			for _, suffix := range []string{"streams", "stream", "stream.m3u8", "stream.m3u8/0.ts", "preview"} {
				request(url(suffix, locations[0].ID), http.StatusConflict)
			}
			// Requests without a location validate the default copy.
			request(fmt.Sprintf("/videos/%d/stream", video.ID), http.StatusConflict)
			request(url("streams", locations[1].ID), http.StatusOK)
			request(url("preview", locations[1].ID), http.StatusOK)
			if err := os.WriteFile(paths[0], []byte("media"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(paths[0], stamp, stamp); err != nil {
				t.Fatal(err)
			}
			request(directURL, http.StatusOK)
		})
	}
}
