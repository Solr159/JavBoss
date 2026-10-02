package util

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestPlainHTTPClientDoesNotUseNegativeCache(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	t.Cleanup(func() { negativeURLCache.Delete(server.URL) })

	for _, cached := range []bool{false, true} {
		entry := negativeURLCacheEntry{statusCode: http.StatusForbidden, expiresAt: time.Now().Add(time.Hour)}
		if cached {
			negativeURLCache.Store(server.URL, entry)
		}
		req, err := http.NewRequest(http.MethodGet, server.URL, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status=%d want=404", resp.StatusCode)
		}
		cachedEntry, exists := negativeURLCache.Load(server.URL)
		if exists != cached || (cached && cachedEntry != entry) {
			t.Fatalf("uncached request modified cache: entry=%v exists=%v", cachedEntry, exists)
		}
	}
	if requests.Load() != 2 {
		t.Fatalf("requests=%d want=2", requests.Load())
	}
}

func TestNegativeCacheTransportExpires(t *testing.T) {
	for _, firstStatus := range []int{http.StatusForbidden, http.StatusNotFound} {
		for _, status := range []int{http.StatusOK, http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError} {
			t.Run(http.StatusText(firstStatus)+" then "+http.StatusText(status), func(t *testing.T) {
				var requests atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if requests.Add(1) == 1 {
						w.WriteHeader(firstStatus)
						return
					}
					w.WriteHeader(status)
				}))
				defer server.Close()
				client := WithNegativeCache(server.Client())
				t.Cleanup(func() { negativeURLCache.Delete(server.URL) })
				request := func(wantStatus int, wantErr error) {
					t.Helper()
					resp, err := client.Get(server.URL)
					if resp != nil {
						defer resp.Body.Close()
					}
					if wantErr != nil {
						if !errors.Is(err, wantErr) || resp != nil {
							t.Fatalf("cached request: response=%v err=%v, want %v", resp, err, wantErr)
						}
						if wantErr == ErrCachedForbidden && errors.Is(err, ErrCachedNotFound) {
							t.Fatal("cached 403 was classified as not found")
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					if resp.StatusCode != wantStatus {
						t.Fatalf("status=%d want=%d", resp.StatusCode, wantStatus)
					}
				}
				cachedError := func(code int) error {
					switch code {
					case http.StatusForbidden:
						return ErrCachedForbidden
					case http.StatusNotFound:
						return ErrCachedNotFound
					default:
						return nil
					}
				}
				before := time.Now()
				request(firstStatus, nil)
				cached, ok := negativeURLCache.Load(server.URL)
				if !ok {
					t.Fatal("response not cached")
				}
				entry := cached.(negativeURLCacheEntry)
				if entry.statusCode != firstStatus || entry.expiresAt.Before(before.Add(7*24*time.Hour)) || entry.expiresAt.After(time.Now().Add(7*24*time.Hour)) {
					t.Fatalf("expected status and seven-day expiration, got %+v", entry)
				}
				request(0, cachedError(firstStatus))
				if requests.Load() != 1 {
					t.Fatal("unexpired cache allowed another HTTP request")
				}
				entry.expiresAt = time.Now().Add(-time.Second)
				negativeURLCache.Store(server.URL, entry)
				request(status, nil)
				if requests.Load() != 2 {
					t.Fatal("expired cache did not retry the HTTP request")
				}
				request(status, cachedError(status))
				wantRequests := int32(3)
				if cachedError(status) != nil {
					wantRequests = 2
				} else if _, ok := negativeURLCache.Load(server.URL); ok {
					t.Fatal("successful or server-error response retained an expired cache entry")
				}
				if requests.Load() != wantRequests {
					t.Fatalf("requests=%d want=%d", requests.Load(), wantRequests)
				}
			})
		}
	}
}

func TestForbiddenURLCacheOnlyAppliesToGetAndHead(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		t.Run(method, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusForbidden)
			}))
			defer server.Close()
			client := WithNegativeCache(server.Client())
			t.Cleanup(func() { negativeURLCache.Delete(server.URL) })
			request := func(method string, wantCached bool) {
				t.Helper()
				req, err := http.NewRequest(method, server.URL, nil)
				if err != nil {
					t.Fatal(err)
				}
				resp, err := client.Do(req)
				if resp != nil {
					defer resp.Body.Close()
				}
				if wantCached {
					if resp != nil || !errors.Is(err, ErrCachedForbidden) {
						t.Fatalf("resp=%v err=%v", resp, err)
					}
				} else if err != nil || resp == nil || resp.StatusCode != http.StatusForbidden {
					t.Fatalf("resp=%v err=%v", resp, err)
				}
			}
			request(http.MethodPost, false)
			request(http.MethodPost, false)
			if _, ok := negativeURLCache.Load(server.URL); ok {
				t.Fatal("POST 403 cached")
			}
			request(method, false)
			request(method, true)
			request(http.MethodPost, false)
			request(method, true)
			if calls.Load() != 4 {
				t.Fatalf("requests=%d want=4", calls.Load())
			}
		})
	}
}
