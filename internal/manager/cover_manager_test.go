package manager

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"javboss/internal/jav"
)

func TestSetCoverDownloadHeadersForJavBus(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://www.javbus.com/pics/cover/c85j_b.jpg", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	setCoverDownloadHeaders(req)

	if got := req.Header.Get("Referer"); got != "https://www.javbus.com/" {
		t.Fatalf("Referer = %q, want javbus referer", got)
	}
	if got := req.Header.Get("Cookie"); !strings.Contains(got, "age=verified") {
		t.Fatalf("Cookie = %q, want age verified cookie", got)
	}
	if got := req.Header.Get("User-Agent"); !strings.Contains(got, "Chrome/") {
		t.Fatalf("User-Agent = %q, want browser user agent", got)
	}
}

func TestEnqueueDeduplicatesScheduledCodes(t *testing.T) {
	manager := &CoverManager{
		tasks:     make(chan string, 2),
		scheduled: make(map[string]struct{}),
	}

	manager.Enqueue("ABC-001")
	manager.Enqueue("abc-001")
	manager.Enqueue(" ABC-001 ")
	manager.Enqueue("ABC-002")

	if got := len(manager.tasks); got != 2 {
		t.Fatalf("queued tasks = %d, want 2", got)
	}
	if got := <-manager.tasks; got != "abc-001" {
		t.Fatalf("first task = %q, want normalized abc-001", got)
	}
	if got := <-manager.tasks; got != "abc-002" {
		t.Fatalf("second task = %q, want normalized abc-002", got)
	}

	manager.clearScheduled("ABC-001")
	manager.Enqueue("ABC-001")
	if got := len(manager.tasks); got != 1 {
		t.Fatalf("queued tasks after clear = %d, want 1", got)
	}
}

func TestDownloadCoverRejectsSmallFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte(strings.Repeat("x", int(minValidCoverSizeBytes)-1)))
	}))
	defer server.Close()

	manager := &CoverManager{coverDir: t.TempDir()}
	err := manager.downloadCover(context.Background(), "ABC-001", server.URL+"/small.jpg")
	if !errors.Is(err, errInvalidCover) {
		t.Fatalf("downloadCover error = %v, want errInvalidCover", err)
	}
	if _, err := os.Stat(filepath.Join(manager.coverDir, "abc-001.jpg")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("small cover should not be finalized, stat err=%v", err)
	}
}

func TestDownloadCoverFromURLReplacesExistingCover(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte(strings.Repeat("p", int(minValidCoverSizeBytes))))
	}))
	defer server.Close()

	coverDir := t.TempDir()
	oldPath := filepath.Join(coverDir, "abc-001.jpg")
	if err := os.WriteFile(oldPath, []byte(strings.Repeat("j", int(minValidCoverSizeBytes))), 0o644); err != nil {
		t.Fatalf("write old cover: %v", err)
	}

	if err := DownloadCoverFromURL(context.Background(), coverDir, "ABC-001", server.URL+"/cover"); err != nil {
		t.Fatalf("DownloadCoverFromURL: %v", err)
	}

	if _, err := os.Stat(oldPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old cover should be removed, stat err=%v", err)
	}
	path, ok := FindCoverPath(coverDir, "ABC-001")
	if !ok {
		t.Fatal("new cover was not found")
	}
	if filepath.Base(path) != "abc-001.png" {
		t.Fatalf("new cover path = %q, want abc-001.png", path)
	}
}

func TestHandleTaskRetriesAfterSmallCover(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		switch r.URL.Path {
		case "/small.jpg":
			_, _ = w.Write([]byte(strings.Repeat("x", int(minValidCoverSizeBytes)-1)))
		case "/valid.jpg":
			_, _ = w.Write([]byte(strings.Repeat("y", int(minValidCoverSizeBytes))))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	originalLookup := lookupCoverURLByCode
	calls := map[jav.Provider]int{}
	lookupCoverURLByCode = func(code string, provider jav.Provider) (string, error) {
		calls[provider]++
		switch provider {
		case jav.ProviderJavDatabase:
			return server.URL + "/small.jpg", nil
		case jav.ProviderJavBus:
			return server.URL + "/valid.jpg", nil
		default:
			return "", jav.ResourceNotFonud
		}
	}
	t.Cleanup(func() { lookupCoverURLByCode = originalLookup })

	manager := &CoverManager{
		coverDir:  t.TempDir(),
		providers: []jav.Provider{jav.ProviderJavDatabase, jav.ProviderJavBus},
	}
	if err := manager.handleTask(context.Background(), "ABC-001"); err != nil {
		t.Fatalf("handleTask: %v", err)
	}

	if calls[jav.ProviderJavDatabase] != 1 || calls[jav.ProviderJavBus] != 1 {
		t.Fatalf("unexpected provider calls: %#v", calls)
	}
	info, err := os.Stat(filepath.Join(manager.coverDir, "abc-001.jpg"))
	if err != nil {
		t.Fatalf("stat final cover: %v", err)
	}
	if info.Size() != minValidCoverSizeBytes {
		t.Fatalf("final cover size = %d, want %d", info.Size(), minValidCoverSizeBytes)
	}
}
