package api

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"javboss/internal/common"
	dbpkg "javboss/internal/db"
	"javboss/internal/models"
	"javboss/internal/mpv"
	"javboss/internal/subtitle"
)

type apiSubtitleProvider struct{ codes []string }

func (p *apiSubtitleProvider) ID() string { return "test" }
func (p *apiSubtitleProvider) Search(_ context.Context, q subtitle.Query) ([]subtitle.Candidate, error) {
	p.codes = append(p.codes, q.Code)
	return []subtitle.Candidate{{ID: q.Code, Code: q.Code}}, nil
}
func (p *apiSubtitleProvider) Tracks(context.Context, subtitle.Candidate) ([]subtitle.Track, error) {
	return []subtitle.Track{{ID: "file", Language: "zh-CN", Format: "srt"}}, nil
}
func (p *apiSubtitleProvider) Download(context.Context, subtitle.Track) ([]byte, error) {
	return []byte("1\n00:00:01,000 --> 00:00:02,000\nTest\n"), nil
}

func TestSubtitleEndpointSettingsLocationAttributionAndCache(t *testing.T) {
	root := t.TempDir()
	deps := newTestDependencies()
	database, err := dbpkg.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sql, _ := database.DB(); sql.Close() })
	deps.database = database
	deps.config = &common.Config{DatabasePath: filepath.Join(root, "test.db")}
	deps.bindDatabase(t)
	provider := &apiSubtitleProvider{}
	previous := subtitleService
	subtitleService = subtitle.NewService(provider)
	t.Cleanup(func() { subtitleService = previous })
	dir := models.Directory{Path: root}
	video := models.Video{Fingerprint: "content"}
	a, b := models.Jav{Code: "ABC-123"}, models.Jav{Code: "DEF-456"}
	for _, row := range []any{&dir, &video, &a, &b} {
		if err := database.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	first := models.VideoLocation{VideoID: video.ID, DirectoryID: dir.ID, RelativePath: "first.mp4", Filename: "first.mp4", JavID: &a.ID}
	second := models.VideoLocation{VideoID: video.ID, DirectoryID: dir.ID, RelativePath: "second.mp4", Filename: "second.mp4", JavID: &b.ID}
	for _, row := range []any{&first, &second} {
		if err := database.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	router := gin.New()
	router.POST("/videos/:id/subtitles/resolve", resolveSubtitle)
	router.PATCH("/videos/:id/subtitle-settings", videoSubtitleSettings)
	router.GET("/videos/:id/subtitle-settings", videoSubtitleSettings)
	router.PATCH("/config", updateConfig)
	call := func(path, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		method := "POST"
		if path == "/config" || strings.HasSuffix(path, "/subtitle-settings") {
			method = "PATCH"
		}
		if body == "" {
			method = "GET"
		}
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		return w
	}
	path := fmt.Sprintf("/videos/%d/subtitles/resolve", video.ID)
	if w := call(path, `{}`, 204); w.Header().Get("X-Subtitle-Status") != "disabled" {
		t.Fatal(w.Header())
	}
	if file, err := localSubtitleResolver(video.ID, second.ID, "")(context.Background(), mpv.SubtitleRequest{SettingsOnly: true}); err != nil || file == nil || file.Status != "disabled" {
		t.Fatalf("MPV disabled status: %+v %v", file, err)
	}
	if len(provider.codes) != 0 {
		t.Fatal("disabled feature contacted provider")
	}
	call("/config", `{"subtitle_auto_download":true,"subtitle_language":"invalid"}`, 400)
	call("/config", `{"subtitle_auto_download":true,"subtitle_language":"zh-CN"}`, 200)
	// A recognizable filename must not substitute for an associated JAV code.
	unlinked := models.VideoLocation{VideoID: video.ID, DirectoryID: dir.ID, RelativePath: "XYZ-789.mp4", Filename: "XYZ-789.mp4"}
	if err := database.Create(&unlinked).Error; err != nil {
		t.Fatal(err)
	}
	if w := call(path, fmt.Sprintf(`{"location_id":%d}`, unlinked.ID), 204); w.Header().Get("X-Subtitle-Status") != "ambiguous" {
		t.Fatal(w.Header())
	}
	if file, err := localSubtitleResolver(video.ID, unlinked.ID, "")(context.Background(), mpv.SubtitleRequest{}); err != nil || file == nil || file.Status != "ambiguous" {
		t.Fatalf("MPV ambiguous status: %+v %v", file, err)
	}
	if len(provider.codes) != 0 {
		t.Fatal("unlinked video contacted provider using its filename")
	}
	body := fmt.Sprintf(`{"location_id":%d,"format":"vtt"}`, second.ID)
	w := call(path, body, 200)
	if w.Header().Get("X-Subtitle-Downloaded") != "true" || !strings.HasPrefix(w.Body.String(), "WEBVTT") || w.Header().Get("X-Subtitle-Source") != "download" {
		t.Fatal(w.Body.String())
	}
	if disposition := w.Header().Get("Content-Disposition"); !strings.Contains(disposition, "DEF-456.zh-CN.vtt") {
		t.Fatalf("unexpected filename: %s", disposition)
	}
	if cached := call(path, body, 200); cached.Header().Get("X-Subtitle-Downloaded") != "false" {
		t.Fatal("cache marked as fresh download")
	}
	if len(provider.codes) != 1 || provider.codes[0] != "DEF-456" {
		t.Fatalf("wrong location or cache: %v", provider.codes)
	}
	call(path, `{}`, 200)
	if len(provider.codes) != 2 || provider.codes[1] != "ABC-123" {
		t.Fatalf("primary JAV not hydrated: %v", provider.codes)
	}
	copyVideo := models.Video{Fingerprint: "different-edit"}
	if err := database.Create(&copyVideo).Error; err != nil {
		t.Fatal(err)
	}
	copyLocation := models.VideoLocation{VideoID: copyVideo.ID, DirectoryID: dir.ID, RelativePath: "another.mp4", Filename: "another.mp4", JavID: &b.ID}
	if err := database.Create(&copyLocation).Error; err != nil {
		t.Fatal(err)
	}
	w = call(fmt.Sprintf("/videos/%d/subtitles/resolve", copyVideo.ID), `{}`, 200)
	if len(provider.codes) != 2 || !strings.Contains(w.Header().Get("Content-Disposition"), "DEF-456.zh-CN.srt") {
		t.Fatal("different videos did not share code/language cache")
	}
	settingsPath := fmt.Sprintf("/videos/%d/subtitle-settings", video.ID)
	if err := localSubtitleOffsetSaver(video.ID)(context.Background(), 30500); err != nil {
		t.Fatal(err)
	}
	if err := localSubtitleOffsetSaver(999999)(context.Background(), 30500); err == nil {
		t.Fatal("missing video save accepted")
	}
	if w := call(settingsPath, "", 200); !strings.Contains(w.Body.String(), `"offset_ms":30500`) {
		t.Fatal("saved duration not returned", w.Body.String())
	}
	if w := call(settingsPath, "", 200); !strings.Contains(w.Body.String(), `"track":""`) {
		t.Fatal("unsaved preference must remain unset", w.Body.String())
	}
	call(settingsPath, `{"track":"off"}`, 200)
	if w := call(settingsPath, "", 200); !strings.Contains(w.Body.String(), `"track":"off"`) || !strings.Contains(w.Body.String(), `"offset_ms":30500`) {
		t.Fatal("visibility save changed offset or lost false", w.Body.String())
	}
	// Scanner writes from a stale model must preserve the user's adjustment.
	if err := dbpkg.Default().SaveVideo(context.Background(), &video); err != nil {
		t.Fatal(err)
	}
	w = call(path, body, 200)
	if w.Header().Get("X-Subtitle-Track") != "off" || w.Header().Get("X-Subtitle-Offset-Ms") != "30500" || !strings.Contains(w.Body.String(), "00:00:31.500 --> 00:00:32.500") {
		t.Fatal("browser subtitle not delayed", w.Header(), w.Body.String())
	}
	w = call(fmt.Sprintf("/videos/%d/subtitles/resolve", copyVideo.ID), `{"format":"vtt"}`, 200)
	if !strings.Contains(w.Body.String(), "00:00:01.000 --> 00:00:02.000") {
		t.Fatal("delay leaked into another edit")
	}
	w = call(path, `{}`, 200)
	if !strings.Contains(w.Body.String(), "00:00:01,000 --> 00:00:02,000") {
		t.Fatal("shared SRT changed")
	}
	file, err := localSubtitleResolver(video.ID, second.ID, "")(context.Background(), mpv.SubtitleRequest{SettingsOnly: true})
	if err != nil || file == nil || file.Path != "" || file.OffsetSec != 30.5 || file.Track != "off" {
		t.Fatalf("existing MPV track settings: %+v %v", file, err)
	}
	file, err = localSubtitleResolver(video.ID, second.ID, "")(context.Background(), mpv.SubtitleRequest{})
	if err != nil || file == nil || file.Path == "" || file.OffsetSec != 30.5 || file.Status != "ready" {
		t.Fatalf("downloaded MPV settings: %+v %v", file, err)
	}
	for _, invalid := range []string{`{"track":null}`, `{"track":false}`, `{"track":0}`, `{}`, `{"offset_ms":null}`, `{"offset_ms":-3600001}`, `{"offset_ms":3600001}`, `{"offset_ms":1.5}`, `{"offset_ms":"30"}`} {
		call(settingsPath, invalid, 400)
	}
	call("/videos/999999/subtitle-settings", `{"offset_ms":0}`, 404)
	call("/videos/999999/subtitle-settings", "", 404)
	call("/videos/bad/subtitle-settings", "", 400)
	w = call(path, `{"settings_only":true}`, 204)
	if w.Header().Get("X-Subtitle-Offset-Ms") != "30500" {
		t.Fatal("settings-only response lost delay")
	}
	call(settingsPath, `{"offset_ms":-1500}`, 200)
	if w := call(settingsPath, "", 200); !strings.Contains(w.Body.String(), `"track":"off"`) || !strings.Contains(w.Body.String(), `"offset_ms":-1500`) {
		t.Fatal("negative offset not persisted", w.Body.String())
	}
	w = call(path, body, 200)
	if w.Header().Get("X-Subtitle-Offset-Ms") != "-1500" || !strings.Contains(w.Body.String(), "00:00:00.000 --> 00:00:00.500") {
		t.Fatal("negative browser offset not applied", w.Header(), w.Body.String())
	}
	file, err = localSubtitleResolver(video.ID, second.ID, "")(context.Background(), mpv.SubtitleRequest{})
	if err != nil || file == nil || file.OffsetSec != -1.5 {
		t.Fatalf("negative MPV offset: %+v %v", file, err)
	}
	if err := localSubtitleSelectionSaver(video.ID)(context.Background(), "download:zh-CN"); err != nil {
		t.Fatal(err)
	}
	if w := call(settingsPath, "", 200); !strings.Contains(w.Body.String(), `"track":"download:zh-CN"`) || !strings.Contains(w.Body.String(), `"offset_ms":-1500`) {
		t.Fatal("MPV visibility save lost settings", w.Body.String())
	}
	if w := call(fmt.Sprintf("/videos/%d/subtitle-settings", copyVideo.ID), "", 200); !strings.Contains(w.Body.String(), `"track":""`) {
		t.Fatal("visibility leaked into another video", w.Body.String())
	}
	call(settingsPath, `{"offset_ms":0}`, 200)
	w = call(path, body, 200)
	if !strings.Contains(w.Body.String(), "00:00:01.000 --> 00:00:02.000") {
		t.Fatal("reset did not restore original timing")
	}

	// Disabling downloads still loads cache; only an explicit manual miss may go online.
	call("/config", `{"subtitle_auto_download":false}`, 200)
	if w := call(path, `{"settings_only":true}`, 204); w.Header().Get("X-Subtitle-Status") != "disabled" {
		t.Fatal(w.Header())
	}
	before := len(provider.codes)
	call(path, body, 200)
	// A cached alternative language beats a new download for the preferred one.
	call("/config", `{"subtitle_language":"zh-TW"}`, 200)
	call(path, body, 200)
	if len(provider.codes) != before {
		t.Fatal("cached playback contacted provider")
	}
	call("/config", `{"subtitle_language":"zh-CN"}`, 200)
	if file, err := localSubtitleResolver(video.ID, second.ID, "")(context.Background(), mpv.SubtitleRequest{}); err != nil || file == nil || file.Path == "" {
		t.Fatalf("automatic cached MPV load: %+v %v", file, err)
	}
	if err := os.Remove(filepath.Join(root, "subtitles", "DEF-456.zh-CN.srt")); err != nil {
		t.Fatal(err)
	}
	call(path, body, 204)
	if len(provider.codes) != before {
		t.Fatal("disabled download contacted provider on cache miss")
	}
	call(path, fmt.Sprintf(`{"location_id":%d,"format":"vtt","manual":true}`, second.ID), 200)
	if file, err := localSubtitleResolver(video.ID, second.ID, "")(context.Background(), mpv.SubtitleRequest{Manual: true}); err != nil || file == nil || file.Path == "" || file.Status != "ready" {
		t.Fatalf("manual MPV load: %+v %v", file, err)
	}
	call("/config", `{"subtitle_auto_download":true}`, 200)
	legacy := fmt.Sprintf(`{"path":"second.mp4","dir_path":%q}`, root)
	call(path, legacy, 200)
	if len(provider.codes) != before+1 {
		t.Fatal("legacy path resolved wrong video location")
	}
	call(path, `{"location_id":999999}`, 404)
	call(path, `{"format":"html"}`, 400)
	if err := os.WriteFile(filepath.Join(root, "second.zh-cn.SRT"), []byte("1\n00:00:01,000 --> 00:00:02,000\nLocal\n"), 0600); err != nil {
		t.Fatal(err)
	}
	call("/config", `{"subtitle_auto_download":false}`, 200)
	w = call(path, body, 200)
	if w.Header().Get("X-Subtitle-Source") != "local" || !strings.Contains(w.Body.String(), "Local") {
		t.Fatal("sidecar not preferred")
	}
	if file, err := localSubtitleResolver(video.ID, second.ID, "")(context.Background(), mpv.SubtitleRequest{}); err != nil || file == nil || file.Title != "local" {
		t.Fatalf("automatic sidecar MPV load: %+v %v", file, err)
	}
	if len(provider.codes) != before+1 {
		t.Fatal("sidecar playback contacted provider")
	}
	manual := fmt.Sprintf(`{"location_id":%d,"format":"vtt","manual":true}`, second.ID)
	w = call(path, manual, 200)
	if w.Header().Get("X-Subtitle-Source") != "download" || strings.Contains(w.Body.String(), "Local") || len(provider.codes) != before+1 {
		t.Fatal("manual loading must add the cached download alongside the sidecar")
	}
	if file, err := localSubtitleResolver(video.ID, second.ID, "")(context.Background(), mpv.SubtitleRequest{Manual: true}); err != nil || file == nil || file.Title != "download" {
		t.Fatalf("manual MPV lookup must bypass sidecar: %+v %v", file, err)
	}
	if err := os.Remove(filepath.Join(root, "subtitles", "DEF-456.zh-CN.srt")); err != nil {
		t.Fatal(err)
	}
	w = call(path, manual, 200)
	if w.Header().Get("X-Subtitle-Downloaded") != "true" || len(provider.codes) != before+2 {
		t.Fatal("manual download blocked by local sidecar")
	}
	w = call(path, body, 200)
	if w.Header().Get("X-Subtitle-Source") != "local" {
		t.Fatal("automatic playback lost local priority")
	}
	cacheOnly := fmt.Sprintf(`{"location_id":%d,"download_only":true,"cache_only":true}`, second.ID)
	w = call(path, cacheOnly, 200)
	if w.Header().Get("X-Subtitle-Source") != "download" || w.Header().Get("X-Subtitle-Key") != "download:zh-CN" {
		t.Fatal("automatic cache lookup returned sidecar", w.Header())
	}
	if err := os.Remove(filepath.Join(root, "subtitles", "DEF-456.zh-CN.srt")); err != nil {
		t.Fatal(err)
	}
	call("/config", `{"subtitle_auto_download":true}`, 200)
	lookups := len(provider.codes)
	call(path, cacheOnly, 204)
	if len(provider.codes) != lookups {
		t.Fatal("cache-only lookup contacted provider")
	}
	call(settingsPath, `{"track":"local:second.zh-cn.SRT"}`, 200)
	call(settingsPath, `{"offset_ms":1000}`, 200)
	if w := call(settingsPath, "", 200); !strings.Contains(w.Body.String(), `"track":"local:second.zh-cn.SRT"`) {
		t.Fatal("offset save lost selection")
	}
	call(settingsPath, `{"track":12}`, 400)
	call(settingsPath, fmt.Sprintf(`{"track":%q}`, strings.Repeat("x", 1025)), 400)
}
