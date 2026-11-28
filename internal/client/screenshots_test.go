package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
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

func TestScreenshotDeletionWaitsForUploadAndDeletesBothCopies(t *testing.T) {
	uploadStarted, releaseUpload := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseUpload) }) }
	defer release()
	var mu sync.Mutex
	remoteFiles := map[string]bool{"mpv_00-00-02.500.jpg": true, "mpv_00-00-03.jpg": true}
	denyDelete := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "session=pinned" {
			t.Error("delete/upload lost pinned credentials")
		}
		name := strings.TrimPrefix(r.URL.Path, "/videos/7/screenshots/")
		if r.Method == http.MethodPut {
			close(uploadStarted)
			<-releaseUpload
			mu.Lock()
			remoteFiles[name] = true
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodDelete {
			if denyDelete {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			delete(remoteFiles, name)
			w.WriteHeader(http.StatusOK)
			return
		}
		items := []map[string]string{}
		for name := range remoteFiles {
			items = append(items, map[string]string{"name": name})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
	}))
	defer server.Close()
	defer release()
	c := newTestClient(t, server.URL, nil)
	dir := filepath.Join(c.clientDataDir(), "video", "7", "screenshot")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	pending := filepath.Join(dir, "mpv_00-00-02.500.png")
	if err := os.WriteFile(pending, []byte("image"), 0600); err != nil {
		t.Fatal(err)
	}
	job := &screenshotSyncJob{client: c, ctx: t.Context(), videoID: 7, dir: dir, cookie: "session=pinned", files: make(map[string]*screenshotFileState)}
	job.syncOnce() // Record the new file; the next scan uploads it.
	uploadDone := make(chan struct{})
	go func() { job.syncOnce(); close(uploadDone) }()
	select {
	case <-uploadStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("upload did not start")
	}
	deleter := c.screenshotDeleter(7, "session=pinned")
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if err := deleter(ctx, 2.5); err != context.DeadlineExceeded {
		t.Fatalf("blocked delete ignored cancellation: %v", err)
	}
	deleted := make(chan error, 1)
	go func() { deleted <- deleter(t.Context(), 2.5) }()
	release()
	select {
	case err := <-deleted:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("delete stalled")
	}
	<-uploadDone
	job.syncOnce()
	mu.Lock()
	if len(remoteFiles) != 1 || !remoteFiles["mpv_00-00-03.jpg"] {
		t.Errorf("deleted screenshot returned or neighbor removed: %v", remoteFiles)
	}
	denyDelete = true
	mu.Unlock()
	if _, err := os.Stat(pending); !os.IsNotExist(err) {
		t.Fatalf("pending copy remains: %v", err)
	}
	// A failed remote deletion must preserve pending local screenshots.
	if err := os.WriteFile(pending, []byte("image"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := deleter(t.Context(), 2.5); err == nil {
		t.Fatal("expected remote deletion failure")
	}
	if _, err := os.Stat(pending); err != nil {
		t.Fatalf("failed delete removed pending file: %v", err)
	}
}
