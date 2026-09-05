package client

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"javboss/internal/mpv"
)

func TestPlaylistPlaysLocalGrantsAndStartsScreenshotSync(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/videos/playlist" {
			t.Error("playlist request reached remote player")
			w.WriteHeader(http.StatusNotImplemented)
			return
		}
		if r.Header.Get("Cookie") != "session=secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/stream") {
			w.WriteHeader(http.StatusPartialContent)
			_, _ = io.WriteString(w, "video")
		}
	}))
	defer remote.Close()
	c := newTestClient(t, remote.URL, nil)
	var played []mpv.PlaylistItem
	c.playPlaylist = func(items []mpv.PlaylistItem) error {
		played = items
		return nil
	}
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/videos/playlist", strings.NewReader(`{"items":[{"video_id":42,"location_id":7,"start_time":12.5},{"video_id":43,"location_id":8}]}`))
	req.Header.Set("Cookie", "session=secret")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	w := httptest.NewRecorder()
	c.ServeHTTP(w, req)
	if w.Code != http.StatusOK || len(played) != 2 || !strings.Contains(w.Body.String(), `"count":2`) {
		t.Fatalf("response=%d %s, played=%v", w.Code, w.Body.String(), played)
	}
	for i, item := range played {
		if item.Options.VideoID != int64(42+i) || !item.Options.EnableNetworkThumbnail {
			t.Fatalf("incorrect per-file options: %+v", item.Options)
		}
		parsed, err := url.Parse(item.Path)
		if err != nil || parsed.Host != "127.0.0.1" || !strings.HasPrefix(parsed.Path, "/__client/media/") {
			t.Fatalf("not a local media grant: %s", item.Path)
		}
		grant := c.lookupMediaGrant(strings.TrimPrefix(parsed.Path, "/__client/media/"))
		if grant == nil || grant.Cookie != "session=secret" || grant.RawQuery != []string{"location_id=7", "location_id=8"}[i] {
			t.Fatalf("incorrect media grant: %+v", grant)
		}
		media := httptest.NewRecorder()
		c.ServeHTTP(media, httptest.NewRequest(http.MethodGet, item.Path, nil))
		if media.Code != http.StatusPartialContent || media.Body.String() != "video" {
			t.Fatalf("media forwarding failed: %d %s", media.Code, media.Body.String())
		}
	}
	if played[0].Options.StartTimeSec != 12.5 {
		t.Fatal("lost start time")
	}
	c.screenshotMu.Lock()
	defer c.screenshotMu.Unlock()
	for _, item := range played {
		if c.screenshotJobs[item.Options.VideoID] == nil {
			t.Fatalf("missing screenshot sync for video %d", item.Options.VideoID)
		}
	}
}

func TestPlaylistRejectsInvalidOrUnavailableFilesBeforeLocalPlayback(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/videos/playlist" {
			t.Error("playlist was forwarded")
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer remote.Close()
	c := newTestClient(t, remote.URL, nil)
	c.playPlaylist = func([]mpv.PlaylistItem) error {
		t.Error("invalid playlist started local playback")
		return nil
	}
	for _, tc := range []struct {
		name, body, cookie, origin, method string
		status                             int
	}{
		{"empty", `{"items":[]}`, "session=x", "", "POST", 400},
		{"invalid ID", `{"items":[{"video_id":0}]}`, "session=x", "", "POST", 400},
		{"missing login", `{"items":[{"video_id":42}]}`, "", "", "POST", 401},
		{"missing media", `{"items":[{"video_id":42,"location_id":7}]}`, "session=x", "", "POST", 404},
		{"cross origin", `{"items":[{"video_id":42}]}`, "session=x", "http://untrusted.example", "POST", 403},
		{"wrong method", "", "session=x", "", "GET", 405},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "http://127.0.0.1/videos/playlist", strings.NewReader(tc.body))
			req.Header.Set("Cookie", tc.cookie)
			req.Header.Set("Origin", tc.origin)
			w := httptest.NewRecorder()
			c.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("status=%d, want=%d: %s", w.Code, tc.status, w.Body.String())
			}
		})
	}
}
