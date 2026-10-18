package api

import (
	"context"
	"encoding/json"
	"errors"
	"javboss/internal/common"
	"javboss/internal/db"
	"javboss/internal/jav/translation"
	"javboss/internal/models"
	"javboss/internal/playback"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRoutersUseSharedBusinessStateWithoutServerInstance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	database, err := db.Open(filepath.Join(root, "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	connection, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	deps := newTestDependencies()
	deps.database = database
	deps.config = &common.Config{DatabasePath: filepath.Join(root, "library.db"), JavCoverDir: root}
	deps.bindDatabase(t)
	dir, err := db.Default().CreateDirectory(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	video := models.Video{Fingerprint: "shared", Filename: "shared.mp4"}
	if err := db.Default().CreateVideo(t.Context(), &video); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Default().UpsertVideoLocation(t.Context(), video.ID, dir.ID, "shared.mp4", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&models.Jav{Code: "TEST-001", Title: "shared"}).Error; err != nil {
		t.Fatal(err)
	}
	cover := strings.Repeat("cover", 2048)
	if err := os.WriteFile(filepath.Join(root, "test-001.jpg"), []byte(cover), 0600); err != nil {
		t.Fatal(err)
	}
	routers := []*gin.Engine{gin.New(), gin.New()}
	for _, router := range routers {
		RegisterRoutes(router)
	}
	for _, router := range routers {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/videos", nil))
		var result struct {
			Items []models.Video `json:"items"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != 200 || len(result.Items) != 1 {
			t.Fatalf("videos: %d %s %v", response.Code, response.Body, err)
		}
		response = httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/jav/test-001/cover", nil))
		if response.Code != 200 || response.Body.String() != cover {
			t.Fatalf("cover: %d", response.Code)
		}
	}
	updates, unsubscribe := playback.SubscribeWatchedTime()
	defer unsubscribe()
	response := httptest.NewRecorder()
	routers[0].ServeHTTP(response, httptest.NewRequest("POST", "/videos/1/playback-sessions", strings.NewReader(`{}`)))
	var session struct {
		ID string `json:"session_id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &session); err != nil || response.Code != 201 {
		t.Fatalf("create session: %d %s", response.Code, response.Body)
	}
	response = httptest.NewRecorder()
	routers[1].ServeHTTP(response, httptest.NewRequest("PUT", "/videos/1/playback-sessions/"+session.ID, strings.NewReader(`{"watched_ms":1000}`)))
	if response.Code != 204 {
		t.Fatalf("shared session: %d %s", response.Code, response.Body)
	}
	select {
	case update := <-updates:
		if len(update.Videos) != 1 || update.Videos[0].WatchedMS != 1000 {
			t.Fatalf("event: %+v", update)
		}
	default:
		t.Fatal("missing committed playback event")
	}
	release, err := translation.Acquire(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if another, err := translation.Acquire(ctx, 1); another != nil || !errors.Is(err, context.DeadlineExceeded) {
		if another != nil {
			another()
		}
		t.Fatalf("translation lock was not shared: %v", err)
	}
}
