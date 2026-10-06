package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"javboss/internal/common"
	dbpkg "javboss/internal/db"
	"javboss/internal/jav/translation"
	"javboss/internal/models"
)

func titleTranslationRouter(t *testing.T, handler http.HandlerFunc) *gin.Engine {
	t.Helper()
	testAuthService(t)
	upstream := httptest.NewServer(handler)
	t.Cleanup(upstream.Close)
	previous := deepSeekClient
	previousBatch := allTitleTranslations
	ctx, cancel := context.WithCancel(context.Background())
	allTitleTranslations = &titleTranslationBatch{ctx: ctx}
	deepSeekClient = &translation.Client{BaseURL: upstream.URL, HTTP: upstream.Client()}
	t.Cleanup(func() { deepSeekClient = previous })
	t.Cleanup(func() {
		cancel()
		waitTitleTranslationBatch(t)
		allTitleTranslations = previousBatch
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
	router := titleTranslationRouter(t, func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Thinking struct{ Type string } `json:"thinking"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Thinking.Type != "enabled" {
			t.Errorf("enabled reasoning request=%+v err=%v", payload, err)
		}
		time.Sleep(80 * time.Millisecond)
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"reasoning_content":"internal reasoning","content":"雨天相遇"}}]}`))
	})
	if err := dbpkg.UpsertConfig(context.Background(), map[string]string{
		"jav_title_translation_enabled": "true", "jav_title_translation_api_key": "key", "jav_title_translation_model": "model", "jav_title_translation_thinking": "true",
	}); err != nil {
		t.Fatal(err)
	}
	item := models.Jav{Code: "REASON-001", Title: "雨の日の出会い"}
	if err := common.DB.Create(&item).Error; err != nil {
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
	var receivedKey string
	router := titleTranslationRouter(t, func(w http.ResponseWriter, r *http.Request) {
		receivedKey = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"data":[{"id":"official-new-model","name":"Official new model"}]}`))
	})
	defaults := translationAPIRequest(router, "GET", "/config", "")
	var defaultConfig map[string]string
	if err := json.Unmarshal(defaults.Body.Bytes(), &defaultConfig); err != nil {
		t.Fatal(err)
	}
	if defaults.Code != http.StatusOK || defaultConfig["jav_title_translation_thinking"] != "false" || defaultConfig["jav_title_translation_prompt"] != translation.DefaultPrompt {
		t.Fatalf("default settings=%d %+v", defaults.Code, defaultConfig)
	}
	body := `{"jav_title_translation_enabled":true,"jav_title_translation_thinking":false,"jav_title_translation_api_key":"private-key","jav_title_translation_model":"official-new-model","jav_title_translation_prompt":"自定义提示词"}`
	response := translationAPIRequest(router, "PATCH", "/config", body)
	if response.Code != 200 || bytes.Contains(response.Body.Bytes(), []byte("private-key")) {
		t.Fatalf("config response: %d %s", response.Code, response.Body)
	}
	var cfg map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg["jav_title_translation_api_key_set"] != "true" || cfg["jav_title_translation_enabled"] != "true" || cfg["jav_title_translation_thinking"] != "false" {
		t.Fatalf("config=%v", cfg)
	}
	response = translationAPIRequest(router, "GET", "/config", "")
	if response.Code != 200 || bytes.Contains(response.Body.Bytes(), []byte("private-key")) {
		t.Fatal("GET leaked key")
	}
	response = translationAPIRequest(router, "POST", "/jav/title-translation/models", `{"api_key":"draft-key"}`)
	if response.Code != 200 || receivedKey != "Bearer draft-key" || !strings.Contains(response.Body.String(), "official-new-model") {
		t.Fatalf("model discovery=%d %s", response.Code, response.Body)
	}
	response = translationAPIRequest(router, "POST", "/jav/title-translation/models", `{}`)
	if response.Code != 200 || receivedKey != "Bearer private-key" {
		t.Fatal("did not reuse saved key")
	}
	stored, _ := dbpkg.ListConfig(context.Background())
	if stored["jav_title_translation_api_key"] != "private-key" || stored["jav_title_translation_thinking"] != "false" {
		t.Fatal("model discovery saved draft key")
	}
	response = translationAPIRequest(router, "PATCH", "/config", `{"jav_title_translation_enabled":false,"jav_title_translation_api_key":""}`)
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"jav_title_translation_api_key_set":"false"`) {
		t.Fatal("clear key failed")
	}
}

func TestTitleTranslationPreservesOriginalAndCachesChineseWithoutOverwritingConcurrentEdits(t *testing.T) {
	calls := 0
	var editDuringTranslation func()
	router := titleTranslationRouter(t, func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Thinking struct{ Type string } `json:"thinking"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Thinking.Type != "disabled" {
			t.Errorf("reasoning should be disabled: %+v err=%v", payload, err)
		}
		calls++
		if editDuringTranslation != nil {
			editDuringTranslation()
		}
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"雨天相遇"}}]}`))
	})
	if err := dbpkg.UpsertConfig(context.Background(), map[string]string{
		"jav_title_translation_enabled": "true", "jav_title_translation_api_key": "key", "jav_title_translation_model": "model",
	}); err != nil {
		t.Fatal(err)
	}
	item := models.Jav{Code: "TRANS-001", Title: "雨の日の出会い"}
	if err := common.DB.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	path := "/jav/items/" + strconv.FormatInt(item.ID, 10) + "/title-translation"
	for i := 0; i < 2; i++ {
		response := translationAPIRequest(router, "POST", path, "")
		if response.Code != 200 {
			t.Fatalf("translation=%d %s", response.Code, response.Body)
		}
	}
	var stored models.Jav
	if err := common.DB.First(&stored, item.ID).Error; err != nil {
		t.Fatal(err)
	}
	if calls != 1 || stored.Title != item.Title || stored.ZhTitle != "雨天相遇" {
		t.Fatalf("calls=%d item=%+v", calls, stored)
	}
	common.DB.Model(&stored).Updates(map[string]any{"title": "New source", "zh_title": ""})
	editDuringTranslation = func() { common.DB.Model(&stored).Update("title", "User edit") }
	response := translationAPIRequest(router, "POST", path, "")
	if response.Code != 200 || !strings.Contains(response.Body.String(), "User edit") {
		t.Fatalf("concurrent edit=%d %s", response.Code, response.Body)
	}
	common.DB.First(&stored, item.ID)
	if stored.ZhTitle != "" {
		t.Fatal("stale translation was saved after the source changed")
	}
	if err := dbpkg.UpsertConfig(context.Background(), map[string]string{"jav_title_translation_enabled": "false"}); err != nil {
		t.Fatal(err)
	}
	response = translationAPIRequest(router, "POST", path, "")
	if response.Code != 400 || calls != 2 {
		t.Fatal("disabled translation called provider")
	}
}

func TestTitleTranslationProviderFailureDoesNotReplaceTitleOrExpireSession(t *testing.T) {
	router := titleTranslationRouter(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":"secret-key"}`))
	})
	dbpkg.UpsertConfig(context.Background(), map[string]string{"jav_title_translation_enabled": "true", "jav_title_translation_api_key": "secret-key", "jav_title_translation_model": "model"})
	item := models.Jav{Code: "TRANS-FAIL", Title: "Existing title"}
	common.DB.Create(&item)
	response := translationAPIRequest(router, "POST", "/jav/items/"+strconv.FormatInt(item.ID, 10)+"/title-translation", "")
	if response.Code != 400 || strings.Contains(response.Body.String(), "secret-key") {
		t.Fatalf("failure=%d %s", response.Code, response.Body)
	}
	var stored models.Jav
	common.DB.First(&stored, item.ID)
	if stored.Title != "Existing title" || stored.ZhTitle != "" {
		t.Fatal("failed request replaced title")
	}
}

func TestTitleTranslationManualEditInvalidatesCachedTitle(t *testing.T) {
	testAuthService(t)
	item := models.Jav{Code: "TRANS-EDIT", Title: "旧译文", ZhTitle: "旧译文"}
	if err := common.DB.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	title := "旧译文"
	updated, err := dbpkg.UpdateJav(context.Background(), item.ID, dbpkg.JavUpdateInput{Title: &title}, nil)
	if err != nil || updated.ZhTitle != "旧译文" {
		t.Fatalf("unchanged edit=%+v err=%v", updated, err)
	}
	title = "New source"
	updated, err = dbpkg.UpdateJav(context.Background(), item.ID, dbpkg.JavUpdateInput{Title: &title}, nil)
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
	router := NewRouter("", testAuthService(t))
	for _, path := range []string{"/jav/title-translation/models", "/jav/items/1/title-translation", "/jav/title-translation/batch"} {
		response := translationAPIRequest(router, "POST", path, `{}`)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("path=%s status=%d", path, response.Code)
		}
	}
	response := translationAPIRequest(router, "GET", "/jav/title-translation/batch", "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("batch status was not protected: %d", response.Code)
	}
}

func waitTitleTranslationBatch(t *testing.T) titleTranslationBatchStatus {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		allTitleTranslations.mu.Lock()
		status := allTitleTranslations.status
		allTitleTranslations.mu.Unlock()
		if !status.Running {
			return status
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("batch translation did not stop")
	return titleTranslationBatchStatus{}
}

func TestTitleTranslationBatchRetranslatesAllAndProtectsConcurrentEdits(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	calls := 0
	router := titleTranslationRouter(t, func(w http.ResponseWriter, r *http.Request) {
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
	if err := dbpkg.UpsertConfig(context.Background(), map[string]string{
		"jav_title_translation_enabled": "true", "jav_title_translation_api_key": "key", "jav_title_translation_model": "model", "jav_title_translation_thinking": "false",
	}); err != nil {
		t.Fatal(err)
	}
	items := []models.Jav{
		{Code: "BATCH-001", Title: "Original one", ZhTitle: "已有译文"},
		{Code: "BATCH-002", Title: "Original two", ZhTitle: "保留手动修改"},
		{Code: "BATCH-003", Title: " "},
	}
	if err := common.DB.Create(&items).Error; err != nil {
		t.Fatal(err)
	}
	response := translationAPIRequest(router, "POST", "/jav/title-translation/batch", "")
	if response.Code != http.StatusAccepted {
		t.Fatalf("start=%d %s", response.Code, response.Body)
	}
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("batch did not start")
	}
	duplicate := translationAPIRequest(router, "POST", "/jav/title-translation/batch", "")
	if duplicate.Code != http.StatusAccepted || !strings.Contains(duplicate.Body.String(), `"running":true`) {
		t.Fatalf("duplicate=%d %s", duplicate.Code, duplicate.Body)
	}
	if err := common.DB.Model(&items[1]).Update("title", "User edited source").Error; err != nil {
		t.Fatal(err)
	}
	close(release)
	status := waitTitleTranslationBatch(t)
	if status.Total != 3 || status.Processed != 3 || status.Translated != 1 || status.Skipped != 2 || calls != 2 {
		t.Fatalf("status=%+v calls=%d", status, calls)
	}
	var stored models.Jav
	common.DB.First(&stored, items[0].ID)
	if stored.Title != items[0].Title || stored.ZhTitle != "新的中文标题" {
		t.Fatal("batch did not replace the existing Chinese title separately")
	}
	var edited models.Jav
	common.DB.First(&edited, items[1].ID)
	if edited.Title != "User edited source" || edited.ZhTitle != items[1].ZhTitle {
		t.Fatal("batch overwrote a concurrent edit")
	}
	response = translationAPIRequest(router, "GET", "/jav/title-translation/batch", "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"processed":3`) {
		t.Fatalf("status response=%d %s", response.Code, response.Body)
	}
}

func TestTitleTranslationBatchFailureKeepsOldTitleAndRequiresConfiguration(t *testing.T) {
	router := titleTranslationRouter(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"secret-key"}`))
	})
	response := translationAPIRequest(router, "POST", "/jav/title-translation/batch", "")
	if response.Code != http.StatusBadRequest {
		t.Fatal("batch started without configuration")
	}
	dbpkg.UpsertConfig(context.Background(), map[string]string{
		"jav_title_translation_enabled": "true", "jav_title_translation_api_key": "secret-key", "jav_title_translation_model": "model",
	})
	item := models.Jav{Code: "BATCH-FAIL", Title: "Original", ZhTitle: "旧译文"}
	common.DB.Create(&item)
	response = translationAPIRequest(router, "POST", "/jav/title-translation/batch", "")
	if response.Code != http.StatusAccepted {
		t.Fatal("batch did not start")
	}
	status := waitTitleTranslationBatch(t)
	if status.ErrorZH == "" || status.Processed != 0 || strings.Contains(status.ErrorEN, "secret-key") {
		t.Fatalf("failed batch=%+v", status)
	}
	var stored models.Jav
	common.DB.First(&stored, item.ID)
	if stored.Title != item.Title || stored.ZhTitle != item.ZhTitle {
		t.Fatal("failed batch erased old titles")
	}
}

func TestTitleTranslationBatchStopsWhenSwitchIsDisabled(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	calls := 0
	router := titleTranslationRouter(t, func(w http.ResponseWriter, r *http.Request) {
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
	dbpkg.UpsertConfig(context.Background(), map[string]string{
		"jav_title_translation_enabled": "true", "jav_title_translation_api_key": "key", "jav_title_translation_model": "model",
	})
	items := []models.Jav{{Code: "STOP-001", Title: "First"}, {Code: "STOP-002", Title: "Second", ZhTitle: "旧译文"}}
	if err := common.DB.Create(&items).Error; err != nil {
		t.Fatal(err)
	}
	response := translationAPIRequest(router, "POST", "/jav/title-translation/batch", "")
	if response.Code != http.StatusAccepted {
		t.Fatalf("start=%d %s", response.Code, response.Body)
	}
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("batch did not start")
	}
	response = translationAPIRequest(router, "PATCH", "/config", `{"jav_title_translation_enabled":false}`)
	if response.Code != http.StatusOK {
		t.Fatal("could not turn off translation")
	}
	close(release)
	status := waitTitleTranslationBatch(t)
	if calls != 1 || status.Processed != 1 || status.ErrorZH == "" {
		t.Fatalf("disabled batch=%+v calls=%d", status, calls)
	}
	var stored models.Jav
	common.DB.First(&stored, items[1].ID)
	if stored.ZhTitle != "旧译文" {
		t.Fatal("disabled batch kept translating")
	}
}

func TestTitleTranslationCanShowExistingChineseTitleWithoutAPIKey(t *testing.T) {
	router := titleTranslationRouter(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("an existing Chinese title must not call DeepSeek")
	})
	response := translationAPIRequest(router, "PATCH", "/config", `{"jav_title_translation_enabled":true}`)
	if response.Code != 200 {
		t.Fatalf("enable display=%d %s", response.Code, response.Body)
	}
	item := models.Jav{Code: "TRANS-EXISTING", Title: "Existing original", ZhTitle: "已有中文标题"}
	if err := common.DB.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	response = translationAPIRequest(router, "POST", "/jav/items/"+strconv.FormatInt(item.ID, 10)+"/title-translation", "")
	if response.Code != 200 || !strings.Contains(response.Body.String(), item.Title) || !strings.Contains(response.Body.String(), item.ZhTitle) {
		t.Fatalf("cached title=%d %s", response.Code, response.Body)
	}
}

func TestTitleTranslationRefreshUsesDraftAndDoesNotPersistUntilSave(t *testing.T) {
	calls := 0
	fail := false
	var reasoningModes []string
	router := titleTranslationRouter(t, func(w http.ResponseWriter, r *http.Request) {
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
			t.Errorf("refresh did not translate editor source: %+v", payload)
		}
		reasoningModes = append(reasoningModes, payload.Thinking.Type)
		if fail {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"新中文译文"}}]}`))
	})
	if err := dbpkg.UpsertConfig(context.Background(), map[string]string{
		"jav_title_translation_enabled": "false", "jav_title_translation_api_key": "key", "jav_title_translation_model": "model", "jav_title_translation_thinking": "false",
	}); err != nil {
		t.Fatal(err)
	}
	item := models.Jav{Code: "TRANS-REFRESH", Title: "Stored original", ZhTitle: "已有中文译文"}
	if err := common.DB.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	path := "/jav/items/" + strconv.FormatInt(item.ID, 10) + "/title-translation"
	for _, body := range []string{`{"refresh":true,"title":"Draft original"}`, `{"refresh":true,"title":"Draft original","thinking":true}`} {
		response := translationAPIRequest(router, "POST", path, body)
		if response.Code != 200 || !strings.Contains(response.Body.String(), "新中文译文") {
			t.Fatalf("refresh=%d %s", response.Code, response.Body)
		}
	}
	if calls != 2 {
		t.Fatal("refresh reused the cached Chinese title")
	}
	fail = true
	response := translationAPIRequest(router, "POST", path, `{"refresh":true,"title":"Draft original"}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("failed refresh=%d %s", response.Code, response.Body)
	}
	if strings.Join(reasoningModes, ",") != "disabled,enabled,disabled" {
		t.Fatalf("reasoning modes=%v", reasoningModes)
	}
	cfg, err := dbpkg.ListConfig(context.Background())
	if err != nil || cfg["jav_title_translation_thinking"] != "false" || cfg["jav_title_translation_enabled"] != "false" {
		t.Fatalf("refresh modified global settings: %v err=%v", cfg, err)
	}
	var stored models.Jav
	if err := common.DB.First(&stored, item.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Title != item.Title || stored.ZhTitle != item.ZhTitle {
		t.Fatal("refresh changed saved titles before the editor was saved")
	}
	for _, body := range []string{`{"refresh":true,"title":""}`, `{"title":"Draft original"}`, `{"refresh":"invalid"}`, `{"thinking":true}`, `{"refresh":true,"thinking":"invalid"}`} {
		response = translationAPIRequest(router, "POST", path, body)
		if response.Code != http.StatusBadRequest || calls != 3 {
			t.Fatalf("invalid refresh=%d %s calls=%d", response.Code, response.Body, calls)
		}
	}
}
