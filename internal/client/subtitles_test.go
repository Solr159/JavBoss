package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"javboss/internal/mpv"
)

func TestClientSubtitleFetchUsesAuthenticationAndLocalFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/videos/12/subtitles/resolve" || r.Header.Get("Cookie") != "session=test" {
			t.Error("wrong subtitle request")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["manual"] != true {
			t.Error("manual flag missing", body, err)
		}
		w.Header().Set("X-Subtitle-Language", "zh-CN")
		w.Header().Set("X-Subtitle-Source", "test")
		w.Header().Set("X-Subtitle-Offset-Ms", "30500")
		w.Header().Set("X-Subtitle-Downloaded", "true")
		w.Header().Set("X-Subtitle-Key", "download:zh-CN")
		w.Header().Set("X-Subtitle-Track", "local:%E4%B8%AD%E6%96%87.srt")
		w.Header().Set("Content-Disposition", `attachment; filename="ABC-123.zh-CN.srt"`)
		fmt.Fprint(w, "1\n00:00:01,000 --> 00:00:02,000\nHello\n")
	}))
	defer server.Close()
	c := newTestClient(t, server.URL, nil)
	result, err := c.subtitleResolver(12, 3, "session=test")(context.Background(), mpv.SubtitleRequest{Manual: true})
	if err != nil || result == nil {
		t.Fatalf("%v %v", result, err)
	}
	if result.Path != filepath.Join(c.clientDataDir(), "subtitles", "ABC-123.zh-CN.srt") {
		t.Fatalf("unexpected cache path: %s", result.Path)
	}
	data, err := os.ReadFile(result.Path)
	if err != nil || !strings.Contains(string(data), "00:00:01,000") || result.Language != "zh-CN" || result.OffsetSec != 30.5 || !result.Downloaded || result.Key != "download:zh-CN" || result.Track != "local:中文.srt" {
		t.Fatalf("%q %v", data, err)
	}
}

func TestClientExistingSubtitleFetchesOnlyDelay(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			SettingsOnly bool  `json:"settings_only"`
			LocationID   int64 `json:"location_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !req.SettingsOnly || req.LocationID != 3 {
			t.Error("missing settings-only request", err)
		}
		w.Header().Set("X-Subtitle-Status", "disabled")
		w.Header().Set("X-Subtitle-Offset-Ms", "-30500")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	c := newTestClient(t, server.URL, nil)
	result, err := c.subtitleResolver(12, 3, "session=test")(context.Background(), mpv.SubtitleRequest{SettingsOnly: true})
	if err != nil || result == nil || result.Path != "" || result.OffsetSec != -30.5 || result.Status != "disabled" {
		t.Fatalf("%+v %v", result, err)
	}
}

func TestClientRejectsSubtitlePathFromServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="../outside.srt"`)
		fmt.Fprint(w, "1\n00:00:01,000 --> 00:00:02,000\nHello\n")
	}))
	defer server.Close()
	c := newTestClient(t, server.URL, nil)
	if _, err := c.subtitleResolver(12, 3, "session=test")(context.Background(), mpv.SubtitleRequest{}); err == nil {
		t.Fatal("accepted a path instead of a filename")
	}
}

func TestClientSubtitleOffsetSave(t *testing.T) {
	status := http.StatusOK
	wantOffset := int64(12346)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" || r.URL.Path != "/videos/12/subtitle-settings" || r.Header.Get("Cookie") != "session=test" {
			t.Error("wrong settings save request")
		}
		var body struct {
			OffsetMS int64 `json:"offset_ms"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.OffsetMS != wantOffset {
			t.Error("wrong offset", body, err)
		}
		w.WriteHeader(status)
	}))
	defer server.Close()
	c := newTestClient(t, server.URL, nil)
	save := c.subtitleOffsetSaver(12, "session=test")
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("save sent to a different server after switching connections")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer other.Close()
	if err := c.setRemoteURL(other.URL); err != nil {
		t.Fatal(err)
	}
	if err := save(context.Background(), 12346); err != nil {
		t.Fatal(err)
	}
	wantOffset = -12346
	if err := save(context.Background(), wantOffset); err != nil {
		t.Fatal(err)
	}
	wantOffset = 12346
	status = http.StatusInternalServerError
	if err := save(context.Background(), 12346); err == nil {
		t.Fatal("save failure ignored")
	}
	if err := save(context.Background(), -3600001); err == nil {
		t.Fatal("out-of-range offset accepted")
	}
}

func TestClientSubtitleSelectionSave(t *testing.T) {
	want := "off"
	status := http.StatusOK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" || r.URL.Path != "/videos/12/subtitle-settings" || r.Header.Get("Cookie") != "session=test" {
			t.Error("wrong settings save request")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body) != 1 || body["track"] != want {
			t.Error("wrong visibility patch", body, err)
		}
		w.WriteHeader(status)
	}))
	defer server.Close()
	c := newTestClient(t, server.URL, nil)
	save := c.subtitleSelectionSaver(12, "session=test")
	if err := c.setRemoteURL("http://127.0.0.1:1"); err != nil {
		t.Fatal(err)
	}
	for _, track := range []string{"off", "download:zh-CN"} {
		want = track
		if err := save(context.Background(), track); err != nil {
			t.Fatal(err)
		}
	}
	status = http.StatusInternalServerError
	if err := save(context.Background(), "download:zh-CN"); err == nil {
		t.Fatal("save failure ignored")
	}
}

func TestLocalAndDownloadedSubtitlesWithSameFilenameStaySeparate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Manual bool `json:"manual"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		source := "local"
		if body.Manual {
			source = "subtitlecat"
		}
		w.Header().Set("X-Subtitle-Source", source)
		w.Header().Set("Content-Disposition", `attachment; filename="ABC-123.zh-CN.srt"`)
		fmt.Fprintf(w, "1\n00:00:01,000 --> 00:00:02,000\n%s\n", source)
	}))
	defer server.Close()
	c := newTestClient(t, server.URL, nil)
	resolve := c.subtitleResolver(12, 3, "session=test")
	local, err := resolve(context.Background(), mpv.SubtitleRequest{})
	if err != nil {
		t.Fatal(err)
	}
	downloaded, err := resolve(context.Background(), mpv.SubtitleRequest{Manual: true})
	if err != nil {
		t.Fatal(err)
	}
	if local.Path == downloaded.Path {
		t.Fatal("local and downloaded tracks share a file")
	}
	data, err := os.ReadFile(local.Path)
	if err != nil || !strings.Contains(string(data), "local") {
		t.Fatal("local subtitle overwritten", err)
	}
}
