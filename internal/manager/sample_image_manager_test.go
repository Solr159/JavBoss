package manager

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func encodeTestJPEG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 8, 8)), nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

func TestSampleImageManagerCachesRemoteImages(t *testing.T) {
	plain := encodeTestJPEG(t)
	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(plain)
	}))
	defer upstream.Close()

	manager := NewSampleImageManager(t.TempDir())
	source := upstream.URL + "/sample.jpg"

	first, err := manager.Get(context.Background(), source)
	if err != nil {
		t.Fatalf("first get: %v", err)
	}
	second, err := manager.Get(context.Background(), source)
	if err != nil {
		t.Fatalf("second get: %v", err)
	}
	if first != second {
		t.Fatalf("cache paths differ: %q vs %q", first, second)
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("upstream hits = %d, want 1 (second request must be served from disk)", got)
	}
	data, err := os.ReadFile(first)
	if err != nil {
		t.Fatalf("read cached image: %v", err)
	}
	if !bytes.Equal(data, plain) {
		t.Fatal("cached bytes differ from the source image")
	}
}

func TestSampleImageManagerDecodesXorEncodedImages(t *testing.T) {
	plain := encodeTestJPEG(t)
	encoded := make([]byte, len(plain)+1)
	encoded[0] = 0xa8
	for i, b := range plain {
		encoded[i+1] = b ^ encoded[0]
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "binary/octet-stream")
		_, _ = w.Write(encoded)
	}))
	defer upstream.Close()

	manager := NewSampleImageManager(t.TempDir())
	path, err := manager.Get(context.Background(), upstream.URL+"/encoded.jpg")
	if err != nil {
		t.Fatalf("get encoded image: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cached image: %v", err)
	}
	if !bytes.Equal(data, plain) {
		t.Fatal("encoded image was not decoded before caching")
	}
}

func TestSampleImageManagerRejectsNonImageResponses(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not an image"))
	}))
	defer upstream.Close()

	manager := NewSampleImageManager(t.TempDir())
	if _, err := manager.Get(context.Background(), upstream.URL+"/bad.jpg"); err == nil {
		t.Fatal("expected non-image response to be rejected")
	}
}

func TestSampleImageManagerGetDeduplicatesConcurrentRequests(t *testing.T) {
	plain := encodeTestJPEG(t)
	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		time.Sleep(50 * time.Millisecond)
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(plain)
	}))
	defer upstream.Close()

	manager := NewSampleImageManager(t.TempDir())
	source := upstream.URL + "/shared.jpg"

	var wg sync.WaitGroup
	paths := make([]string, 8)
	for i := range paths {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			path, err := manager.Get(context.Background(), source)
			if err != nil {
				t.Errorf("get %d: %v", index, err)
				return
			}
			paths[index] = path
		}(i)
	}
	wg.Wait()

	if got := hits.Load(); got != 1 {
		t.Fatalf("upstream hits = %d, want 1 (concurrent requests must be coalesced)", got)
	}
	for _, path := range paths {
		if path == "" || path != paths[0] {
			t.Fatalf("paths = %#v, want all identical", paths)
		}
	}
}

func TestSampleImageManagerWarmPrefetchesInBackground(t *testing.T) {
	plain := encodeTestJPEG(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(plain)
	}))
	defer upstream.Close()

	manager := NewSampleImageManager(t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager.Start(ctx)

	source := upstream.URL + "/warm.jpg"
	manager.Warm(source)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := manager.cached(source); ok {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("warm request was not cached in time")
}
