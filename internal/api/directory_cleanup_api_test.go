package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	dbpkg "javboss/internal/db"
	"javboss/internal/directory"
	"javboss/internal/models"
)

func TestDirectoryCleanupRoutes(t *testing.T) {
	deps := newTestDependencies()
	database, err := dbpkg.Open(filepath.Join(t.TempDir(), "cleanup.db"))
	if err != nil {
		t.Fatal(err)
	}
	deps.database = database
	deps.bindDatabase(t)
	t.Cleanup(func() {
		if conn, err := database.DB(); err == nil {
			_ = conn.Close()
		}
	})
	dir := models.Directory{ID: 1, Path: t.TempDir()}
	if err := database.Create(&dir).Error; err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ad.txt", "video.mp4"} {
		if err := os.WriteFile(filepath.Join(dir.Path, name), []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	router := gin.New()
	RegisterRoutes(router)
	request := func(route string, payload any) *httptest.ResponseRecorder {
		t.Helper()
		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, route, bytes.NewReader(body)))
		return recorder
	}
	previewResponse := request("/directories/1/cleanup/preview", gin.H{"include_misc": true})
	var preview directory.CleanupPreview
	if err := json.Unmarshal(previewResponse.Body.Bytes(), &preview); err != nil || previewResponse.Code != 200 || len(preview.Files) != 1 {
		t.Fatalf("preview: %d %s err=%v", previewResponse.Code, previewResponse.Body, err)
	}
	response := request("/directories/1/cleanup", gin.H{"root_path": dir.Path, "files": preview.Files})
	var result directory.CleanupResult
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != 200 || len(result.Deleted) != 1 {
		t.Fatalf("cleanup: %d %s err=%v", response.Code, response.Body, err)
	}
	if _, err := os.Stat(filepath.Join(dir.Path, "ad.txt")); !os.IsNotExist(err) {
		t.Fatalf("ad still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir.Path, "video.mp4")); err != nil {
		t.Fatalf("video was affected: %v", err)
	}
	for _, tc := range []struct {
		path string
		body any
		code int
	}{
		{"/directories/no/cleanup/preview", gin.H{}, 400},
		{"/directories/999/cleanup/preview", gin.H{}, 404},
		{"/directories/1/cleanup", gin.H{"root_path": dir.Path}, 400},
		{"/directories/1/cleanup", gin.H{"root_path": "stale", "files": preview.Files}, 400},
	} {
		if response := request(tc.path, tc.body); response.Code != tc.code {
			t.Fatalf("%s: %d %s", tc.path, response.Code, response.Body)
		}
	}
	reserved, err := directory.CancelAndReserveDirectoryScan(t.Context(), dir.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer reserved()
	if response := request("/directories/1/cleanup/preview", gin.H{}); response.Code != http.StatusConflict {
		t.Fatalf("busy directory: %d %s", response.Code, response.Body)
	}
	reserved()
	// Protect a video indexed through an overlapping root, including a download suffix.
	subdir := models.Directory{ID: 2, Path: filepath.Join(dir.Path, "nested")}
	if err := os.Mkdir(subdir.Path, 0755); err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&subdir).Error; err != nil {
		t.Fatal(err)
	}
	video := models.Video{Fingerprint: "cleanup-indexed-download"}
	if err := database.Create(&video).Error; err != nil {
		t.Fatal(err)
	}
	name := "video.mp4.bt.xltd"
	if err := os.WriteFile(filepath.Join(subdir.Path, name), []byte("indexed video"), 0644); err != nil {
		t.Fatal(err)
	}
	loc := models.VideoLocation{VideoID: video.ID, DirectoryID: subdir.ID, RelativePath: name, Filename: name}
	if err := database.Create(&loc).Error; err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir.Path, "ad.torrent"), []byte("torrent"), 0644); err != nil {
		t.Fatal(err)
	}
	response = request("/directories/1/cleanup/preview", gin.H{"include_downloads": true, "include_torrents": true})
	if err := json.Unmarshal(response.Body.Bytes(), &preview); err != nil || response.Code != 200 || len(preview.Files) != 1 || preview.Files[0].Path != "ad.torrent" {
		t.Fatalf("expanded preview: %d %s err=%v", response.Code, response.Body, err)
	}
	info, err := os.Stat(filepath.Join(subdir.Path, name))
	if err != nil {
		t.Fatal(err)
	}
	response = request("/directories/1/cleanup", gin.H{"root_path": dir.Path, "files": []directory.CleanupFile{{Path: "nested/" + name, Size: info.Size(), Modified: info.ModTime().UTC().Format("2006-01-02T15:04:05.999999999Z"), Reason: "download"}}})
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != 200 || len(result.Deleted) != 0 || len(result.Issues) != 1 || result.Issues[0].Reason != "indexed_video" {
		t.Fatalf("indexed file delete: %d %s err=%v", response.Code, response.Body, err)
	}
}

func TestDirectoryCleanupRequiresAuthentication(t *testing.T) {
	deps := newTestDependencies()
	router := NewRouter("", testAuthService(deps, t))
	for _, route := range []string{"/directories/1/cleanup/preview", "/directories/1/cleanup"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, route, bytes.NewBufferString("{}")))
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated %s: %d", route, response.Code)
		}
	}
}

func TestDirectoryCleanupResponsesOutlastServerWriteTimeout(t *testing.T) {
	deps := newTestDependencies()
	database, err := dbpkg.Open(filepath.Join(t.TempDir(), "cleanup.db"))
	if err != nil {
		t.Fatal(err)
	}
	conn, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	deps.database = database
	deps.bindDatabase(t)
	dir := models.Directory{ID: 1, Path: t.TempDir()}
	if err := database.Create(&dir).Error; err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(dir.Path, "ad.txt")
	if err := os.WriteFile(filename, []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Delay a real cleanup operation without requiring slow disks or FFprobe.
	// Both endpoints read video locations before walking/deleting files.
	var delayedQueries atomic.Int32
	if err := database.Callback().Query().Before("gorm:query").Register("test:slow_cleanup_index", func(tx *gorm.DB) {
		if tx.Statement.Table != "video_location" {
			return
		}
		delayedQueries.Add(1)
		timer := time.NewTimer(150 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-tx.Statement.Context.Done():
			tx.AddError(tx.Statement.Context.Err())
		}
	}); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	RegisterRoutes(router)
	server := httptest.NewUnstartedServer(router)
	server.Config.WriteTimeout = 50 * time.Millisecond
	server.Start()
	defer server.Close()
	client := server.Client()
	client.Timeout = 5 * time.Second
	request := func(route string, payload, result any) {
		t.Helper()
		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		response, err := client.Post(server.URL+route, "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("long cleanup request lost its response: %v", err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s status: %d", route, response.StatusCode)
		}
		if err := json.NewDecoder(response.Body).Decode(result); err != nil {
			t.Fatalf("%s returned incomplete JSON: %v", route, err)
		}
	}
	var preview directory.CleanupPreview
	request("/directories/1/cleanup/preview", gin.H{"include_misc": true}, &preview)
	if len(preview.Files) != 1 || preview.Files[0].Path != "ad.txt" {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if _, err := os.Stat(filename); err != nil {
		t.Fatalf("preview changed the file: %v", err)
	}
	var result directory.CleanupResult
	request("/directories/1/cleanup", gin.H{"root_path": dir.Path, "files": preview.Files}, &result)
	if len(result.Deleted) != 1 || result.Deleted[0] != "ad.txt" || result.DeletedBytes != 4 || len(result.Issues) != 0 {
		t.Fatalf("unexpected cleanup result: %+v", result)
	}
	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		t.Fatalf("cleanup result does not match filesystem: %v", err)
	}
	if delayedQueries.Load() != 2 {
		t.Fatalf("expected both requests to exceed the write timeout, got %d delays", delayedQueries.Load())
	}
	if directory.DirectoryWorkStatus(dir.ID) != directory.DirectoryWorkIdle {
		t.Fatal("cleanup request leaked the directory reservation")
	}
}
