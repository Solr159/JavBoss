package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"javboss/internal/common"
	dbpkg "javboss/internal/db"
	"javboss/internal/jav/translation"
	"javboss/internal/models"
)

func TestSeriesTranslationPreviewSaveAndAutomaticTranslation(t *testing.T) {
	var calls atomic.Int32
	router := titleTranslationRouter(t, func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		wantPrompt := translation.DefaultPrompt
		if call > 1 {
			wantPrompt = "共用自定义翻译提示词"
		}
		var payload struct{ Messages []struct{ Content string } }
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if len(payload.Messages) != 2 || payload.Messages[0].Content != wantPrompt || payload.Messages[1].Content != "雨の日" {
			t.Errorf("unexpected series translation request: %+v", payload)
		}
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"雨天系列"}}]}`))
	})
	configureGroupedTranslation(t)
	series := models.JavSeries{Name: "雨の日"}
	if err := common.DB.Create(&series).Error; err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/jav/series/%d", series.ID)
	body := fmt.Sprintf(`{"ids":[%d,%d]}`, series.ID, series.ID)
	response := translationAPIRequest(router, "POST", "/jav/series-translation/items", body)
	if response.Code != http.StatusBadRequest || calls.Load() != 0 {
		t.Fatalf("title switch enabled series translation: %d %s", response.Code, response.Body)
	}
	if err := dbpkg.UpsertConfig(context.Background(), map[string]string{"jav_series_translation_enabled": "true", "jav_series_translation_auto_enabled": "false"}); err != nil {
		t.Fatal(err)
	}
	response = translationAPIRequest(router, "POST", "/jav/series-translation/items", body)
	if response.Code != 400 || calls.Load() != 0 {
		t.Fatalf("automatic disabled: %d %s", response.Code, response.Body)
	}
	response = translationAPIRequest(router, "POST", path+"/name-translation", `{}`)
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"zh_name":"雨天系列"`) {
		t.Fatalf("preview: %d %s", response.Code, response.Body)
	}
	assertStoredSeries := func(want string) {
		t.Helper()
		var stored models.JavSeries
		if err := common.DB.First(&stored, series.ID).Error; err != nil {
			t.Fatal(err)
		}
		if stored.Name != series.Name || stored.ZhName != want {
			t.Fatalf("stored series = %+v, want translation %q", stored, want)
		}
	}
	assertStoredSeries("")
	response = translationAPIRequest(router, "PATCH", path, `{"name":"雨の日","zh_name":"人工校对"}`)
	if response.Code != 200 {
		t.Fatalf("save: %d %s", response.Code, response.Body)
	}
	assertStoredSeries("人工校对")
	response = translationAPIRequest(router, "PATCH", path, `{"name":"过期名称","zh_name":"错误译文"}`)
	if response.Code != http.StatusConflict {
		t.Fatalf("stale save: %d %s", response.Code, response.Body)
	}
	assertStoredSeries("人工校对")
	response = translationAPIRequest(router, "PATCH", "/config", `{"jav_series_translation_enabled":true,"jav_series_translation_auto_enabled":true,"jav_title_translation_enabled":false,"jav_title_translation_prompt":"共用自定义翻译提示词"}`)
	if response.Code != 200 {
		t.Fatalf("configure series: %d %s", response.Code, response.Body)
	}
	response = translationAPIRequest(router, "POST", "/jav/series-translation/items", body)
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"zh_name":"人工校对"`) || calls.Load() != 1 {
		t.Fatalf("cached series: %d %s calls=%d", response.Code, response.Body, calls.Load())
	}
	response = translationAPIRequest(router, "PATCH", path, `{"zh_name":""}`)
	if response.Code != 200 {
		t.Fatalf("clear: %d %s", response.Code, response.Body)
	}
	response = translationAPIRequest(router, "POST", "/jav/series-translation/items", body)
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"done":true`) || calls.Load() != 2 {
		t.Fatalf("automatic: %d %s calls=%d", response.Code, response.Body, calls.Load())
	}
	assertStoredSeries("雨天系列")
}

func TestSeriesTranslationBatchScopesAndConcurrentEdits(t *testing.T) {
	var calls atomic.Int32
	items := []models.JavSeries{
		{Name: "Series one", ZhName: "原译名"},
		{Name: "Series two"},
		{Name: "Internal English", IsEnglish: true},
		{Name: " "},
	}
	router := titleTranslationRouter(t, func(w http.ResponseWriter, r *http.Request) {
		var payload struct{ Messages []struct{ Content string } }
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if len(payload.Messages) != 2 || payload.Messages[0].Content != "批量共用提示词" {
			t.Errorf("unexpected batch translation prompt: %+v", payload)
		}
		if calls.Add(1) == 1 {
			if err := common.DB.Model(&items[0]).Update("zh_name", "同时人工修改").Error; err != nil {
				t.Error(err)
			}
		}
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"新系列译名"}}]}`))
	})
	configureGroupedTranslation(t)
	if err := dbpkg.UpsertConfig(context.Background(), map[string]string{"jav_series_translation_enabled": "true", "jav_title_translation_prompt": "批量共用提示词"}); err != nil {
		t.Fatal(err)
	}
	if err := common.DB.Create(&items).Error; err != nil {
		t.Fatal(err)
	}
	for _, scope := range []string{"missing", "all"} {
		response := translationAPIRequest(router, "GET", "/jav/series-translation/batch/preview?scope="+scope, "")
		var preview struct{ Total, Existing int }
		if err := json.Unmarshal(response.Body.Bytes(), &preview); err != nil {
			t.Fatal(err)
		}
		wantTotal, wantExisting := 1, 0
		if scope == "all" {
			wantTotal, wantExisting = 2, 1
		}
		if response.Code != 200 || preview.Total != wantTotal || preview.Existing != wantExisting {
			t.Fatalf("%s preview: %d %s", scope, response.Code, response.Body)
		}
	}
	response := translationAPIRequest(router, "POST", "/jav/series-translation/batch", `{"scope":"all"}`)
	if response.Code != 400 || calls.Load() != 0 {
		t.Fatal("overwrites were not confirmed")
	}
	response = translationAPIRequest(router, "POST", "/jav/series-translation/batch", `{"scope":"all","confirm_overwrite":true}`)
	if response.Code != http.StatusAccepted {
		t.Fatalf("start: %d %s", response.Code, response.Body)
	}
	status := waitTextTranslationBatch(t, allSeriesTranslations)
	if status.Processed != 2 || status.Translated != 1 || status.Skipped != 1 || calls.Load() != 2 {
		t.Fatalf("batch status = %+v, calls=%d", status, calls.Load())
	}
	var stored []models.JavSeries
	if err := common.DB.Order("id").Find(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored[0].ZhName != "同时人工修改" || stored[1].ZhName != "新系列译名" || stored[2].ZhName != "" {
		t.Fatalf("unexpected batch result: %+v", stored)
	}
}

func TestSeriesTranslationRejectsStaleAutomaticResultsAndInvalidRequests(t *testing.T) {
	series := models.JavSeries{Name: "Original"}
	router := titleTranslationRouter(t, func(w http.ResponseWriter, r *http.Request) {
		if err := common.DB.Model(&series).Update("name", "Changed source").Error; err != nil {
			t.Error(err)
		}
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"过期译名"}}]}`))
	})
	configureGroupedTranslation(t)
	if err := dbpkg.UpsertConfig(context.Background(), map[string]string{"jav_series_translation_enabled": "true", "jav_series_translation_auto_enabled": "true"}); err != nil {
		t.Fatal(err)
	}
	if err := common.DB.Create(&series).Error; err != nil {
		t.Fatal(err)
	}
	response := translationAPIRequest(router, "POST", "/jav/series-translation/items", fmt.Sprintf(`{"ids":[%d]}`, series.ID))
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"name":"Changed source"`) || !strings.Contains(response.Body.String(), `"zh_name":""`) {
		t.Fatalf("stale result: %d %s", response.Code, response.Body)
	}
	for _, test := range []struct {
		method, path, body string
		status             int
	}{
		{"POST", "/jav/series/999/name-translation", `{}`, 404},
		{"POST", "/jav/series/0/name-translation", `{}`, 400},
		{"POST", "/jav/series/1/name-translation", `{"name":""}`, 400},
		{"POST", "/jav/series-translation/items", `{"ids":[0]}`, 400},
		{"PATCH", "/jav/series/1", `{}`, 400},
		{"PATCH", "/jav/series/999", `{"zh_name":"译名"}`, 404},
	} {
		response := translationAPIRequest(router, test.method, test.path, test.body)
		if response.Code != test.status {
			t.Fatalf("%s %s: %d %s", test.method, test.path, response.Code, response.Body)
		}
	}
}
