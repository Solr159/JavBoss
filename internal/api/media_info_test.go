package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"javboss/internal/db"
	"javboss/internal/models"
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
	for i, name := range []string{"first.mp4", "second.mp4"} {
		path := filepath.Join(dir.Path, name)
		// These files cannot be probed as video. Success demonstrates the database path.
		if err := os.WriteFile(path, []byte("media"), 0600); err != nil {
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
	router.GET("/videos/:id/stream.m3u8", streamHLSManifest)
	for _, locationID := range append([]int64{0}, locationIDs...) {
		for _, suffix := range []string{"streams", "stream.m3u8"} {
			url := "/videos/" + strconv.FormatInt(video.ID, 10) + "/" + suffix
			if locationID > 0 {
				url += "?location_id=" + strconv.FormatInt(locationID, 10)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, url, nil))
			if response.Code != http.StatusOK {
				t.Fatalf("%s: %d %s", url, response.Code, response.Body.String())
			}
			expected := `"video_codec":"h264"`
			if suffix == "stream.m3u8" {
				expected = "#EXTINF:0.250000,"
			}
			if !strings.Contains(response.Body.String(), expected) {
				t.Fatalf("%s missing %q: %s", url, expected, response.Body.String())
			}
		}
	}
}
