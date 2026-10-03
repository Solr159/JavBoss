package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"javboss/internal/common"
	"javboss/internal/monitor"
)

func TestResourcesRoute(t *testing.T) {
	previous := common.AppConfig
	common.AppConfig = &common.Config{DatabasePath: filepath.Join(t.TempDir(), "test.db")}
	t.Cleanup(func() { common.AppConfig = previous })
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
	if snapshot.DataDisk == nil || snapshot.DataDisk.Path != filepath.Dir(common.AppConfig.DatabasePath) {
		t.Fatalf("wrong filesystem: %+v", snapshot.DataDisk)
	}
	if !isAPIPath("/system/resources/missing") {
		t.Fatal("system endpoints must not fall back to HTML")
	}
}

func TestResourcesRequireAuthentication(t *testing.T) {
	router := NewRouter("", testAuthService(t))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/system/resources", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated resource request: %d", response.Code)
	}
}
