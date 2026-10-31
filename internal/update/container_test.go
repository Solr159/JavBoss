package update

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

var testLatestDigest = "sha256:" + strings.Repeat("a", 64)
var testOtherDigest = "sha256:" + strings.Repeat("b", 64)

func serveTestManifest(t *testing.T, w http.ResponseWriter, r *http.Request, matchingTag string) bool {
	t.Helper()
	const prefix = "/v2/solr159/javboss/manifests/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		return false
	}
	if r.Method != http.MethodHead || r.Header.Get("Authorization") == "" {
		t.Errorf("unexpected manifest request: %s %s", r.Method, r.URL)
	}
	for _, mediaType := range []string{
		"application/vnd.oci.image.index.v1+json", "application/vnd.oci.image.manifest.v1+json",
		"application/vnd.docker.distribution.manifest.list.v2+json", "application/vnd.docker.distribution.manifest.v2+json",
	} {
		if !strings.Contains(r.Header.Get("Accept"), mediaType) {
			t.Errorf("manifest request does not accept %s", mediaType)
		}
	}
	tag := strings.TrimPrefix(r.URL.Path, prefix)
	if tag != "latest" && !releaseTag.MatchString(tag) {
		t.Errorf("unexpected manifest tag: %s", tag)
	}
	digest := testOtherDigest
	if tag == "latest" || tag == matchingTag {
		digest = testLatestDigest
	}
	w.Header().Set("Docker-Content-Digest", digest)
	return true
}

func TestLatestContainerRelease(t *testing.T) {
	for _, tc := range []struct {
		name, tags, want string
		requests         int
	}{
		{"numeric ordering", `["v2.9.0","v2.10.0","v2.1.0"]`, "v2.10.0", 4},
		{"latest behind highest version", `["latest","v2.9.0","v2.10.0","v2.1.0"]`, "v2.9.0", 5},
		{"latest rolled back", `["latest","v2.9.0","v2.10.0","v2.1.0"]`, "v2.1.0", 6},
		{"ignore nonstable tags", `["latest","dev","v3.0.0-rc.1","browser-extension-v9.0.0","v02.0.0","v2.1.3"]`, "v2.1.3", 4},
		{"no matching image", `["latest","v2.1.3"]`, "", 4},
		{"no stable version", `["latest","dev","v3.0.0-rc.1"]`, "", 3},
		{"empty registry", `[]`, "", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if serveTestManifest(t, w, r, tc.want) {
					return
				}
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
			if (err != nil) != (tc.want == "") || requests != tc.requests {
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
				if serveTestManifest(t, w, r, "v2.9.0") {
					return
				}
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
			} else if err != nil || pages != 2 || r == nil || r.Tag != "v2.9.0" {
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
				if serveTestManifest(t, w, r, "") {
					return
				}
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

type containerTestTransport func(*http.Request) (*http.Response, error)

func (f containerTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestContainerCheckUsesRegistryVersionForReleaseNotesAndNeverInstalls(t *testing.T) {
	for _, tc := range []struct {
		version, reason string
		release, newer  bool
		notesStatus     int
	}{
		{"v2.0.0", "container", true, true, http.StatusOK},
		{"v2.1.3", "container", true, false, http.StatusOK},
		{"v2.2.0", "container", true, false, http.StatusOK},
		{"dev", "development", false, false, http.StatusOK},
		{"v2.0.0", "container", true, true, http.StatusNotFound},
		{"v2.0.0", "container", true, true, http.StatusForbidden},
	} {
		t.Run(tc.version+"/"+http.StatusText(tc.notesStatus), func(t *testing.T) {
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
				if serveTestManifest(t, w, r, "v2.1.3") {
					return
				}
				switch r.URL.Path {
				case "/token":
					fmt.Fprint(w, `{"token":"test"}`)
				case "/v2/solr159/javboss/tags/list":
					fmt.Fprint(w, `{"name":"solr159/javboss","tags":["latest","v2.2.0","v2.1.3"]}`)
				case "/Solr159/JavBoss/releases/tag/v2.1.3":
					if r.Method != http.MethodGet || r.Header.Get("Authorization") != "" {
						t.Errorf("unexpected release notes request: %s %s", r.Method, r.URL)
					}
					w.WriteHeader(tc.notesStatus)
					fmt.Fprint(w, `<div data-test-selector="body-content" class="markdown-body"><h2>更新说明</h2><p>修复 <strong>Docker 更新</strong></p></div>`)
				default:
					t.Errorf("unexpected container update request: %s", r.URL)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			m.endpoint, m.client = server.URL, server.Client()
			// Keep the production GitHub URL in the request, but serve its page
			// locally so the test checks the selected tag without network access.
			serverURL, _ := url.Parse(server.URL)
			transport := m.client.Transport
			m.client.Transport = containerTestTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host == "github.com" {
					if r.URL.Scheme != "https" {
						t.Errorf("insecure release notes URL: %s", r.URL)
					}
					r = r.Clone(r.Context())
					r.URL.Scheme, r.URL.Host = serverURL.Scheme, serverURL.Host
				}
				return transport.RoundTrip(r)
			})
			m.check()
			status := m.Status()
			if requests != 6 || status.Phase != "idle" || status.Error != "" || status.CheckedAt == 0 || status.LatestVersion != "v2.1.3" || status.ReleaseURL != "https://github.com/Solr159/JavBoss/releases" || status.Available != tc.newer || status.DisabledReason != tc.reason || status.CanInstall {
				t.Fatalf("requests=%d status=%+v", requests, status)
			}
			wantNotes, wantHTML := "", ""
			if tc.notesStatus == http.StatusOK {
				wantNotes = "更新说明\n修复 Docker 更新"
				wantHTML = "<h2>更新说明</h2><p>修复 <strong>Docker 更新</strong></p>"
			}
			if status.Notes != wantNotes || status.NotesHTML != wantHTML {
				t.Fatalf("notes=%q HTML=%q", status.Notes, status.NotesHTML)
			}
			if m.selected != (asset{}) || m.StartInstall() == nil {
				t.Fatal("container update must not install a desktop archive")
			}
		})
	}
}

func TestContainerManifestFailuresDoNotFallBackToHighestVersion(t *testing.T) {
	for _, tag := range []string{"latest", "v2.1.3"} {
		for _, tc := range []struct {
			name, digest string
			status       int
		}{
			{"missing image", "", 404},
			{"unauthorized", "", 401},
			{"rate limited", "", 429},
			{"redirect", testLatestDigest, 302},
			{"missing digest", "", 200},
			{"invalid digest", "sha256:invalid", 200},
		} {
			t.Run(tag+"/"+tc.name, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/v2/solr159/javboss/manifests/"+tag {
						w.Header().Set("Location", "/must-not-follow")
						w.Header().Set("Docker-Content-Digest", tc.digest)
						w.WriteHeader(tc.status)
						return
					}
					if serveTestManifest(t, w, r, "v2.1.3") {
						return
					}
					switch r.URL.Path {
					case "/token":
						fmt.Fprint(w, `{"token":"test"}`)
					case "/v2/solr159/javboss/tags/list":
						fmt.Fprint(w, `{"name":"solr159/javboss","tags":["v2.1.3"]}`)
					default:
						t.Errorf("unexpected request: %s", r.URL)
					}
				}))
				defer server.Close()
				if r, err := latestContainerRelease(t.Context(), server.Client(), server.URL); err == nil || r != nil {
					t.Fatalf("release=%+v err=%v", r, err)
				}
			})
		}
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
