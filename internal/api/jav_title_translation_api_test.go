package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	dbpkg "javboss/internal/db"
	"javboss/internal/jav/translation"
	"javboss/internal/models"

	"github.com/gin-gonic/gin"
)

type translationTestTransport func(*http.Request) (*http.Response, error)

func (f translationTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func titleTranslationRouter(deps *testDependencies, t *testing.T, handler http.HandlerFunc) *gin.Engine {
	t.Helper()
	testAuthService(deps, t)
	upstream := httptest.NewServer(handler)
	t.Cleanup(upstream.Close)

	ctx, cancel := context.WithCancel(context.Background())
	translation.Init(ctx)
	target, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := upstream.Client()
	transport := client.Transport
	// Keep saved profiles intact while routing every request locally.
	client.Transport = translationTestTransport(func(req *http.Request) (*http.Response, error) {
		local := req.Clone(req.Context())
		local.URL.Scheme, local.URL.Host = target.Scheme, target.Host
		local.Host = target.Host
		return transport.RoundTrip(local)
	})
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	previousHTTP := translation.SetHTTPClient(client)
	t.Cleanup(func() {
		cancel()
		waitTitleTranslationBatch(t)
		waitTextTranslationBatch(t, true)
		translation.WaitBatches()
		translation.SetHTTPClient(previousHTTP)
	})
	router := gin.New()
	RegisterRoutes(router)
	return router
}

func translationAPIRequest(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	return response
}

func TestTitleTranslationWaitsForReasoningBeyondServerWriteTimeout(t *testing.T) {
	deps := newTestDependencies()
	router := titleTranslationRouter(deps, t, func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Thinking struct{ Type string } `json:"thinking"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Thinking.Type != "enabled" {
			t.Errorf("enabled reasoning request=%+v err=%v", payload, err)
		}
		time.Sleep(80 * time.Millisecond)
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"reasoning_content":"internal reasoning","content":"雨天相遇"}}]}`))
	})
	if err := dbpkg.Default().UpsertConfig(context.Background(), map[string]string{
		"jav_title_translation_enabled": "true", translation.ProfilesKey: `{"deepseek":{"api_key":"key","model":"model","thinking":true}}`,
	}); err != nil {
		t.Fatal(err)
	}
	item := models.Jav{Code: "REASON-001", Title: "雨の日の出会い"}
	if err := deps.database.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(router)
	server.Config.WriteTimeout = 20 * time.Millisecond
	server.Start()
	defer server.Close()
	path := server.URL + "/jav/items/" + strconv.FormatInt(item.ID, 10) + "/title-translation"
	response, err := server.Client().Post(path, "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || body["title"] != item.Title || body["zh_title"] != "雨天相遇" || body["reasoning_content"] != nil {
		t.Fatalf("status=%d body=%+v", response.StatusCode, body)
	}
}

func TestTitleTranslationConfigAndModelDiscoveryKeepKeyPrivate(t *testing.T) {
	deps := newTestDependencies()
	var receivedKey string
	router := titleTranslationRouter(deps, t, func(w http.ResponseWriter, r *http.Request) {
		receivedKey = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"data":[{"id":"official-new-model","name":"Official new model"}]}`))
	})
	defaults := translationAPIRequest(router, "GET", "/config", "")
	var defaultConfig map[string]string
	if err := json.Unmarshal(defaults.Body.Bytes(), &defaultConfig); err != nil {
		t.Fatal(err)
	}
	if defaults.Code != http.StatusOK || defaultConfig[translation.ProviderKey] != "deepseek" || defaultConfig[translation.ProfilesKey] != "{}" || defaultConfig["jav_title_translation_prompt"] != translation.DefaultPrompt {
		t.Fatalf("default settings=%d %+v", defaults.Code, defaultConfig)
	}
	body := `{"jav_title_translation_enabled":true,"jav_title_translation_profiles":{"deepseek":{"thinking":false,"api_key":"private-key","model":"official-new-model"}},"jav_title_translation_prompt":"自定义提示词"}`
	response := translationAPIRequest(router, "PATCH", "/config", body)
	if response.Code != 200 || bytes.Contains(response.Body.Bytes(), []byte("private-key")) {
		t.Fatalf("config response: %d %s", response.Code, response.Body)
	}
	var cfg map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &cfg); err != nil {
		t.Fatal(err)
	}
	if p := translation.ActiveProfile(cfg); !p.APIKeySet || p.APIKey != "" || p.Thinking || p.Model != "official-new-model" || cfg["jav_title_translation_enabled"] != "true" {
		t.Fatalf("config=%v", cfg)
	}
	response = translationAPIRequest(router, "GET", "/config", "")
	if response.Code != 200 || bytes.Contains(response.Body.Bytes(), []byte("private-key")) {
		t.Fatal("GET leaked key")
	}
	response = translationAPIRequest(router, "POST", "/jav/title-translation/models", `{"profile":{"api_key":"draft-key"}}`)
	if response.Code != 200 || receivedKey != "Bearer draft-key" || !strings.Contains(response.Body.String(), "official-new-model") {
		t.Fatalf("model discovery=%d %s", response.Code, response.Body)
	}
	response = translationAPIRequest(router, "POST", "/jav/title-translation/models", `{}`)
	if response.Code != 200 || receivedKey != "Bearer private-key" {
		t.Fatal("did not reuse saved profile key")
	}
	receivedKey = ""
	response = translationAPIRequest(router, "POST", "/jav/title-translation/models", `{"profile":{"api_key":""}}`)
	if response.Code != http.StatusBadRequest || receivedKey != "" {
		t.Fatal("model discovery reused an explicitly cleared draft key")
	}
	stored, _ := dbpkg.Default().ListConfig(context.Background())
	if p := translation.ActiveProfile(stored); p.APIKey != "private-key" || p.Thinking || p.Model != "official-new-model" {
		t.Fatal("model discovery saved draft key")
	}
	response = translationAPIRequest(router, "PATCH", "/config", `{"jav_title_translation_enabled":false,"jav_title_translation_profiles":{"deepseek":{"api_key":""}}}`)
	if err := json.Unmarshal(response.Body.Bytes(), &cfg); err != nil || response.Code != 200 || translation.ActiveProfile(cfg).APIKeySet {
		t.Fatal("clear key failed")
	}
	response = translationAPIRequest(router, "POST", "/jav/title-translation/models", `{}`)
	if response.Code != http.StatusBadRequest {
		t.Fatal("model discovery reused cleared key")
	}
}

func TestTitleTranslationProviderFailureDoesNotReplaceTitleOrExpireSession(t *testing.T) {
	deps := newTestDependencies()
	router := titleTranslationRouter(deps, t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":"secret-key"}`))
	})
	dbpkg.Default().UpsertConfig(context.Background(), map[string]string{"jav_title_translation_enabled": "true", translation.ProfilesKey: `{"deepseek":{"api_key":"secret-key","model":"model"}}`})
	item := models.Jav{Code: "TRANS-FAIL", Title: "Existing title"}
	deps.database.Create(&item)
	response := translationAPIRequest(router, "POST", "/jav/items/"+strconv.FormatInt(item.ID, 10)+"/title-translation", "")
	if response.Code != 400 || strings.Contains(response.Body.String(), "secret-key") {
		t.Fatalf("failure=%d %s", response.Code, response.Body)
	}
	var stored models.Jav
	deps.database.First(&stored, item.ID)
	if stored.Title != "Existing title" || stored.ZhTitle != "" {
		t.Fatal("failed request replaced title")
	}
}

func TestTitleTranslationManualEditInvalidatesCachedTitle(t *testing.T) {
	deps := newTestDependencies()
	testAuthService(deps, t)
	item := models.Jav{Code: "TRANS-EDIT", Title: "旧译文", ZhTitle: "旧译文"}
	if err := deps.database.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	title := "旧译文"
	updated, err := dbpkg.Default().UpdateJav(context.Background(), item.ID, dbpkg.JavUpdateInput{Title: &title}, nil)
	if err != nil || updated.ZhTitle != "旧译文" {
		t.Fatalf("unchanged edit=%+v err=%v", updated, err)
	}
	title = "New source"
	updated, err = dbpkg.Default().UpdateJav(context.Background(), item.ID, dbpkg.JavUpdateInput{Title: &title}, nil)
	if err != nil || updated.Title != "New source" || updated.ZhTitle != "" {
		t.Fatalf("changed edit=%+v err=%v", updated, err)
	}
	zhTitle := "手动中文标题"
	router := gin.New()
	RegisterRoutes(router)
	response := translationAPIRequest(router, "PUT", "/jav/items/"+strconv.FormatInt(item.ID, 10), `{"zh_title":"手动中文标题"}`)
	if response.Code != 200 || !strings.Contains(response.Body.String(), zhTitle) || !strings.Contains(response.Body.String(), "New source") {
		t.Fatalf("Chinese edit=%d %s", response.Code, response.Body)
	}
}

func TestTitleTranslationRoutesRequireAuthentication(t *testing.T) {
	deps := newTestDependencies()
	router := NewRouter("", testAuthService(deps, t))
	for _, path := range []string{"/jav/title-translation/models", "/jav/title-translation/test", "/jav/items/1/title-translation", "/jav/title-translation/batch", "/jav/title-translation/items", "/jav/series/1/name-translation", "/jav/series-translation/items", "/jav/series-translation/batch"} {
		response := translationAPIRequest(router, "POST", path, `{}`)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("path=%s status=%d", path, response.Code)
		}
	}
	response := translationAPIRequest(router, "GET", "/jav/title-translation/batch", `{"scope":"all","confirm_overwrite":true}`)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("batch status was not protected: %d", response.Code)
	}
}

func waitTitleTranslationBatch(t *testing.T) translation.BatchStatus {
	return waitTextTranslationBatch(t, false)
}

func waitTextTranslationBatch(t *testing.T, series bool) translation.BatchStatus {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		status := translation.GetBatchStatus(series)
		if !status.Running {
			return status
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("batch translation did not stop")
	return translation.BatchStatus{}
}

func TestTitleTranslationBatchRetranslatesAllAndProtectsConcurrentEdits(t *testing.T) {
	deps := newTestDependencies()
	entered, release := make(chan struct{}), make(chan struct{})
	calls := 0
	router := titleTranslationRouter(deps, t, func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Thinking struct{ Type string } `json:"thinking"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Thinking.Type != "disabled" {
			t.Errorf("batch reasoning should be disabled: %+v err=%v", payload, err)
		}
		calls++
		if calls == 1 {
			close(entered)
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"新的中文标题"}}]}`))
	})
	if err := dbpkg.Default().UpsertConfig(context.Background(), map[string]string{
		"jav_title_translation_enabled": "true", translation.ProfilesKey: `{"deepseek":{"api_key":"key","model":"model","thinking":false}}`,
	}); err != nil {
		t.Fatal(err)
	}
	items := []models.Jav{
		{Code: "BATCH-001", Title: "Original one", ZhTitle: "已有译文"},
		{Code: "BATCH-002", Title: "Original two", ZhTitle: "保留手动修改"},
		{Code: "BATCH-003", Title: " "},
	}
	if err := deps.database.Create(&items).Error; err != nil {
		t.Fatal(err)
	}
	response := translationAPIRequest(router, "POST", "/jav/title-translation/batch", `{"scope":"all","confirm_overwrite":true}`)
	if response.Code != http.StatusAccepted {
		t.Fatalf("start=%d %s", response.Code, response.Body)
	}
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("batch did not start")
	}
	duplicate := translationAPIRequest(router, "POST", "/jav/title-translation/batch", `{"scope":"all","confirm_overwrite":true}`)
	if duplicate.Code != http.StatusAccepted || !strings.Contains(duplicate.Body.String(), `"running":true`) {
		t.Fatalf("duplicate=%d %s", duplicate.Code, duplicate.Body)
	}
	if err := deps.database.Model(&items[1]).Update("title", "User edited source").Error; err != nil {
		t.Fatal(err)
	}
	close(release)
	status := waitTitleTranslationBatch(t)
	if status.Total != 2 || status.Processed != 2 || status.Translated != 1 || status.Skipped != 1 || calls != 1 {
		t.Fatalf("status=%+v calls=%d", status, calls)
	}
	var stored models.Jav
	deps.database.First(&stored, items[0].ID)
	if stored.Title != items[0].Title || stored.ZhTitle != "新的中文标题" {
		t.Fatal("batch did not replace the existing Chinese title separately")
	}
	var edited models.Jav
	deps.database.First(&edited, items[1].ID)
	if edited.Title != "User edited source" || edited.ZhTitle != items[1].ZhTitle {
		t.Fatal("batch overwrote a concurrent edit")
	}
	response = translationAPIRequest(router, "GET", "/jav/title-translation/batch", `{"scope":"all","confirm_overwrite":true}`)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"processed":2`) {
		t.Fatalf("status response=%d %s", response.Code, response.Body)
	}
}

func TestTitleTranslationBatchFailureKeepsOldTitleAndRequiresConfiguration(t *testing.T) {
	deps := newTestDependencies()
	router := titleTranslationRouter(deps, t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"secret-key"}`))
	})
	response := translationAPIRequest(router, "POST", "/jav/title-translation/batch", `{"scope":"all","confirm_overwrite":true}`)
	if response.Code != http.StatusBadRequest {
		t.Fatal("batch started without configuration")
	}
	dbpkg.Default().UpsertConfig(context.Background(), map[string]string{
		"jav_title_translation_enabled": "true", translation.ProfilesKey: `{"deepseek":{"api_key":"secret-key","model":"model"}}`,
	})
	item := models.Jav{Code: "BATCH-FAIL", Title: "Original", ZhTitle: "旧译文"}
	deps.database.Create(&item)
	response = translationAPIRequest(router, "POST", "/jav/title-translation/batch", `{"scope":"all","confirm_overwrite":true}`)
	if response.Code != http.StatusAccepted {
		t.Fatal("batch did not start")
	}
	status := waitTitleTranslationBatch(t)
	if status.ErrorZH == "" || status.Processed != 0 || strings.Contains(status.ErrorEN, "secret-key") {
		t.Fatalf("failed batch=%+v", status)
	}
	var stored models.Jav
	deps.database.First(&stored, item.ID)
	if stored.Title != item.Title || stored.ZhTitle != item.ZhTitle {
		t.Fatal("failed batch erased old titles")
	}
}

func TestTitleTranslationBatchStopsAfterCurrentRequest(t *testing.T) {
	deps := newTestDependencies()
	entered, release := make(chan struct{}), make(chan struct{})
	calls := 0
	router := titleTranslationRouter(deps, t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			close(entered)
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"新译文"}}]}`))
	})
	dbpkg.Default().UpsertConfig(context.Background(), map[string]string{
		"jav_title_translation_enabled": "true", translation.ProfilesKey: `{"deepseek":{"api_key":"key","model":"model"}}`,
	})
	items := []models.Jav{{Code: "STOP-001", Title: "First"}, {Code: "STOP-002", Title: "Second", ZhTitle: "旧译文"}}
	if err := deps.database.Create(&items).Error; err != nil {
		t.Fatal(err)
	}
	response := translationAPIRequest(router, "POST", "/jav/title-translation/batch", `{"scope":"all","confirm_overwrite":true}`)
	if response.Code != http.StatusAccepted {
		t.Fatalf("start=%d %s", response.Code, response.Body)
	}
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("batch did not start")
	}
	response = translationAPIRequest(router, "DELETE", "/jav/title-translation/batch", "")
	if response.Code != http.StatusOK {
		t.Fatal("could not turn off translation")
	}
	close(release)
	status := waitTitleTranslationBatch(t)
	if calls != 1 || status.Processed != 1 || !status.Stopping {
		t.Fatalf("disabled batch=%+v calls=%d", status, calls)
	}
	var stored models.Jav
	deps.database.First(&stored, items[1].ID)
	if stored.ZhTitle != "旧译文" {
		t.Fatal("disabled batch kept translating")
	}
}

func TestTitleTranslationPreviewUsesDraftAndDoesNotPersistUntilSave(t *testing.T) {
	deps := newTestDependencies()
	calls := 0
	fail := false
	var reasoningModes []string
	router := titleTranslationRouter(deps, t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		var payload struct {
			Messages []struct{ Content string } `json:"messages"`
			Thinking struct{ Type string }      `json:"thinking"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if len(payload.Messages) != 2 || payload.Messages[1].Content != "Draft original" {
			t.Errorf("preview did not translate editor source: %+v", payload)
		}
		reasoningModes = append(reasoningModes, payload.Thinking.Type)
		if fail {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"新中文译文"}}]}`))
	})
	if err := dbpkg.Default().UpsertConfig(context.Background(), map[string]string{
		"jav_title_translation_enabled": "true", "jav_title_translation_auto_enabled": "false", translation.ProfilesKey: `{"deepseek":{"api_key":"key","model":"model","thinking":false}}`,
	}); err != nil {
		t.Fatal(err)
	}
	item := models.Jav{Code: "TRANS-PREVIEW", Title: "Stored original", ZhTitle: "已有中文译文"}
	if err := deps.database.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	path := "/jav/items/" + strconv.FormatInt(item.ID, 10) + "/title-translation"
	for _, body := range []string{`{"title":"Draft original"}`, `{"title":"Draft original","thinking":true}`} {
		response := translationAPIRequest(router, "POST", path, body)
		if response.Code != 200 || !strings.Contains(response.Body.String(), "新中文译文") {
			t.Fatalf("preview=%d %s", response.Code, response.Body)
		}
	}
	if calls != 2 {
		t.Fatal("preview reused the cached Chinese title")
	}
	fail = true
	response := translationAPIRequest(router, "POST", path, `{"title":"Draft original"}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("failed preview=%d %s", response.Code, response.Body)
	}
	if strings.Join(reasoningModes, ",") != "disabled,enabled,disabled" {
		t.Fatalf("reasoning modes=%v", reasoningModes)
	}
	cfg, err := dbpkg.Default().ListConfig(context.Background())
	if err != nil || translation.ActiveProfile(cfg).Thinking || cfg["jav_title_translation_auto_enabled"] != "false" {
		t.Fatalf("preview modified global settings: %v err=%v", cfg, err)
	}
	var stored models.Jav
	if err := deps.database.First(&stored, item.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Title != item.Title || stored.ZhTitle != item.ZhTitle {
		t.Fatal("preview changed saved titles before the editor was saved")
	}
	for _, body := range []string{`{"title":""}`, `{"title":"  "}`, `{"title":42}`, `{"thinking":"invalid"}`, `{"title":"` + strings.Repeat("x", 8193) + `"}`} {
		response = translationAPIRequest(router, "POST", path, body)
		if response.Code != http.StatusBadRequest || calls != 3 {
			t.Fatalf("invalid preview=%d %s calls=%d", response.Code, response.Body, calls)
		}
	}
}

func TestTitleTranslationPreviewUsesStoredSourceWithoutCachingOrSaving(t *testing.T) {
	deps := newTestDependencies()
	var calls int
	router := titleTranslationRouter(deps, t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		var payload struct{ Messages []struct{ Content string } }
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || len(payload.Messages) != 2 || payload.Messages[1].Content != "Stored original" {
			t.Errorf("preview source=%+v err=%v", payload, err)
		}
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"预览译文"}}]}`))
	})
	if err := dbpkg.Default().UpsertConfig(context.Background(), map[string]string{
		translation.ProfilesKey: `{"deepseek":{"api_key":"key","model":"model"}}`,
	}); err != nil {
		t.Fatal(err)
	}
	for _, enabled := range []string{"false", "true"} {
		if err := dbpkg.Default().UpsertConfig(context.Background(), map[string]string{"jav_title_translation_enabled": "true", "jav_title_translation_auto_enabled": enabled}); err != nil {
			t.Fatal(err)
		}
		for i, savedTitle := range []string{"", "人工校对"} {
			item := models.Jav{Code: "PREVIEW-" + enabled + strconv.Itoa(i), Title: "Stored original", ZhTitle: savedTitle}
			if err := deps.database.Create(&item).Error; err != nil {
				t.Fatal(err)
			}
			for _, body := range []string{"", `{}`, `{"thinking":false}`} {
				response := translationAPIRequest(router, "POST", "/jav/items/"+strconv.FormatInt(item.ID, 10)+"/title-translation", body)
				var preview models.Jav
				if err := json.Unmarshal(response.Body.Bytes(), &preview); err != nil || response.Code != 200 || preview.ID != item.ID || preview.Title != item.Title || preview.ZhTitle != "预览译文" {
					t.Fatalf("preview=%d %s err=%v", response.Code, response.Body, err)
				}
				stored, err := dbpkg.Default().GetJavTitleTranslationItem(context.Background(), item.ID)
				if err != nil || stored.Title != item.Title || stored.ZhTitle != savedTitle {
					t.Fatalf("preview saved titles: %+v err=%v", stored, err)
				}
			}
		}
	}
	if calls != 12 {
		t.Fatalf("previews reused a cached title: calls=%d", calls)
	}
	for _, id := range []string{"invalid", "0", "-1", "999999"} {
		response := translationAPIRequest(router, "POST", "/jav/items/"+id+"/title-translation", `{}`)
		want := http.StatusBadRequest
		if id == "999999" {
			want = http.StatusNotFound
		}
		if response.Code != want || calls != 12 {
			t.Fatalf("invalid/missing item=%d %s calls=%d", response.Code, response.Body, calls)
		}
	}
}
