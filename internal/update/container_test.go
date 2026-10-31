package update

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLatestContainerRelease(t *testing.T) {
	for _, tc := range []struct {
		name, tags, want string
	}{
		{"numeric ordering", `["v2.9.0","v2.10.0","v2.1.0"]`, "v2.10.0"},
		{"ignore nonstable tags", `["latest","dev","v3.0.0-rc.1","browser-extension-v9.0.0","v02.0.0","v2.1.3"]`, "v2.1.3"},
		{"no stable version", `["latest","dev","v3.0.0-rc.1"]`, ""},
		{"empty registry", `[]`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Method != http.MethodGet {
					t.Errorf("unexpected method: %s", r.Method)
				}
				switch r.URL.Path {
				case "/token":
					if r.URL.Query().Get("service") != "ghcr.io" || r.URL.Query().Get("scope") != "repository:solr159/javboss:pull" || r.Header.Get("Authorization") != "" {
						t.Errorf("unexpected token request: %s", r.URL)
					}
					fmt.Fprint(w, `{"token":"anonymous-token"}`)
				case "/v2/solr159/javboss/tags/list":
					if r.Header.Get("Authorization") != "Bearer anonymous-token" || r.URL.Query().Get("n") != "100" {
						t.Error("missing registry authorization or page size")
					}
					fmt.Fprintf(w, `{"name":"solr159/javboss","tags":%s}`, tc.tags)
				default:
					t.Errorf("unexpected request: %s", r.URL)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			r, err := latestContainerRelease(t.Context(), server.Client(), server.URL)
			if err != nil || requests != 2 {
				t.Fatalf("release=%+v requests=%d err=%v", r, requests, err)
			}
			if tc.want == "" {
				if r != nil {
					t.Fatalf("expected no stable release, got %+v", r)
				}
			} else if r == nil || r.Tag != tc.want {
				t.Fatalf("release=%+v want=%s", r, tc.want)
			}
		})
	}
}

func TestContainerRegistryPagination(t *testing.T) {
	for _, tc := range []struct {
		name, link string
		bad        bool
	}{
		{"next page", `</v2/solr159/javboss/tags/list?n=100&last=v2.1.0>; rel="next"`, false},
		{"query relative", `<?n=100&last=v2.1.0>; rel="next"`, false},
		{"foreign host", `<https://example.com/v2/solr159/javboss/tags/list?n=100>; rel="next"`, true},
		{"foreign repository", `</v2/other/repo/tags/list?n=100>; rel="next"`, true},
		{"invalid link", `broken`, true},
		{"pagination loop", `</v2/solr159/javboss/tags/list?n=100>; rel="next"`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pages := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/token" {
					fmt.Fprint(w, `{"access_token":"anonymous-token"}`)
					return
				}
				pages++
				if pages == 1 {
					w.Header().Set("Link", tc.link)
					fmt.Fprint(w, `{"name":"solr159/javboss","tags":["v2.1.0"]}`)
					return
				}
				if r.URL.Query().Get("last") != "v2.1.0" || r.Header.Get("Authorization") != "Bearer anonymous-token" {
					t.Errorf("unexpected next page request: %s", r.URL)
				}
				fmt.Fprint(w, `{"name":"solr159/javboss","tags":["v2.10.0","v2.9.0"]}`)
			}))
			defer server.Close()
			r, err := latestContainerRelease(t.Context(), server.Client(), server.URL)
			if tc.bad {
				if err == nil || pages != 1 {
					t.Fatalf("release=%+v pages=%d err=%v", r, pages, err)
				}
			} else if err != nil || pages != 2 || r == nil || r.Tag != "v2.10.0" {
				t.Fatalf("release=%+v pages=%d err=%v", r, pages, err)
			}
		})
	}
}

func TestContainerRegistryFailures(t *testing.T) {
	for _, tc := range []struct {
		name, token, body string
		status            int
	}{
		{"empty token", `{}`, ``, 200},
		{"invalid token JSON", `not json`, ``, 200},
		{"unauthorized", `{"token":"test"}`, ``, 401},
		{"rate limited", `{"token":"test"}`, ``, 429},
		{"not found", `{"token":"test"}`, ``, 404},
		{"invalid tags JSON", `{"token":"test"}`, `<html>error</html>`, 200},
		{"wrong repository", `{"token":"test"}`, `{"name":"other/repo","tags":["v9.0.0"]}`, 200},
		{"oversized response", `{"token":"test"}`, strings.Repeat(" ", (1<<20)+1), 200},
		{"redirect", `{"token":"test"}`, ``, 302},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/token" {
					fmt.Fprint(w, tc.token)
					return
				}
				if r.URL.Path != "/v2/solr159/javboss/tags/list" {
					t.Errorf("unexpected request: %s", r.URL)
				}
				w.Header().Set("Location", "/must-not-follow")
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			if r, err := latestContainerRelease(t.Context(), server.Client(), server.URL); err == nil || r != nil {
				t.Fatalf("release=%+v err=%v", r, err)
			}
		})
	}
}

func TestContainerCheckUsesRegistryAndNeverInstalls(t *testing.T) {
	for _, tc := range []struct {
		version, reason string
		release, newer  bool
	}{
		{"v2.0.0", "container", true, true},
		{"v2.1.3", "container", true, false},
		{"v2.2.0", "container", true, false},
		{"dev", "development", false, false},
	} {
		t.Run(tc.version, func(t *testing.T) {
			m := newManager(t.Context(), Options{BaseDir: t.TempDir(), Version: tc.version, Release: tc.release, Container: true, Stop: func() { t.Error("checking stopped app") }})
			if m.endpoint != "https://ghcr.io" {
				t.Fatalf("wrong container endpoint: %s", m.endpoint)
			}
			// ARM64 containers must check updates even without a desktop archive.
			m.platform = ""
			m.status.Notes, m.status.NotesHTML = "old notes", "<p>old notes</p>"
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				switch r.URL.Path {
				case "/token":
					fmt.Fprint(w, `{"token":"test"}`)
				case "/v2/solr159/javboss/tags/list":
					fmt.Fprint(w, `{"name":"solr159/javboss","tags":["v2.1.3"]}`)
				default:
					t.Errorf("container check requested a non-registry URL: %s", r.URL)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			m.endpoint, m.client = server.URL, server.Client()
			m.check()
			status := m.Status()
			if requests != 2 || status.Phase != "idle" || status.Error != "" || status.CheckedAt == 0 || status.LatestVersion != "v2.1.3" || status.ReleaseURL != containerPackageURL || status.Available != tc.newer || status.DisabledReason != tc.reason || status.CanInstall || status.Notes != "" || status.NotesHTML != "" {
				t.Fatalf("requests=%d status=%+v", requests, status)
			}
			if m.selected != (asset{}) || m.StartInstall() == nil {
				t.Fatal("container update must not install a desktop archive")
			}
		})
	}
}

func TestContainerRegistryCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cancel()
		<-r.Context().Done()
	}))
	defer server.Close()
	defer cancel()
	if _, err := latestContainerRelease(ctx, server.Client(), server.URL); err == nil {
		t.Fatal("canceled container check succeeded")
	}
}
