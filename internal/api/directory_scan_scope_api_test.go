package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"javboss/internal/common"
	"javboss/internal/db"

	"github.com/gin-gonic/gin"
)

// newDirectoryAPIRouter registers the directory handlers against an isolated database
// so the scan-scope request/response contract can be tested without the full server.
func newDirectoryAPIRouter(t *testing.T) (*gin.Engine, string) {
	t.Helper()
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

	router := gin.New()
	router.GET("/directories", listDirectories)
	router.POST("/directories", createDirectory)
	router.PATCH("/directories/:id", updateDirectory)
	return router, root
}

// waitForDirectoryScan blocks until the scan spawned by a directory write finishes.
// Without it the goroutine can outlive the test and touch a closed database.
func waitForDirectoryScan(t *testing.T, id int64, sinceUnixMS int64) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		dir, err := db.Default().GetDirectory(context.Background(), id)
		if err != nil {
			t.Fatalf("load directory %d: %v", id, err)
		}
		if dir != nil && dir.LastScanSummary.FinishedAtUnixMS > sinceUnixMS {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("directory %d scan did not finish", id)
}

func TestCreateDirectoryAcceptsSelectedSubdirectories(t *testing.T) {
	router, root := newDirectoryAPIRouter(t)
	library := filepath.Join(root, "library")
	for _, name := range []string{"keep", "skip"} {
		if err := os.MkdirAll(filepath.Join(library, name), 0700); err != nil {
			t.Fatal(err)
		}
	}

	body := `{"path":` + jsonString(library) + `,"scan_include":["keep"]}`
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/directories", strings.NewReader(body)))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var created struct {
		ID        int64  `json:"id"`
		Path      string `json:"path"`
		ScanScope struct {
			Include []string `json:"include"`
		} `json:"scan_scope"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Path != library {
		t.Fatalf("path = %q, want %q", created.Path, library)
	}
	if len(created.ScanScope.Include) != 1 || created.ScanScope.Include[0] != "keep" {
		t.Fatalf("scan scope = %v, want [keep]", created.ScanScope.Include)
	}
	waitForDirectoryScan(t, created.ID, 0)
}

func TestCreateDirectoryRejectsInvalidSubdirectories(t *testing.T) {
	router, root := newDirectoryAPIRouter(t)
	library := filepath.Join(root, "library")
	if err := os.MkdirAll(library, 0700); err != nil {
		t.Fatal(err)
	}
	for _, include := range []string{`["missing"]`, `[".."]`, `["` + root + `"]`} {
		body := `{"path":` + jsonString(library) + `,"scan_include":` + include + `}`
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/directories", strings.NewReader(body)))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("include %s: status = %d, body = %s", include, recorder.Code, recorder.Body.String())
		}
	}
}

func TestUpdateDirectoryReplacesScanScope(t *testing.T) {
	router, root := newDirectoryAPIRouter(t)
	library := filepath.Join(root, "library")
	for _, name := range []string{"a", "b"} {
		if err := os.MkdirAll(filepath.Join(library, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	dir, err := db.Default().CreateDirectoryWithScope(t.Context(), library, []string{"a"})
	if err != nil {
		t.Fatal(err)
	}

	path := "/directories/" + itoa(dir.ID)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPatch, path, strings.NewReader(`{"scan_include":["b"]}`)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	updated, err := db.Default().GetDirectory(t.Context(), dir.ID)
	if err != nil || updated == nil {
		t.Fatalf("reload directory: %v", err)
	}
	if got := updated.ScanScope.Includes(); len(got) != 1 || got[0] != "b" {
		t.Fatalf("scan scope = %v, want [b]", got)
	}
	waitForDirectoryScan(t, dir.ID, 0)

	// An empty list restores the default of scanning every subdirectory.
	before := updated.LastScanSummary.FinishedAtUnixMS
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPatch, path, strings.NewReader(`{"scan_include":[]}`)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	updated, err = db.Default().GetDirectory(t.Context(), dir.ID)
	if err != nil || updated == nil {
		t.Fatalf("reload directory: %v", err)
	}
	if got := updated.ScanScope.Includes(); len(got) != 0 {
		t.Fatalf("scan scope = %v, want none", got)
	}
	waitForDirectoryScan(t, dir.ID, before)
}

func jsonString(value string) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func itoa(value int64) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(data)
}
