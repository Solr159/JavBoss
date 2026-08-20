package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"javboss/internal/common"
	"javboss/internal/db"
	"javboss/internal/models"

	"github.com/gin-gonic/gin"
)

func TestOpenListTestExplainsMissingTemporaryDirectory(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	previousDB := common.DB
	common.DB = database
	t.Cleanup(func() {
		common.DB = previousDB
		if sqlDB, dbErr := database.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	openList := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/me":
			_ = json.NewEncoder(response).Encode(map[string]any{
				"code": http.StatusOK, "data": map[string]any{"username": "admin", "role": 2},
			})
		case "/api/admin/setting/get":
			_ = json.NewEncoder(response).Encode(map[string]any{
				"code": http.StatusOK, "data": map[string]any{"value": ""},
			})
		default:
			response.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(response).Encode(map[string]any{"code": http.StatusNotFound})
		}
	}))
	defer openList.Close()
	if err := db.SaveDownloaderProviderSettings(context.Background(), &models.DownloaderProviderSettings{
		Provider: models.DownloaderProviderOpenList, Address: openList.URL,
		APIToken: "admin-token", RemoteFolder: "/115/JavBoss",
	}); err != nil {
		t.Fatalf("save OpenList settings: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router)
	response := performDiscoveryRequest(t, router, http.MethodPost, "/jav/downloader/providers/openlist/test", nil)
	if response.Code != http.StatusBadGateway {
		t.Fatalf("test status = %d body=%s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if !strings.Contains(body, "尚未配置 115 Open 临时目录") ||
		!strings.Contains(body, "The 115 Open temporary directory is not configured") {
		t.Fatalf("missing localized temporary-directory guidance: %s", body)
	}
}

func TestDownloaderAPIRevealsTokenOnlyThroughTokenEndpointAndQueuesForActiveProvider(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	previousDB := common.DB
	common.DB = database
	t.Cleanup(func() {
		common.DB = previousDB
		if sqlDB, dbErr := database.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	directory := models.Directory{Path: t.TempDir()}
	if err := database.Create(&directory).Error; err != nil {
		t.Fatalf("create directory: %v", err)
	}
	item := models.JavDiscoveryItem{
		Code:            "ABC-001",
		MetadataJSON:    `{}`,
		MagnetLinksJSON: `[{"name":"ABC-001 HD","url":"magnet:?xt=urn:btih:0123456789ABCDEF0123456789ABCDEF01234567&dn=ABC-001"}]`,
	}
	if err := database.Create(&item).Error; err != nil {
		t.Fatalf("create discovery item: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router)
	providerBody, _ := json.Marshal(map[string]any{
		"address": "http://127.0.0.1:5244", "api_token": "secret-token",
		"remote_folder": "/115/JavBoss",
	})
	saved := performDiscoveryRequest(t, router, http.MethodPut, "/jav/downloader/providers/openlist", providerBody)
	if saved.Code != http.StatusOK {
		t.Fatalf("save provider status = %d body=%s", saved.Code, saved.Body.String())
	}
	if value := saved.Body.String(); strings.Contains(value, "secret-token") {
		t.Fatalf("settings response exposed token: %s", value)
	}
	cloudDriveBody, _ := json.Marshal(map[string]any{
		"address": "http://127.0.0.1:19798", "api_token": "cloud-secret-token",
		"remote_folder": "/115/JavBoss",
	})
	saved = performDiscoveryRequest(t, router, http.MethodPut, "/jav/downloader/providers/clouddrive2", cloudDriveBody)
	if saved.Code != http.StatusOK || strings.Contains(saved.Body.String(), "cloud-secret-token") {
		t.Fatalf("save CloudDrive2 provider status=%d body=%s", saved.Code, saved.Body.String())
	}
	settingsBody, _ := json.Marshal(map[string]any{
		"active_provider": models.DownloaderProviderCloudDrive2,
		"directory_id":    directory.ID, "local_concurrency": 3,
	})
	saved = performDiscoveryRequest(t, router, http.MethodPut, "/jav/downloader/settings", settingsBody)
	if saved.Code != http.StatusOK {
		t.Fatalf("save settings status = %d body=%s", saved.Code, saved.Body.String())
	}
	if value := saved.Body.String(); !strings.Contains(value, `"local_concurrency":3`) {
		t.Fatalf("settings response omitted local concurrency: %s", value)
	}
	loaded := performDiscoveryRequest(t, router, http.MethodGet, "/jav/downloader/settings", nil)
	if loaded.Code != http.StatusOK || strings.Contains(loaded.Body.String(), "secret-token") {
		t.Fatalf("get settings status=%d body=%s", loaded.Code, loaded.Body.String())
	}
	revealed := performDiscoveryRequest(t, router, http.MethodGet, "/jav/downloader/providers/openlist/token", nil)
	if revealed.Code != http.StatusOK || !strings.Contains(revealed.Body.String(), `"api_token":"secret-token"`) {
		t.Fatalf("reveal OpenList token status=%d body=%s", revealed.Code, revealed.Body.String())
	}

	downloadBody, _ := json.Marshal(map[string]any{
		"magnet_url": "magnet:?xt=urn:btih:0123456789ABCDEF0123456789ABCDEF01234567&dn=ABC-001",
	})
	queued := performDiscoveryRequest(t, router, http.MethodPost, "/jav/discovery/items/"+strconv.FormatInt(item.ID, 10)+"/downloads", downloadBody)
	if queued.Code != http.StatusCreated {
		t.Fatalf("queue download status = %d body=%s", queued.Code, queued.Body.String())
	}
	jobs, err := db.ListDownloadJobs(context.Background(), 10)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("list jobs = %#v, %v", jobs, err)
	}
	if jobs[0].Status != models.DownloadQueued || jobs[0].Code != "ABC-001" ||
		jobs[0].Provider != models.DownloaderProviderCloudDrive2 || jobs[0].SourceType == nil ||
		*jobs[0].SourceType != models.DownloadSourceDiscovery || jobs[0].SourceID == nil || *jobs[0].SourceID != item.ID {
		t.Fatalf("unexpected queued job: %+v", jobs[0])
	}

	sourceFreeBody, _ := json.Marshal(map[string]any{
		"code": "XYZ-002", "magnet_name": "XYZ-002 HD",
		"magnet_url": "magnet:?xt=urn:btih:89ABCDEF0123456789ABCDEF0123456789ABCDEF&dn=XYZ-002",
	})
	queued = performDiscoveryRequest(t, router, http.MethodPost, "/jav/downloads", sourceFreeBody)
	if queued.Code != http.StatusCreated {
		t.Fatalf("queue source-free download status = %d body=%s", queued.Code, queued.Body.String())
	}
	var sourceFree models.DownloadJob
	if err := json.Unmarshal(queued.Body.Bytes(), &sourceFree); err != nil {
		t.Fatalf("decode source-free download: %v", err)
	}
	if sourceFree.SourceType != nil || sourceFree.SourceID != nil || sourceFree.Code != "XYZ-002" {
		t.Fatalf("unexpected source-free job: %+v", sourceFree)
	}
	listed := performDiscoveryRequest(t, router, http.MethodGet, "/jav/downloads", nil)
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), `"code":"XYZ-002"`) {
		t.Fatalf("list generic downloads status=%d body=%s", listed.Code, listed.Body.String())
	}
}
