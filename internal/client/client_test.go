package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"javboss/internal/mpv"
)

func TestNormalizeServerURL(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "http", value: " http://server.local:8655/ ", want: "http://server.local:8655"},
		{name: "https", value: "https://example.com", want: "https://example.com"},
		{name: "missing scheme", value: "example.com", wantErr: true},
		{name: "credentials", value: "https://user:pass@example.com", wantErr: true},
		{name: "subpath", value: "https://example.com/javboss", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := NormalizeServerURL(test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("NormalizeServerURL() error = %v, wantErr %v", err, test.wantErr)
			}
			if got != test.want {
				t.Fatalf("NormalizeServerURL() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestLoadBootstrapConfig(t *testing.T) {
	baseDir := t.TempDir()
	config := []byte("server_url = \"https://example.com\"\nport = 9000\n")
	if err := os.WriteFile(filepath.Join(baseDir, bootstrapConfigName), config, 0o600); err != nil {
		t.Fatalf("write bootstrap config: %v", err)
	}
	got, err := LoadBootstrapConfig(baseDir)
	if err != nil {
		t.Fatalf("LoadBootstrapConfig: %v", err)
	}
	if got.ServerURL != "https://example.com" || got.Port != 9000 {
		t.Fatalf("loaded config = %#v", got)
	}
}

func TestNewClientRequiresRemoteServerURL(t *testing.T) {
	_, err := New(Options{
		BaseDir:      t.TempDir(),
		LocalBaseURL: "http://127.0.0.1:8655",
	})
	if err == nil || !strings.Contains(err.Error(), "remote server URL is required") {
		t.Fatalf("New() error = %v", err)
	}
}

func TestProxyRewritesOriginAndSecureCookie(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/login" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("Origin"); got != "http://"+r.Host {
			t.Errorf("remote Origin = %q, want %q", got, "http://"+r.Host)
		}
		http.SetCookie(w, &http.Cookie{Name: "remote_session", Value: "secret", Path: "/", Secure: true, HttpOnly: true})
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"authenticated":true}`)
	}))
	defer remote.Close()

	client := newTestClient(t, remote.URL, nil)
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/auth/login", strings.NewReader(`{"password":"secret"}`))
	request.Header.Set("Origin", "http://127.0.0.1")
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	recorder := httptest.NewRecorder()
	client.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	cookie := recorder.Header().Get("Set-Cookie")
	if cookie == "" || strings.Contains(strings.ToLower(cookie), "secure") || strings.Contains(strings.ToLower(cookie), "domain=") {
		t.Fatalf("rewritten Set-Cookie = %q", cookie)
	}
}

func TestProxyRejectsCrossSiteMutation(t *testing.T) {
	called := false
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer remote.Close()
	client := newTestClient(t, remote.URL, nil)
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/tags", strings.NewReader(`{"name":"x"}`))
	request.Header.Set("Origin", "https://attacker.example")
	request.Header.Set("Sec-Fetch-Site", "cross-site")
	recorder := httptest.NewRecorder()
	client.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
	if called {
		t.Fatal("cross-site mutation reached remote server")
	}
}

func TestClientPlaybackUsesLocalMPVAndRangeProxy(t *testing.T) {
	playCount := make(chan struct{}, 1)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "remote_session=secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/videos/42/stream" && r.Header.Get("Range") == "bytes=0-0":
			if r.URL.Query().Get("location_id") != "7" {
				t.Errorf("location_id = %q", r.URL.Query().Get("location_id"))
			}
			w.Header().Set("Accept-Ranges", "bytes")
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/videos/42/stream":
			if r.Header.Get("Range") != "bytes=2-5" {
				t.Errorf("Range = %q", r.Header.Get("Range"))
			}
			w.Header().Set("Content-Range", "bytes 2-5/6")
			w.Header().Set("Content-Type", "video/mp4")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = io.WriteString(w, "cdef")
		case r.Method == http.MethodPost && r.URL.Path == "/videos/42/play":
			playCount <- struct{}{}
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer remote.Close()

	var mu sync.Mutex
	var playedURL string
	var playedOptions mpv.PlayOptions
	client := newTestClient(t, remote.URL, func(path string, options mpv.PlayOptions) error {
		mu.Lock()
		playedURL = path
		playedOptions = options
		mu.Unlock()
		return nil
	})
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/videos/play", strings.NewReader(`{"video_id":42,"location_id":7,"start_time":12.5}`))
	request.Header.Set("Cookie", "remote_session=secret")
	request.Header.Set("Origin", "http://127.0.0.1")
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	recorder := httptest.NewRecorder()
	client.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("play status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	mu.Lock()
	capturedURL := playedURL
	capturedOptions := playedOptions
	mu.Unlock()
	if capturedOptions.VideoID != 42 || capturedOptions.StartTimeSec != 12.5 || !capturedOptions.EnableNetworkThumbnail {
		t.Fatalf("play options = %#v", capturedOptions)
	}
	parsed, err := url.Parse(capturedURL)
	if err != nil || !strings.HasPrefix(parsed.Path, "/__client/media/") {
		t.Fatalf("played URL = %q, err = %v", capturedURL, err)
	}

	mediaRequest := httptest.NewRequest(http.MethodGet, "http://127.0.0.1"+parsed.Path, nil)
	mediaRequest.Header.Set("Range", "bytes=2-5")
	mediaRecorder := httptest.NewRecorder()
	client.ServeHTTP(mediaRecorder, mediaRequest)
	if mediaRecorder.Code != http.StatusPartialContent || mediaRecorder.Body.String() != "cdef" {
		t.Fatalf("media response = %d %q", mediaRecorder.Code, mediaRecorder.Body.String())
	}
	if got := mediaRecorder.Header().Get("Content-Range"); got != "bytes 2-5/6" {
		t.Fatalf("Content-Range = %q", got)
	}
	select {
	case <-playCount:
	case <-time.After(2 * time.Second):
		t.Fatal("remote play count was not incremented")
	}
}

func TestClientUploadsMPVScreenshotAndRetriesFailure(t *testing.T) {
	var uploadAttempts atomic.Int32
	uploaded := make(chan []byte, 1)
	playCount := make(chan struct{}, 1)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "remote_session=secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/videos/42/stream" && r.Header.Get("Range") == "bytes=0-0":
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte{0})
		case r.Method == http.MethodPost && r.URL.Path == "/videos/42/play":
			playCount <- struct{}{}
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPut && r.URL.Path == "/videos/42/screenshots/mpv_00-00-12.500.jpg":
			body, _ := io.ReadAll(r.Body)
			if uploadAttempts.Add(1) == 1 {
				http.Error(w, "temporary failure", http.StatusServiceUnavailable)
				return
			}
			uploaded <- body
			w.WriteHeader(http.StatusCreated)
		default:
			http.NotFound(w, r)
		}
	}))
	defer remote.Close()

	var options mpv.PlayOptions
	client := newTestClient(t, remote.URL, func(_ string, received mpv.PlayOptions) error {
		options = received
		return nil
	})
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/videos/play", strings.NewReader(`{"video_id":42,"location_id":7}`))
	request.Header.Set("Cookie", "remote_session=secret")
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	recorder := httptest.NewRecorder()
	client.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("play status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if filepath.Base(options.DataDir) != remoteStorageKey(remote.URL) {
		t.Fatalf("client data dir %q is not isolated for remote %q", options.DataDir, remote.URL)
	}

	screenshotDir := filepath.Join(options.DataDir, "video", "42", "screenshot")
	if err := os.MkdirAll(screenshotDir, 0o755); err != nil {
		t.Fatalf("create screenshot dir: %v", err)
	}
	screenshotPath := filepath.Join(screenshotDir, "mpv_00-00-12.500.jpg")
	want := []byte{0xff, 0xd8, 0xff, 0xe0, 1, 2, 3, 4}
	if err := os.WriteFile(screenshotPath, want, 0o644); err != nil {
		t.Fatalf("write client screenshot: %v", err)
	}

	select {
	case got := <-uploaded:
		if !bytes.Equal(got, want) {
			t.Fatalf("uploaded screenshot = %v, want %v", got, want)
		}
	case <-time.After(4 * time.Second):
		t.Fatalf("screenshot was not uploaded after retry; attempts=%d", uploadAttempts.Load())
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		_, err := os.Stat(screenshotPath)
		if os.IsNotExist(err) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("uploaded screenshot was not removed, stat error=%v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	select {
	case <-playCount:
	case <-time.After(time.Second):
		t.Fatal("remote play count was not incremented")
	}
}

func TestClientConfigKeepsPlayerSettingsLocal(t *testing.T) {
	remotePatches := make(chan map[string]json.RawMessage, 1)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "remote_session=secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]string{"default_player": "system", "video_page_size": "25", "desktop_integration_enabled": "true"})
		case http.MethodPatch:
			var payload map[string]json.RawMessage
			_ = json.NewDecoder(r.Body).Decode(&payload)
			remotePatches <- payload
			_ = json.NewEncoder(w).Encode(map[string]string{"video_page_size": "50"})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	defer remote.Close()

	client := newTestClient(t, remote.URL, func(string, mpv.PlayOptions) error { return nil })
	request := httptest.NewRequest(http.MethodPatch, "http://127.0.0.1/config", strings.NewReader(`{"player_volume":88,"default_player":"mpv","video_page_size":50}`))
	request.Header.Set("Cookie", "remote_session=secret")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	recorder := httptest.NewRecorder()
	client.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("config status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var config map[string]string
	if err := json.NewDecoder(recorder.Body).Decode(&config); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if config["player_volume"] != "88" || config["default_player"] != "mpv" || config["desktop_integration_enabled"] != "false" {
		t.Fatalf("merged config = %#v", config)
	}
	patch := <-remotePatches
	if _, exists := patch["player_volume"]; exists {
		t.Fatalf("local player setting leaked to remote patch: %#v", patch)
	}
	if _, exists := patch["video_page_size"]; !exists {
		t.Fatalf("remote setting missing from patch: %#v", patch)
	}
	data, err := os.ReadFile(filepath.Join(client.baseDir, "data", localSettingsFilename))
	if err != nil || !strings.Contains(string(data), `"player_volume": "88"`) {
		t.Fatalf("saved local settings = %q, err = %v", data, err)
	}
}

func TestClientLocalConfigUpdateDoesNotRequireAnotherRemoteRequest(t *testing.T) {
	var remoteRequests atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteRequests.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/config" {
			http.Error(w, "unexpected request", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"video_page_size": "25"})
	}))
	defer remote.Close()

	client := newTestClient(t, remote.URL, nil)
	getRequest := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/config", nil)
	getRecorder := httptest.NewRecorder()
	client.ServeHTTP(getRecorder, getRequest)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("initial config status = %d, body = %s", getRecorder.Code, getRecorder.Body.String())
	}

	patchRequest := httptest.NewRequest(http.MethodPatch, "http://127.0.0.1/config", strings.NewReader(`{"default_player":"browser"}`))
	patchRequest.Header.Set("Content-Type", "application/json")
	patchRequest.Header.Set("Sec-Fetch-Site", "same-origin")
	patchRecorder := httptest.NewRecorder()
	client.ServeHTTP(patchRecorder, patchRequest)
	if patchRecorder.Code != http.StatusOK {
		t.Fatalf("local config status = %d, body = %s", patchRecorder.Code, patchRecorder.Body.String())
	}
	var config map[string]string
	if err := json.NewDecoder(patchRecorder.Body).Decode(&config); err != nil {
		t.Fatalf("decode local config response: %v", err)
	}
	if config["default_player"] != "browser" || config["video_page_size"] != "25" {
		t.Fatalf("merged local config = %#v", config)
	}
	if got := remoteRequests.Load(); got != 1 {
		t.Fatalf("remote request count = %d, want 1", got)
	}
}

func TestRemoteRequestKeepsRewrittenBodyLength(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		if string(body) != `{"default_player":"browser"}` {
			t.Errorf("remote body = %q", body)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer remote.Close()

	client := newTestClient(t, remote.URL, nil)
	body := []byte(`{"default_player":"browser"}`)
	headers := make(http.Header)
	headers.Set("Content-Length", "999")
	response, err := client.remoteRequest(context.Background(), http.MethodPatch, "/config", "", bytes.NewReader(body), headers)
	if err != nil {
		t.Fatalf("remoteRequest returned error: %v", err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("remote status = %d", response.StatusCode)
	}
}

func TestMergeCookieHeaderRefreshesRemoteSession(t *testing.T) {
	got := mergeCookieHeader("other=value; remote_session=old", []*http.Cookie{
		{Name: "remote_session", Value: "new"},
		{Name: "removed", Value: "", MaxAge: -1},
	})
	if got != "other=value; remote_session=new" {
		t.Fatalf("merged cookie header = %q", got)
	}
}

func TestAuthenticatedResponseResumesPendingScreenshotAfterRestart(t *testing.T) {
	uploaded := make(chan []byte, 1)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/auth/status":
			http.SetCookie(w, &http.Cookie{Name: "remote_session", Value: "renewed", Path: "/", HttpOnly: true})
			_, _ = io.WriteString(w, `{"authenticated":true}`)
		case r.Method == http.MethodPut && r.URL.Path == "/videos/9/screenshots/mpv_00-00-09.jpg":
			if r.Header.Get("Cookie") != "remote_session=renewed" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			body, _ := io.ReadAll(r.Body)
			uploaded <- body
			w.WriteHeader(http.StatusCreated)
		default:
			http.NotFound(w, r)
		}
	}))
	defer remote.Close()
	client := newTestClient(t, remote.URL, func(string, mpv.PlayOptions) error { return nil })
	pendingDir := filepath.Join(client.clientDataDir(), "video", "9", "screenshot")
	if err := os.MkdirAll(pendingDir, 0o755); err != nil {
		t.Fatalf("create pending screenshot dir: %v", err)
	}
	want := []byte{0xff, 0xd8, 0xff, 1, 2, 3}
	if err := os.WriteFile(filepath.Join(pendingDir, "mpv_00-00-09.jpg"), want, 0o644); err != nil {
		t.Fatalf("write pending screenshot: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/auth/status", nil)
	recorder := httptest.NewRecorder()
	client.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("auth status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	select {
	case got := <-uploaded:
		if !bytes.Equal(got, want) {
			t.Fatalf("resumed screenshot = %v, want %v", got, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("pending screenshot was not resumed after authenticated response")
	}
}

func newTestClient(t *testing.T, remoteURL string, play func(string, mpv.PlayOptions) error) *Client {
	t.Helper()
	client, err := New(Options{
		BaseDir:      t.TempDir(),
		LocalBaseURL: "http://127.0.0.1",
		RemoteURL:    remoteURL,
		PlayVideo:    play,
	})
	if err != nil {
		t.Fatalf("New client: %v", err)
	}
	t.Cleanup(client.Close)
	return client
}
