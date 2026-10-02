package util

import (
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestHTTPProbeDoesNotUseNegativeCache(t *testing.T) {
	for _, tc := range []struct {
		status    int
		cachedErr error
	}{
		{http.StatusNotFound, ErrCachedNotFound},
		{http.StatusForbidden, ErrCachedForbidden},
	} {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			var status, calls atomic.Int32
			status.Store(int32(tc.status))
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(int(status.Load()))
			}))
			defer server.Close()
			t.Cleanup(func() { negativeURLCache.Delete(server.URL) })
			cachedClient := WithNegativeCache(NewDefaultHTTPClient())
			defer cachedClient.CloseIdleConnections()
			probe := NewHTTPProbe(NewDefaultHTTPClient())
			defer probe.Close()
			request := func(client *http.Client) (*http.Response, error) {
				resp, err := client.Get(server.URL)
				if resp != nil {
					resp.Body.Close()
				}
				return resp, err
			}
			if _, err := request(cachedClient); err != nil {
				t.Fatal(err)
			}
			status.Store(http.StatusOK)
			resp, err := request(probe.Client)
			if err != nil || resp.StatusCode != 200 || calls.Load() != 2 || probe.HTTPStatus() != 200 {
				t.Fatalf("resp=%v err=%v calls=%d", resp, err, calls.Load())
			}
			if _, err := request(cachedClient); !errors.Is(err, tc.cachedErr) {
				t.Fatal("probe removed existing negative cache")
			}
			negativeURLCache.Delete(server.URL)
			status.Store(int32(tc.status))
			if _, err := request(probe.Client); err != nil {
				t.Fatal(err)
			}
			if _, ok := negativeURLCache.Load(server.URL); ok {
				t.Fatal("probe populated negative cache")
			}
			if probe.HTTPStatus() != tc.status {
				t.Fatal("probe did not record the latest response status")
			}
		})
	}
}

func TestHTTPProbeRecordsRedirectsAndKeepsSessionsIndependent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			if _, err := r.Cookie("session"); err == nil {
				t.Error("a fresh check reused another check's cookie")
			}
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "value"})
			http.Redirect(w, r, "/detail", http.StatusFound)
			return
		}
		if cookie, err := r.Cookie("session"); err != nil || cookie.Value != "value" {
			t.Error("cookie did not survive within the same check")
		}
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	var transports []http.RoundTripper
	for range 2 {
		client := NewDefaultHTTPClient()
		client.Jar, _ = cookiejar.New(nil)
		transports = append(transports, client.Transport)
		probe := NewHTTPProbe(client)
		resp, err := probe.Client.Get(server.URL + "/start")
		if err != nil {
			probe.Close()
			t.Fatal(err)
		}
		resp.Body.Close()
		probe.Close()
		if probe.HTTPStatus() != http.StatusForbidden {
			t.Fatalf("status=%d", probe.HTTPStatus())
		}
	}
	if transports[0] == transports[1] {
		t.Fatal("checks reused a transport")
	}
}
