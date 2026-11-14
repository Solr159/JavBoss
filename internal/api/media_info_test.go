package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"javboss/internal/db"
	"javboss/internal/models"
	"javboss/internal/video/stream"
)

func TestPlaybackInfoAndHLSUsePersistedMediaInfoForSelectedLocation(t *testing.T) {
	deps := newTestDependencies()
	conn, err := db.Open(filepath.Join(t.TempDir(), "media.db"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := conn.DB()
	defer sqlDB.Close()
	deps.database = conn
	deps.bindDatabase(t)
	dir := models.Directory{Path: t.TempDir()}
	if err := conn.Create(&dir).Error; err != nil {
		t.Fatal(err)
	}
	video := models.Video{Fingerprint: "persisted", Size: 5, DurationSec: 12, MediaInfo: models.MediaInfo{Version: models.MediaInfoVersion, Container: "mp4", VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080, DurationSeconds: 12.25}}
	if err := conn.Create(&video).Error; err != nil {
		t.Fatal(err)
	}
	var locationIDs []int64
	contents := []string{"first", "other"}
	for i, name := range []string{"first.mp4", "second.mp4"} {
		path := filepath.Join(dir.Path, name)
		// These files cannot be probed as video. Success demonstrates the database path.
		if err := os.WriteFile(path, []byte(contents[i]), 0600); err != nil {
			t.Fatal(err)
		}
		stamp := time.Unix(1700000000+int64(i), 0)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
		loc, err := db.Default().UpsertVideoLocation(t.Context(), video.ID, dir.ID, name, stamp)
		if err != nil {
			t.Fatal(err)
		}
		locationIDs = append(locationIDs, loc.ID)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/videos/:id/streams", getVideoStreams)
	router.GET("/videos/:id/stream", streamVideo)
	router.GET("/videos/:id/stream.m3u8", streamHLSManifest)
	router.GET("/videos/:id/stream.m3u8/:segment", streamHLSSegment)
	cacheDir := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stream.Start(ctx, cacheDir)
	queries := 0
	if err := conn.Callback().Query().After("gorm:query").Register("test:count_playback_queries", func(*gorm.DB) { queries++ }); err != nil {
		t.Fatal(err)
	}
	for index, locationID := range append([]int64{0}, locationIDs...) {
		segmentDir := filepath.Join(cacheDir, stream.StreamTypeHLS.FileDir(streamCacheKey(video.ID, locationID), 0))
		if err := os.MkdirAll(segmentDir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(segmentDir, "0.ts"), []byte("segment"), 0600); err != nil {
			t.Fatal(err)
		}
		for _, suffix := range []string{"streams", "stream", "stream.m3u8", "stream.m3u8/0.ts"} {
			url := "/videos/" + strconv.FormatInt(video.ID, 10) + "/" + suffix
			if locationID > 0 {
				url += "?location_id=" + strconv.FormatInt(locationID, 10)
			}
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, url, nil)
			wantStatus := http.StatusOK
			if suffix == "stream" {
				request.Header.Set("Range", "bytes=1-3")
				wantStatus = http.StatusPartialContent
			}
			queries = 0
			router.ServeHTTP(response, request)
			if queries != 1 {
				t.Fatalf("%s: %d queries, want 1", url, queries)
			}
			if response.Code != wantStatus {
				t.Fatalf("%s: %d %s", url, response.Code, response.Body.String())
			}
			expected := `"video_codec":"h264"`
			switch suffix {
			case "stream":
				expected = contents[max(0, index-1)][1:4]
			case "stream.m3u8":
				expected = "#EXTINF:0.250000,"
			case "stream.m3u8/0.ts":
				expected = "segment"
			}
			if !strings.Contains(response.Body.String(), expected) {
				t.Fatalf("%s missing %q: %s", url, expected, response.Body.String())
			}
		}
	}
}
