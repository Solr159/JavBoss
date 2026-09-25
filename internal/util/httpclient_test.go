package util

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestDoRequestNotFoundCacheExpires(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusNotFound, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if requests.Add(1) == 1 {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.WriteHeader(status)
			}))
			defer server.Close()
			previousClient := DefaultHTTPClient()
			defaultHTTPClient = server.Client()
			t.Cleanup(func() {
				defaultHTTPClient = previousClient
				notFoundURLCache.Delete(server.URL)
			})

			request := func(wantStatus int, wantCached bool) {
				t.Helper()
				req, err := http.NewRequest(http.MethodGet, server.URL, nil)
				if err != nil {
					t.Fatal(err)
				}
				resp, err := DoRequest(req)
				if resp != nil {
					defer resp.Body.Close()
				}
				if wantCached {
					if !errors.Is(err, ErrCachedNotFound) || resp != nil {
						t.Fatalf("cached request: response=%v err=%v", resp, err)
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

			before := time.Now()
			request(http.StatusNotFound, false)
			expiresAt, ok := notFoundURLCache.Load(server.URL)
			if !ok || expiresAt.(time.Time).Before(before.Add(7*24*time.Hour)) || expiresAt.(time.Time).After(time.Now().Add(7*24*time.Hour)) {
				t.Fatalf("expected seven-day expiration, got %v", expiresAt)
			}
			request(0, true)
			if requests.Load() != 1 {
				t.Fatal("unexpired cache allowed another HTTP request")
			}

			// Advance the cache entry into the past without sleeping.
			notFoundURLCache.Store(server.URL, time.Now().Add(-time.Second))
			request(status, false)
			if requests.Load() != 2 {
				t.Fatal("expired cache did not retry the HTTP request")
			}
			request(status, status == http.StatusNotFound)
			wantRequests := int32(3)
			if status == http.StatusNotFound {
				wantRequests = 2
			} else if _, ok := notFoundURLCache.Load(server.URL); ok {
				t.Fatal("non-404 response retained an expired cache entry")
			}
			if requests.Load() != wantRequests {
				t.Fatalf("requests=%d want=%d", requests.Load(), wantRequests)
			}
		})
	}
}
