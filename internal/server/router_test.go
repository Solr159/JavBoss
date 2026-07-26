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

func TestFrontendStaticFilesAreServed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	staticDir := t.TempDir()
	files := map[string]string{
		"index.html":       "<!doctype html><title>frontend</title>",
		"site.webmanifest": `{"name":"JavBoss"}`,
		"icon-192.png":     "png data",
		"assets/app.js":    "console.log('JavBoss')",
		"ico/javdb.png":    "provider icon",
	}
	for name, content := range files {
		filePath := filepath.Join(staticDir, name)
		if err := os.MkdirAll(filepath.Dir(filePath), 0o700); err != nil {
			t.Fatalf("create directory for %s: %v", name, err)
		}
		if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	router := NewRouter(staticDir, testAuthService(t))
	for _, test := range []struct {
		method      string
		path        string
		body        string
		contentType string
	}{
		{
			method:      http.MethodGet,
			path:        "/site.webmanifest?v=1",
			body:        files["site.webmanifest"],
			contentType: "application/manifest+json",
		},
		{method: http.MethodGet, path: "/icon-192.png?v=1", body: files["icon-192.png"], contentType: "image/png"},
		{
			method:      http.MethodGet,
			path:        "/assets/app.js",
			body:        files["assets/app.js"],
			contentType: "text/javascript",
		},
		{
			method:      http.MethodGet,
			path:        "/ico/javdb.png",
			body:        files["ico/javdb.png"],
			contentType: "image/png",
		},
		{method: http.MethodHead, path: "/site.webmanifest?v=1", contentType: "application/manifest+json"},
	} {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			req := httptest.NewRequest(test.method, test.path, nil)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
			}
			if recorder.Body.String() != test.body {
				t.Fatalf("body = %q, want %q", recorder.Body.String(), test.body)
			}
			if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, test.contentType) {
				t.Fatalf("Content-Type = %q, want %q", contentType, test.contentType)
			}
		})
	}
}

func TestFrontendStaticFileCannotEscapeStaticDirectory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	parentDir := t.TempDir()
	staticDir := filepath.Join(parentDir, "dist")
	if err := os.Mkdir(staticDir, 0o700); err != nil {
		t.Fatalf("create static directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("<!doctype html>"), 0o600); err != nil {
		t.Fatalf("write index: %v", err)
	}
	secretPath := filepath.Join(parentDir, "secret.txt")
	if err := os.WriteFile(secretPath, []byte("secret"), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	if err := os.Symlink(secretPath, filepath.Join(staticDir, "secret.txt")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	router := NewRouter(staticDir, testAuthService(t))
	req := httptest.NewRequest(http.MethodGet, "/secret.txt", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if strings.Contains(recorder.Body.String(), "secret") {
		t.Fatalf("body exposed file outside static directory: %s", recorder.Body.String())
	}
}
