//go:build !updatetest

package update

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReleaseClientIgnoresLocalUpdateSource(t *testing.T) {
	t.Setenv("JAVBOSS_UPDATE_TEST_URL", "http://127.0.0.1:1")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "https://github.com/"+repository+"/releases/tag/v0.0.2")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()
	client := newUpdateHTTPClient()
	defer client.CloseIdleConnections()
	r, err := latestRelease(t.Context(), client, server.URL)
	if err != nil || r.Tag != "v0.0.2" {
		t.Fatalf("normal build used test source: release=%+v err=%v", r, err)
	}
}
