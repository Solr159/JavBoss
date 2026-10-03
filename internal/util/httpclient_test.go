package util

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNegativeCacheReturnsIndependentHTTPResponses(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusNotFound} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(status)
				_, _ = io.WriteString(w, "original response")
			}))
			defer server.Close()
			t.Cleanup(func() { negativeURLCache.Delete(server.URL) })
			client := WithNegativeCache(server.Client())
			var previous *http.Response
			for attempt := 0; attempt < 3; attempt++ {
				req, err := http.NewRequest(http.MethodGet, server.URL, nil)
				if err != nil {
					t.Fatal(err)
				}
				resp, err := client.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				body, readErr := io.ReadAll(resp.Body)
				closeErr := resp.Body.Close()
				if readErr != nil || closeErr != nil {
					t.Fatalf("read=%v close=%v", readErr, closeErr)
				}
				if resp.StatusCode != status || !strings.Contains(resp.Status, http.StatusText(status)) || resp.Request != req {
					t.Fatalf("invalid response: %+v", resp)
				}
				if attempt == 0 && string(body) != "original response" {
					t.Fatal("network response body was changed")
				}
				if attempt > 0 && (len(body) != 0 || resp.ContentLength != 0 || resp == previous || resp.Header.Get("Modified") != "") {
					t.Fatalf("cached response reused state or returned a body: %+v", resp)
				}
				resp.Header.Set("Modified", "yes")
				previous = resp
			}
			if calls.Load() != 1 {
				t.Fatalf("network calls=%d want=1", calls.Load())
			}
		})
	}
}

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

func TestNegativeURLCacheDoesNotCachePostNotFound(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method == http.MethodGet || r.Header.Get("X-CSRF-Token") == "expired" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	t.Cleanup(func() { negativeURLCache.Delete(server.URL) })
	client := WithNegativeCache(server.Client())
	request := func(method, token string, want int) {
		t.Helper()
		req, err := http.NewRequest(method, server.URL, strings.NewReader(`{"code":"ABC-001"}`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-CSRF-Token", token)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Fatalf("status=%d want=%d", resp.StatusCode, want)
		}
	}
	request(http.MethodPost, "expired", http.StatusNotFound)
	request(http.MethodPost, "fresh", http.StatusOK)
	if _, ok := negativeURLCache.Load(server.URL); ok {
		t.Fatal("POST response was cached by URL")
	}
	request(http.MethodGet, "", http.StatusNotFound)
	request(http.MethodPost, "fresh", http.StatusOK)
	if calls.Load() != 4 {
		t.Fatalf("requests=%d want=4", calls.Load())
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
				request := func(wantStatus int) {
					t.Helper()
					resp, err := client.Get(server.URL)
					if resp != nil {
						defer resp.Body.Close()
					}
					if err != nil {
						t.Fatal(err)
					}
					if resp.StatusCode != wantStatus {
						t.Fatalf("status=%d want=%d", resp.StatusCode, wantStatus)
					}
				}
				before := time.Now()
				request(firstStatus)
				cached, ok := negativeURLCache.Load(server.URL)
				if !ok {
					t.Fatal("response not cached")
				}
				entry := cached.(negativeURLCacheEntry)
				if entry.statusCode != firstStatus || entry.expiresAt.Before(before.Add(7*24*time.Hour)) || entry.expiresAt.After(time.Now().Add(7*24*time.Hour)) {
					t.Fatalf("expected status and seven-day expiration, got %+v", entry)
				}
				request(firstStatus)
				if requests.Load() != 1 {
					t.Fatal("unexpired cache allowed another HTTP request")
				}
				entry.expiresAt = time.Now().Add(-time.Second)
				negativeURLCache.Store(server.URL, entry)
				request(status)
				if requests.Load() != 2 {
					t.Fatal("expired cache did not retry the HTTP request")
				}
				request(status)
				wantRequests := int32(3)
				if status == http.StatusForbidden || status == http.StatusNotFound {
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
			request := func(method string) {
				t.Helper()
				req, err := http.NewRequest(method, server.URL, nil)
				if err != nil {
					t.Fatal(err)
				}
				resp, err := client.Do(req)
				if resp != nil {
					defer resp.Body.Close()
				}
				if err != nil || resp == nil || resp.StatusCode != http.StatusForbidden {
					t.Fatalf("resp=%v err=%v", resp, err)
				}
			}
			request(http.MethodPost)
			request(http.MethodPost)
			if _, ok := negativeURLCache.Load(server.URL); ok {
				t.Fatal("POST 403 cached")
			}
			request(method)
			request(method)
			request(http.MethodPost)
			request(method)
			if calls.Load() != 4 {
				t.Fatalf("requests=%d want=4", calls.Load())
			}
		})
	}
}
