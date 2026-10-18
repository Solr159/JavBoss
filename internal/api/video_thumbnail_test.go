package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"javboss/internal/common"
	"javboss/internal/db"
	"javboss/internal/models"
	"javboss/internal/video/screenshot"

	"github.com/gin-gonic/gin"
)

func TestThumbnailUsesBusinessPackageFunctions(t *testing.T) {
	dataDir := t.TempDir()
	database, err := db.Open(filepath.Join(dataDir, "test.db"))
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
	deps.config = &common.Config{DatabasePath: filepath.Join(dataDir, "test.db")}
	deps.bindDatabase(t)
	dir, err := db.Default().CreateDirectory(t.Context(), dataDir)
	if err != nil {
		t.Fatal(err)
	}
	video := &models.Video{ID: 1, DurationSec: 300, Fingerprint: "cached-thumbnail"}
	if err := db.Default().CreateVideo(t.Context(), video); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Default().UpsertVideoLocation(t.Context(), video.ID, dir.ID, "video.mp4", time.Now()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dataDir, "video", "1", "screenshot", "128.jpg")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("cached image"), 0600); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.GET("/videos/:id/thumbnail", getThumbnail)
	request := func() *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/videos/1/thumbnail", nil))
		return response
	}
	t.Run("unconfigured", func(t *testing.T) {
		screenshot.Start(t.Context(), "")
		if response := request(); response.Code != http.StatusServiceUnavailable {
			t.Fatalf("unconfigured screenshot response = %d", response.Code)
		}
	})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	screenshot.Start(ctx, dataDir)
	if response := request(); response.Code != http.StatusOK || response.Body.String() != "cached image" {
		t.Fatalf("shared screenshot response = %d %q", response.Code, response.Body.String())
	}
}

func TestGeneratedThumbnailReturnsImageInSameRequest(t *testing.T) {

	path := filepath.Join(t.TempDir(), "128.jpg")
	started, complete := make(chan struct{}), make(chan struct{})
	router := gin.New()
	router.GET("/thumbnail", func(c *gin.Context) {
		serveThumbnail(c, &models.Video{ID: 1}, func(ctx context.Context, video *models.Video) (string, error) {
			if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 20*time.Second {
				t.Error("thumbnail wait has no bounded deadline")
			}
			close(started)
			<-complete
			return path, nil
		})
	})
	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/thumbnail", nil))
		close(done)
	}()
	<-started
	select {
	case <-done:
		t.Fatal("request returned before generation completed")
	default:
	}
	if err := os.WriteFile(path, []byte("generated JPEG"), 0600); err != nil {
		t.Fatal(err)
	}
	close(complete)
	<-done
	if recorder.Code != http.StatusOK || recorder.Body.String() != "generated JPEG" || recorder.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("response = %d %s, headers=%v", recorder.Code, recorder.Body.String(), recorder.Header())
	}
}

func TestGeneratedThumbnailRetryResponses(t *testing.T) {

	for _, tc := range []struct {
		failure error
		status  int
		retry   string
	}{
		{context.DeadlineExceeded, http.StatusServiceUnavailable, "3"},
		{screenshot.ErrQueueFull, http.StatusServiceUnavailable, "3"},
		{screenshot.ErrUnavailable, http.StatusServiceUnavailable, "3"},
		{errors.New("capture failed"), http.StatusServiceUnavailable, "3"},
		{screenshot.ErrNoThumbnail, http.StatusNotFound, ""},
	} {
		t.Run(tc.failure.Error(), func(t *testing.T) {
			router := gin.New()
			router.GET("/thumbnail", func(c *gin.Context) {
				serveThumbnail(c, &models.Video{}, func(context.Context, *models.Video) (string, error) { return "", tc.failure })
			})
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/thumbnail", nil))
			if recorder.Code != tc.status || recorder.Header().Get("Retry-After") != tc.retry || recorder.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("retry response = %d, headers=%v", recorder.Code, recorder.Header())
			}
		})
	}
}
