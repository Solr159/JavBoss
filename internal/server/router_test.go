package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestIndexHTMLIgnoresStaleBrowserValidators(t *testing.T) {
	gin.SetMode(gin.TestMode)
	staticDir := t.TempDir()
	indexPath := filepath.Join(staticDir, "index.html")
	const currentIndex = "<!doctype html><html><body>current build</body></html>"
	if err := os.WriteFile(indexPath, []byte(currentIndex), 0o600); err != nil {
		t.Fatalf("write index: %v", err)
	}
	oldModTime := time.Now().Add(-24 * time.Hour)
	if err := os.Chtimes(indexPath, oldModTime, oldModTime); err != nil {
		t.Fatalf("set index modification time: %v", err)
	}

	router := NewRouter(staticDir, testAuthService(t))
	for _, path := range []string{"/", "/index.html", "/client/route"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Accept", "text/html")
			req.Header.Set("If-Modified-Since", time.Now().Add(24*time.Hour).UTC().Format(http.TimeFormat))
			req.Header.Set("If-None-Match", `"old-build"`)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
			}
			if recorder.Body.String() != currentIndex {
				t.Fatalf("body = %q, want current index", recorder.Body.String())
			}
			if cacheControl := recorder.Header().Get("Cache-Control"); !strings.Contains(cacheControl, "no-store") {
				t.Fatalf("Cache-Control = %q, want no-store", cacheControl)
			}
			if lastModified := recorder.Header().Get("Last-Modified"); lastModified != "" {
				t.Fatalf("Last-Modified = %q, want empty", lastModified)
			}
		})
	}
}

func TestUnknownToolAPIPathDoesNotServeIndexHTML(t *testing.T) {
	gin.SetMode(gin.TestMode)
	staticDir := t.TempDir()
	indexPath := filepath.Join(staticDir, "index.html")
	if err := os.WriteFile(indexPath, []byte("<!doctype html><title>frontend</title>"), 0o600); err != nil {
		t.Fatalf("write index: %v", err)
	}

	router := NewRouter(staticDir, testAuthService(t))
	req := httptest.NewRequest(http.MethodGet, "/tools/missing", nil)
	req.Header.Set("Accept", "text/html")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("Content-Type = %q, want JSON", contentType)
	}
	if strings.Contains(recorder.Body.String(), "<!doctype") {
		t.Fatalf("body served frontend HTML: %s", recorder.Body.String())
	}
}
