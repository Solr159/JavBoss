package client

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestClientUpdateNeverForwardsLocalInstall(t *testing.T) {
	var requests atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.WriteHeader(http.StatusOK) }))
	defer remote.Close()
	client := newTestClient(t, remote.URL, nil)
	for _, path := range []string{"/__client/update", "/__client/update/check", "/__client/update/install"} {
		method := http.MethodPost
		if path == "/__client/update" {
			method = http.MethodGet
		}
		req := httptest.NewRequest(method, "http://127.0.0.1"+path, nil)
		req.Header.Set("Origin", "http://127.0.0.1")
		response := httptest.NewRecorder()
		client.ServeHTTP(response, req)
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("local status=%d", response.Code)
		}
	}
	if requests.Load() != 0 {
		t.Fatal("local update reached remote server")
	}
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/__client/update/install", nil)
	req.Header.Set("Origin", "http://attacker.example")
	response := httptest.NewRecorder()
	client.ServeHTTP(response, req)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin status=%d", response.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "http://127.0.0.1/system/update", nil)
	response = httptest.NewRecorder()
	client.ServeHTTP(response, req)
	if requests.Load() != 1 {
		t.Fatal("remote update status was not proxied")
	}
}
