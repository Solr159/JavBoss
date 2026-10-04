package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"javboss/internal/common"
	dbpkg "javboss/internal/db"
	"javboss/internal/models"
	"javboss/internal/playback"
)

func TestPlaybackSessionAttributesActualLocationAndRetainsHistory(t *testing.T) {
	database, err := dbpkg.Open(filepath.Join(t.TempDir(), "watch.db"))
	if err != nil {
		t.Fatal(err)
	}
	oldDB, oldSessions := common.DB, playbackSessions
	common.DB, playbackSessions = database, playback.NewSessions(dbpkg.AddWatchedTime)
	t.Cleanup(func() { common.DB, playbackSessions = oldDB, oldSessions; sqlDB, _ := database.DB(); _ = sqlDB.Close() })
	video := models.Video{Fingerprint: "same-content"}
	dir := models.Directory{Path: "/media"}
	a, b := models.Jav{Code: "WATCH-A"}, models.Jav{Code: "WATCH-B"}
	for _, row := range []any{&video, &dir, &a, &b} {
		if err := database.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	first := models.VideoLocation{VideoID: video.ID, DirectoryID: dir.ID, RelativePath: "a.mp4", JavID: &a.ID}
	bID := b.ID
	second := models.VideoLocation{VideoID: video.ID, DirectoryID: dir.ID, RelativePath: "b.mp4", JavID: &bID}
	for _, row := range []any{&first, &second} {
		if err := database.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	router := gin.New()
	router.POST("/videos/:id/playback-sessions", createPlaybackSession)
	router.PUT("/videos/:id/playback-sessions/:session_id", reportPlaybackSession)
	call := func(method, path, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		return w
	}
	base := fmt.Sprintf("/videos/%d/playback-sessions", video.ID)
	response := call("POST", base, fmt.Sprintf(`{"location_id":%d}`, second.ID), 201)
	var session struct {
		ID string `json:"session_id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	// Relinking the location does not move the already-open session to A.
	if err := database.Model(&second).Update("jav_id", a.ID).Error; err != nil {
		t.Fatal(err)
	}
	for _, value := range []int{1000, 1000, 500, 2000} {
		call("PUT", base+"/"+session.ID, fmt.Sprintf(`{"watched_ms":%d}`, value), 204)
	}
	for _, row := range []any{&video, &a, &b} {
		if err := database.First(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if video.WatchedMS != 2000 || a.WatchedMS != 0 || b.WatchedMS != 2000 {
		t.Fatalf("totals video=%d A=%d B=%d", video.WatchedMS, a.WatchedMS, b.WatchedMS)
	}
	call("PUT", base+"/"+session.ID, `{"watched_ms":-1}`, 400)
	call("PUT", base+"/"+session.ID, `{}`, 400)
	call("POST", base, `{"location_id":9999}`, 404)
	if err := database.Delete(&video).Error; err != nil {
		t.Fatal(err)
	}
	call("PUT", base+"/"+session.ID, `{"watched_ms":3000}`, 204)
	if err := database.First(&b, b.ID).Error; err != nil {
		t.Fatal(err)
	}
	if b.WatchedMS != 3000 {
		t.Fatal("deletion lost JAV history")
	}
	playbackSessions = playback.NewSessions(dbpkg.AddWatchedTime)
	call("PUT", base+"/"+session.ID, `{"watched_ms":3000}`, 410)
}
