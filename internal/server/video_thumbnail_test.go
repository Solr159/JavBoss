package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"javboss/internal/manager"
	"javboss/internal/models"
)

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
		{manager.ErrScreenshotQueueFull, http.StatusServiceUnavailable, "3"},
		{manager.ErrScreenshotUnavailable, http.StatusServiceUnavailable, "3"},
		{errors.New("capture failed"), http.StatusServiceUnavailable, "3"},
		{manager.ErrNoThumbnail, http.StatusNotFound, ""},
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
