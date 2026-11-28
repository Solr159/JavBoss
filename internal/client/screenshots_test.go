package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func TestScreenshotResolverMergesPendingUploadsAndRemoteChanges(t *testing.T) {
	var mu sync.Mutex
	names := []string{"mpv_00-00-10.jpg", "mpv_00-00-10.png", "128.jpg"}
	status := http.StatusOK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/videos/7/screenshots" || r.Header.Get("Cookie") != "session=pinned" {
			t.Errorf("unexpected screenshot request: %s %s", r.URL.Path, r.Header.Get("Cookie"))
		}
		mu.Lock()
		defer mu.Unlock()
		w.WriteHeader(status)
		items := []map[string]string{}
		for _, name := range names {
			items = append(items, map[string]string{"name": name})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
	}))
	defer server.Close()
	c := newTestClient(t, server.URL, nil)
	dir := filepath.Join(c.clientDataDir(), "video", "7", "screenshot")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	pending := filepath.Join(dir, "mpv_00-00-20.250.png")
	if err := os.WriteFile(pending, []byte("image"), 0600); err != nil {
		t.Fatal(err)
	}
	resolve := c.screenshotResolver(7, "session=pinned")
	check := func(want []float64) {
		t.Helper()
		got, err := resolve(t.Context())
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("times=%v, err=%v, want=%v", got, err, want)
		}
	}
	check([]float64{10, 20.25})
	// An upload moves ownership to the server without losing its marker.
	mu.Lock()
	names = append(names, filepath.Base(pending))
	mu.Unlock()
	if err := os.Remove(pending); err != nil {
		t.Fatal(err)
	}
	check([]float64{10, 20.25})
	mu.Lock()
	names = []string{}
	mu.Unlock()
	check([]float64{})
	mu.Lock()
	status = http.StatusUnauthorized
	mu.Unlock()
	if _, err := resolve(t.Context()); err == nil {
		t.Fatal("authentication failure must not replace markers with an empty list")
	}
}
