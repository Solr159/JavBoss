package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"javboss/internal/common"
	"javboss/internal/monitor"

	"github.com/gin-gonic/gin"
)

func TestResourcesRoute(t *testing.T) {
	deps := newTestDependencies()

	deps.config = &common.Config{DatabasePath: filepath.Join(t.TempDir(), "test.db")}
	previousConfig := common.SetConfig(deps.config)
	t.Cleanup(func() { common.SetConfig(previousConfig) })
	router := gin.New()
	RegisterRoutes(router)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/system/resources", nil))
	var snapshot monitor.Snapshot
	if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil || response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s err=%v", response.Code, response.Body, err)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("resource metrics must not be HTTP cached")
	}
	if snapshot.DataDisk == nil || snapshot.DataDisk.Path != filepath.Dir(deps.config.DatabasePath) {
		t.Fatalf("wrong filesystem: %+v", snapshot.DataDisk)
	}
	if !isAPIPath("/system/resources/missing") {
		t.Fatal("system endpoints must not fall back to HTML")
	}
}

func TestResourcesRequireAuthentication(t *testing.T) {
	deps := newTestDependencies()
	router := NewRouter("", testAuthService(deps, t))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/system/resources", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated resource request: %d", response.Code)
	}
}
