package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"javboss/internal/common"
	dbpkg "javboss/internal/db"

	"github.com/gin-gonic/gin"
)

func TestUpdateConfigPersistsJavWaterfallDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database, err := dbpkg.Open(filepath.Join(t.TempDir(), "config.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("database handle: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	previousDB := common.DB
	common.DB = database
	t.Cleanup(func() { common.DB = previousDB })

	router := gin.New()
	router.PATCH("/config", updateConfig)
	body := []byte(`{
		"jav_waterfall_default": true,
		"idol_waterfall_default": false,
		"studio_waterfall_default": true,
		"series_waterfall_default": false,
		"jav_tag_show_simplified": true
	}`)
	req := httptest.NewRequest(http.MethodPatch, "/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("update config status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}

	got, err := dbpkg.ListConfig(context.Background())
	if err != nil {
		t.Fatalf("list config: %v", err)
	}
	want := map[string]string{
		"jav_waterfall_default":    "true",
		"idol_waterfall_default":   "false",
		"studio_waterfall_default": "true",
		"series_waterfall_default": "false",
		"jav_tag_show_simplified":  "true",
	}
	for key, value := range want {
		if got[key] != value {
			t.Errorf("config %s = %q, want %q", key, got[key], value)
		}
	}
}

func TestUpdateConfigAcceptsBrowserPlayerAndLANAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database, err := dbpkg.Open(filepath.Join(t.TempDir(), "config.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("database handle: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	previousDB := common.DB
	common.DB = database
	t.Cleanup(func() { common.DB = previousDB })

	router := gin.New()
	router.PATCH("/config", updateConfig)
	req := httptest.NewRequest(
		http.MethodPatch,
		"/config",
		bytes.NewReader([]byte(`{"default_player":"browser","allow_lan_access":true}`)),
	)
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("update config status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}

	got, err := dbpkg.ListConfig(context.Background())
	if err != nil {
		t.Fatalf("list config: %v", err)
	}
	if got["default_player"] != "browser" {
		t.Fatalf("default_player = %q, want browser", got["default_player"])
	}
	if got["allow_lan_access"] != "true" {
		t.Fatalf("allow_lan_access = %q, want true", got["allow_lan_access"])
	}
}

func TestNormalizeProxyHost(t *testing.T) {
	tests := []struct {
		name string
		host string
		want string
		ok   bool
	}{
		{name: "ipv4", host: "192.168.1.10", want: "192.168.1.10", ok: true},
		{name: "hostname", host: "proxy.local", want: "proxy.local", ok: true},
		{name: "bracketed ipv6", host: "[::1]", want: "::1", ok: true},
		{name: "url host", host: "http://10.0.0.2", want: "10.0.0.2", ok: true},
		{name: "host with port", host: "10.0.0.2:7890", ok: false},
		{name: "path", host: "10.0.0.2/proxy", ok: false},
		{name: "space", host: "bad host", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := normalizeProxyHost(tt.host)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("normalizeProxyHost(%q) = (%q, %v), want (%q, %v)", tt.host, got, ok, tt.want, tt.ok)
			}
		})
	}
}
