package api

import (
	"context"
	"encoding/json"
	"io"
	dbpkg "javboss/internal/db"
	"javboss/internal/jav/translation"
	"javboss/internal/models"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestTitleTranslationItemsKeepCachedTitlesAndConcurrentCorrections(t *testing.T) {
	deps := newTestDependencies()
	items := []models.Jav{
		{Code: "GROUP-1", Title: "雨の日"},
		{Code: "GROUP-2", Title: "駅で再会", ZhTitle: "人工校对"},
		{Code: "GROUP-3", Title: "旅の始まり"},
		{Code: "GROUP-4", Title: " "},
	}
	var calls atomic.Int32
	router := titleTranslationRouter(deps, t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var payload struct{ Messages []struct{ Content string } }
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if payload.Messages[len(payload.Messages)-1].Content == items[2].Title {
			if err := deps.database.Model(&items[2]).Update("zh_title", "翻译期间的人工校对").Error; err != nil {
				t.Error(err)
			}
		}
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"雨天"}}]}`))
	})
	if err := dbpkg.Default().UpsertConfig(context.Background(), map[string]string{
		"jav_title_translation_enabled": "true", "jav_title_translation_auto_enabled": "true", translation.ProfilesKey: `{"deepseek":{"api_key":"key","model":"model"}}`,
	}); err != nil {
		t.Fatal(err)
	}
	if err := deps.database.Create(&items).Error; err != nil {
		t.Fatal(err)
	}
	ids := []int64{items[0].ID, items[0].ID, items[1].ID, items[2].ID, items[3].ID, 999999}
	body, _ := json.Marshal(map[string]any{"ids": ids})
	for i := 0; i < 2; i++ {
		response := translationAPIRequest(router, "POST", "/jav/title-translation/items", string(body))
		result := readTitleTranslationItems(t, response.Code, response.Header(), response.Body)
		if len(result) != 4 || result[items[0].ID].ZhTitle != "雨天" || result[items[1].ID].ZhTitle != "人工校对" || result[items[2].ID].ZhTitle != "翻译期间的人工校对" || result[items[3].ID].ZhTitle != "" {
			t.Fatalf("titles=%+v", result)
		}
		for _, item := range items {
			stored, err := dbpkg.Default().GetJavTitleTranslationItem(context.Background(), item.ID)
			if err != nil || result[item.ID].Title != item.Title || stored.Title != item.Title || stored.ZhTitle != result[item.ID].ZhTitle {
				t.Fatalf("translated titles: stored=%+v result=%+v err=%v", stored, result[item.ID], err)
			}
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("expected only two provider requests, got %d", calls.Load())
	}
}

func TestTitleTranslationItemsReturnPartialSuccessAndStopOnProviderError(t *testing.T) {
	deps := newTestDependencies()
	var calls atomic.Int32
	arrived := make(chan struct{}, 3)
	release := make(chan struct{})
	router := titleTranslationRouter(deps, t, func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		arrived <- struct{}{}
		if n == 1 {
			w.WriteHeader(429)
			_, _ = w.Write([]byte(`{"error":{"code":"credit_balance_exhausted","message":"Add credits."}}`))
			return
		}
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"雨天"}}]}`))
	})
	configureGroupedTranslation(deps, t)
	items := []models.Jav{}
	for i := 0; i < 8; i++ {
		items = append(items, models.Jav{Code: "PART-" + strconv.Itoa(i), Title: "Source " + strconv.Itoa(i)})
	}
	if err := deps.database.Create(&items).Error; err != nil {
		t.Fatal(err)
	}
	ids := []int64{}
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	body, _ := json.Marshal(map[string]any{"ids": ids})
	srv := httptest.NewServer(router)
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "POST", srv.URL+"/jav/title-translation/items", strings.NewReader(string(body)))
	req.Header.Set("Accept", "application/json")
	response, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	decoder := json.NewDecoder(response.Body)
	var first struct {
		Error string `json:"error_en"`
	}
	if err := decoder.Decode(&first); err != nil || !strings.Contains(first.Error, "credit_balance_exhausted") {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	// The error arrives before already-started paid requests finish.
	close(release)
	completed := 0
	for {
		var event struct {
			Item *models.Jav
			Done bool
		}
		if err := decoder.Decode(&event); err != nil {
			t.Fatal(err)
		}
		if event.Done {
			break
		}
		if event.Item != nil {
			completed++
		}
	}
	if calls.Load() > 3 || completed != int(calls.Load())-1 {
		t.Fatalf("calls=%d completed=%d", calls.Load(), completed)
	}
	for _, item := range items {
		stored, err := dbpkg.Default().GetJavTitleTranslationItem(context.Background(), item.ID)
		if err != nil {
			t.Fatal(err)
		}
		if stored.ZhTitle != "" && stored.ZhTitle != "雨天" {
			t.Fatalf("title=%s", stored.ZhTitle)
		}
	}
}

func configureGroupedTranslation(deps *testDependencies, t *testing.T) {
	t.Helper()
	if err := dbpkg.Default().UpsertConfig(context.Background(), map[string]string{
		"jav_title_translation_enabled": "true", "jav_title_translation_auto_enabled": "true", translation.ProfilesKey: `{"deepseek":{"api_key":"key","model":"model"}}`,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestTitleTranslationItemsStreamBeforeBatchFinishesAndDeduplicateAcrossRequests(t *testing.T) {
	deps := newTestDependencies()
	var calls, active, maximum atomic.Int32
	arrived := make(chan string, 10)
	first := make(chan struct{})
	rest := make(chan struct{})
	router := titleTranslationRouter(deps, t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		n := active.Add(1)
		defer active.Add(-1)
		for old := maximum.Load(); n > old; old = maximum.Load() {
			if maximum.CompareAndSwap(old, n) {
				break
			}
		}
		var payload struct{ Messages []struct{ Content string } }
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			return
		}
		source := payload.Messages[len(payload.Messages)-1].Content
		arrived <- source
		gate := rest
		if source == "Source 0" {
			gate = first
		}
		select {
		case <-gate:
		case <-r.Context().Done():
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"雨天"}}]}`))
	})
	configureGroupedTranslation(deps, t)
	items := []models.Jav{}
	for i := 0; i < 6; i++ {
		items = append(items, models.Jav{Code: "STREAM-" + strconv.Itoa(i), Title: "Source " + strconv.Itoa(i)})
	}
	if err := deps.database.Create(&items).Error; err != nil {
		t.Fatal(err)
	}
	ids := []int64{}
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	body, _ := json.Marshal(map[string]any{"ids": ids})
	srv := httptest.NewServer(router)
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "POST", srv.URL+"/jav/title-translation/items", strings.NewReader(string(body)))
	req.Header.Set("Accept", "application/x-ndjson")
	response, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	for i := 0; i < 3; i++ {
		select {
		case <-arrived:
		case <-ctx.Done():
			t.Fatal("three titles did not run concurrently")
		}
	}
	duplicate := make(chan *http.Response, 1)
	go func() {
		req, _ := http.NewRequestWithContext(ctx, "POST", srv.URL+"/jav/title-translation/items", strings.NewReader(string(body)))
		res, _ := srv.Client().Do(req)
		duplicate <- res
	}()
	close(first)
	decoder := json.NewDecoder(response.Body)
	var event struct {
		Item *models.Jav
		Done bool
	}
	if err := decoder.Decode(&event); err != nil || event.Item == nil || event.Item.ID != items[0].ID {
		t.Fatalf("first streamed item=%+v err=%v", event, err)
	}
	// Other titles are still blocked, so receiving this item proves incremental delivery.
	close(rest)
	count := 1
	for !event.Done {
		event.Item = nil
		if err := decoder.Decode(&event); err != nil {
			t.Fatal(err)
		}
		if event.Item != nil {
			count++
		}
	}
	other := <-duplicate
	if other == nil {
		t.Fatal("duplicate request failed")
	}
	defer other.Body.Close()
	otherItems := readTitleTranslationItems(t, other.StatusCode, other.Header, other.Body)
	if count != 6 || len(otherItems) != 6 || calls.Load() != 6 || maximum.Load() != 3 {
		t.Fatalf("streamed=%d duplicate=%d calls=%d concurrent=%d", count, len(otherItems), calls.Load(), maximum.Load())
	}
}

func TestTitleTranslationItemsValidateBeforeSpendingRequests(t *testing.T) {
	deps := newTestDependencies()
	router := titleTranslationRouter(deps, t, func(http.ResponseWriter, *http.Request) { t.Error("unexpected provider request") })
	oversized := `{"ids":[` + strings.Repeat("1,", 20) + `1]}`
	for _, body := range []string{`{}`, `{"ids":[]}`, `{"ids":[0]}`, `{"ids":[1,-1]}`, `{"ids":["1"]}`, oversized} {
		t.Run(body, func(t *testing.T) {
			response := translationAPIRequest(router, "POST", "/jav/title-translation/items", body)
			if response.Code != 400 {
				t.Fatalf("status=%d %s", response.Code, response.Body)
			}
		})
	}
	item := models.Jav{Code: "DISABLED-1", Title: "雨の日"}
	if err := deps.database.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	response := translationAPIRequest(router, "POST", "/jav/title-translation/items", `{"ids":[`+strconv.FormatInt(item.ID, 10)+`]}`)
	if response.Code != 400 || !strings.Contains(response.Body.String(), "disabled") {
		t.Fatalf("disabled response=%d %s", response.Code, response.Body)
	}
}

func TestTitleTranslationItemsDiscardTranslationAfterConcurrentSourceEdit(t *testing.T) {
	deps := newTestDependencies()
	calls := 0
	item := models.Jav{Code: "TRANS-001", Title: "Original source"}
	router := titleTranslationRouter(deps, t, func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Thinking struct{ Type string } `json:"thinking"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Thinking.Type != "disabled" {
			t.Errorf("reasoning should be disabled: %+v err=%v", payload, err)
		}
		calls++
		if err := deps.database.Model(&item).Update("title", "User edit").Error; err != nil {
			t.Error(err)
		}
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"雨天相遇"}}]}`))
	})
	if err := dbpkg.Default().UpsertConfig(context.Background(), map[string]string{
		"jav_title_translation_enabled": "true", "jav_title_translation_auto_enabled": "true", translation.ProfilesKey: `{"deepseek":{"api_key":"key","model":"model"}}`,
	}); err != nil {
		t.Fatal(err)
	}
	if err := deps.database.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	path := "/jav/title-translation/items"
	body := `{"ids":[` + strconv.FormatInt(item.ID, 10) + `]}`
	response := translationAPIRequest(router, "POST", path, body)
	result := readTitleTranslationItems(t, response.Code, response.Header(), response.Body)
	if calls != 1 || len(result) != 1 || result[item.ID].Title != "User edit" || result[item.ID].ZhTitle != "" {
		t.Fatalf("concurrent edit=%+v", result)
	}
	var stored models.Jav
	if err := deps.database.First(&stored, item.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Title != "User edit" || stored.ZhTitle != "" {
		t.Fatalf("stale translation was saved after the source changed: %+v", stored)
	}
}

func TestTitleTranslationItemsCanShowExistingChineseTitleWithoutAPIKey(t *testing.T) {
	deps := newTestDependencies()
	router := titleTranslationRouter(deps, t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("an existing Chinese title must not call DeepSeek")
	})
	response := translationAPIRequest(router, "PATCH", "/config", `{"jav_title_translation_enabled":true,"jav_title_translation_auto_enabled":true}`)
	if response.Code != 200 {
		t.Fatalf("enable display=%d %s", response.Code, response.Body)
	}
	item := models.Jav{Code: "TRANS-EXISTING", Title: "Existing original", ZhTitle: "已有中文标题"}
	if err := deps.database.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	response = translationAPIRequest(router, "POST", "/jav/title-translation/items", `{"ids":[`+strconv.FormatInt(item.ID, 10)+`]}`)
	result := readTitleTranslationItems(t, response.Code, response.Header(), response.Body)
	if len(result) != 1 || result[item.ID].Title != item.Title || result[item.ID].ZhTitle != item.ZhTitle {
		t.Fatalf("cached title=%+v", result)
	}
}

// Collect successful streams by ID: completion order is deliberately unspecified.
func readTitleTranslationItems(t *testing.T, status int, header http.Header, body io.Reader) map[int64]models.Jav {
	t.Helper()
	if status != http.StatusOK || header.Get("Content-Type") != "application/x-ndjson; charset=utf-8" {
		t.Fatalf("status=%d content-type=%s", status, header.Get("Content-Type"))
	}
	items := make(map[int64]models.Jav)
	decoder := json.NewDecoder(body)
	for {
		var event struct {
			Item      *models.Jav
			Done      bool
			Heartbeat bool
			Error     string `json:"error_en"`
		}
		if err := decoder.Decode(&event); err != nil {
			t.Fatalf("stream ended before done: %v", err)
		}
		if event.Error != "" {
			t.Fatalf("unexpected stream error: %s", event.Error)
		}
		if event.Done {
			var extra any
			if err := decoder.Decode(&extra); err != io.EOF {
				t.Fatalf("expected EOF after done, got %v (%v)", extra, err)
			}
			return items
		}
		if event.Item != nil {
			if _, exists := items[event.Item.ID]; exists {
				t.Fatalf("duplicate streamed ID: %d", event.Item.ID)
			}
			items[event.Item.ID] = *event.Item
		} else if !event.Heartbeat {
			t.Fatalf("unexpected stream event: %+v", event)
		}
	}
}
