//go:build updatetest

package update

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocalUpdateSourceKeepsReleaseValidation(t *testing.T) {
	var location = "https://github.com/" + repository + "/releases/tag/v0.0.2"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead || r.URL.Path != "/"+repository+"/releases/latest" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		w.Header().Set("Location", location)
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()
	t.Setenv("JAVBOSS_UPDATE_TEST_URL", server.URL)
	client := newUpdateHTTPClient()
	defer client.CloseIdleConnections()
	endpoint := "https://github.com/" + repository + "/releases/latest"
	r, err := latestRelease(t.Context(), client, endpoint)
	if err != nil || r.Tag != "v0.0.2" {
		t.Fatalf("release=%+v err=%v", r, err)
	}
	location = "https://example.com/releases/tag/v0.0.2"
	if _, err := latestRelease(t.Context(), client, endpoint); err == nil {
		t.Fatal("accepted redirect outside official repository")
	}
	if _, err := client.Get("https://example.com/"); err == nil {
		t.Fatal("test client allowed an external request")
	}
}

func TestLocalUpdateSourceRejectsMissingOrNonLoopbackURL(t *testing.T) {
	for _, target := range []string{"", "http://example.com:80", "http://127.0.0.1", "http://127.0.0.1:80/path", "https://127.0.0.1:80"} {
		t.Run(target, func(t *testing.T) {
			t.Setenv("JAVBOSS_UPDATE_TEST_URL", target)
			client := newUpdateHTTPClient()
			defer client.CloseIdleConnections()
			if _, err := client.Get("https://github.com/" + repository + "/releases/latest"); err == nil {
				t.Fatal("invalid test source accepted")
			}
		})
	}
}
