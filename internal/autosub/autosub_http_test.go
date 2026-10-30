package autosub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"javboss/internal/models"
)

func TestSweepHandlerRunsAndReportsLastRun(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "ABC-129.mp4"), "video")

	service := NewAutoService(&fakeGateway{result: DownloadResult{Success: true}}, &fakeLocations{
		locations: []models.VideoLocation{
			{ID: 1, Filename: "ABC-129.mp4", RelativePath: "ABC-129.mp4", DirectoryRef: models.Directory{Path: dir}},
		},
	}, fakeConfig{values: map[string]string{AutoEnabledKey: "false"}})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewSweepHandler(func() *AutoService { return service })
	router.GET("/autosub/auto/status", handler.ServeSweepStatus)
	router.POST("/autosub/auto/run", handler.ServeSweepRun)

	serve := func(method, path string, wantStatus int, out any) {
		t.Helper()
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(method, path, nil))
		if response.Code != wantStatus {
			t.Fatalf("%s %s status=%d want=%d body=%s", method, path, response.Code, wantStatus, response.Body)
		}
		if out != nil {
			if err := json.Unmarshal(response.Body.Bytes(), out); err != nil {
				t.Fatalf("decode %s %s: %v", method, path, err)
			}
		}
	}

	var initial SweepStatusResponse
	serve(http.MethodGet, "/autosub/auto/status", http.StatusOK, &initial)
	if !initial.Available || initial.Running || initial.LastRun != nil {
		t.Fatalf("unexpected initial status: %+v", initial)
	}

	var started SweepRunResponse
	serve(http.MethodPost, "/autosub/auto/run", http.StatusAccepted, &started)
	if !started.Available || !started.Started {
		t.Fatalf("unexpected run response: %+v", started)
	}

	deadline := time.Now().Add(5 * time.Second)
	for service.Status().Running {
		if time.Now().After(deadline) {
			t.Fatal("sweep did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}

	var finished SweepStatusResponse
	serve(http.MethodGet, "/autosub/auto/status", http.StatusOK, &finished)
	if finished.LastRun == nil || finished.LastRun.Downloaded != 1 {
		t.Fatalf("unexpected last run: %+v", finished.LastRun)
	}
}

func TestSweepHandlerReportsUnavailableSweep(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewSweepHandler(func() *AutoService { return nil })
	router.GET("/autosub/auto/status", handler.ServeSweepStatus)
	router.POST("/autosub/auto/run", handler.ServeSweepRun)

	status := httptest.NewRecorder()
	router.ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/autosub/auto/status", nil))
	var body SweepStatusResponse
	if err := json.Unmarshal(status.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Available {
		t.Fatalf("expected the sweep to be unavailable, got %+v", body)
	}

	run := httptest.NewRecorder()
	router.ServeHTTP(run, httptest.NewRequest(http.MethodPost, "/autosub/auto/run", nil))
	if run.Code != http.StatusServiceUnavailable {
		t.Fatalf("run status=%d want=%d body=%s", run.Code, http.StatusServiceUnavailable, run.Body)
	}
}
