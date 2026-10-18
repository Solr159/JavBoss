package api

import (
	"context"
	"encoding/json"
	dbpkg "javboss/internal/db"
	"javboss/internal/jav/translation"
	"javboss/internal/models"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTitleTranslationDraftTestDoesNotSaveSettingsOrTitles(t *testing.T) {
	deps := newTestDependencies()
	calls := 0
	router := titleTranslationRouter(deps, t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		var payload struct {
			Model    string
			Thinking struct{ Type string }
			Messages []struct{ Content string }
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if payload.Model != "draft-model" || payload.Thinking.Type != "enabled" || len(payload.Messages) != 2 || payload.Messages[0].Content != "Draft prompt" || payload.Messages[1].Content != "Sample" {
			t.Errorf("unexpected payload: %+v", payload)
		}
		if r.Header.Get("Authorization") != "Bearer saved-key" {
			t.Error("did not retain saved provider key")
		}
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"示例译文"}}]}`))
	})
	if err := dbpkg.Default().UpsertConfig(context.Background(), map[string]string{translation.ProfilesKey: `{"deepseek":{"api_key":"saved-key","model":"saved-model"}}`, "jav_title_translation_enabled": "false"}); err != nil {
		t.Fatal(err)
	}
	item := models.Jav{Code: "SAMPLE-001", Title: "Original", ZhTitle: "人工译名"}
	deps.database.Create(&item)
	before, _ := dbpkg.Default().ListConfig(context.Background())
	response := translationAPIRequest(router, "POST", "/jav/title-translation/test", `{"provider":"deepseek","profile":{"model":"draft-model","thinking":true},"title":"Sample","prompt":"Draft prompt"}`)
	if response.Code != 200 || !strings.Contains(response.Body.String(), "示例译文") || strings.Contains(response.Body.String(), "saved-key") {
		t.Fatalf("test=%d %s", response.Code, response.Body)
	}
	after, _ := dbpkg.Default().ListConfig(context.Background())
	a, _ := json.Marshal(before)
	b, _ := json.Marshal(after)
	if string(a) != string(b) {
		t.Fatal("sample saved draft configuration")
	}
	var stored models.Jav
	deps.database.First(&stored, item.ID)
	if stored.ZhTitle != item.ZhTitle {
		t.Fatal("sample changed library title")
	}
	response = translationAPIRequest(router, "POST", "/jav/title-translation/test", `{"provider":"deepseek","profile":{"model":"draft-model","api_key":""},"title":"Sample"}`)
	if response.Code != 400 || calls != 1 {
		t.Fatal("explicitly cleared key was reused")
	}
}

func TestTranslationConfigMasterAndAutomaticSwitchesAreIndependent(t *testing.T) {
	deps := newTestDependencies()
	router := titleTranslationRouter(deps, t, func(w http.ResponseWriter, r *http.Request) { t.Error("configuration should not translate") })
	response := translationAPIRequest(router, "GET", "/config", "")
	var cfg map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &cfg); err != nil {
		t.Fatal(err)
	}
	if _, exists := cfg["jav_title_prefer_chinese"]; exists {
		t.Fatal("obsolete display preference exposed")
	}
	keys := []string{"jav_title_translation_enabled", "jav_series_translation_enabled", "jav_title_translation_auto_enabled", "jav_series_translation_auto_enabled"}
	for _, key := range keys {
		if cfg[key] == "true" {
			t.Fatalf("%s should default off", key)
		}
	}
	want := map[string]string{}
	for _, enabled := range []bool{true, false} {
		for _, key := range keys {
			body, _ := json.Marshal(map[string]bool{key: enabled})
			response = translationAPIRequest(router, "PATCH", "/config", string(body))
			if response.Code != 200 {
				t.Fatalf("save: %s", response.Body)
			}
			want[key] = strconv.FormatBool(enabled)
			response = translationAPIRequest(router, "GET", "/config", "")
			if err := json.Unmarshal(response.Body.Bytes(), &cfg); err != nil {
				t.Fatal(err)
			}
			for k, value := range want {
				if cfg[k] != value {
					t.Fatalf("%s = %q, want %q", k, cfg[k], value)
				}
			}
		}
	}
}

func TestTitleTranslationBatchDefaultsToMissingAndRequiresOverwriteConfirmation(t *testing.T) {
	deps := newTestDependencies()
	calls := 0
	router := titleTranslationRouter(deps, t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"新译文"}}]}`))
	})
	dbpkg.Default().UpsertConfig(context.Background(), map[string]string{"jav_title_translation_enabled": "true", "jav_title_translation_auto_enabled": "false", translation.ProfilesKey: `{"deepseek":{"api_key":"key","model":"model"}}`})
	items := []models.Jav{{Code: "MISSING-001", Title: "Missing"}, {Code: "MANUAL-002", Title: "Manual", ZhTitle: "人工校对"}, {Code: "EMPTY-003", Title: " "}}
	if err := deps.database.Create(&items).Error; err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		scope           string
		total, existing int
	}{{"missing", 1, 0}, {"all", 2, 1}} {
		response := translationAPIRequest(router, "GET", "/jav/title-translation/batch/preview?scope="+tt.scope, "")
		var preview struct{ Total, Existing int }
		json.Unmarshal(response.Body.Bytes(), &preview)
		if response.Code != 200 || preview.Total != tt.total || preview.Existing != tt.existing {
			t.Fatalf("preview: %s", response.Body)
		}
	}
	response := translationAPIRequest(router, "POST", "/jav/title-translation/batch", `{"scope":"all"}`)
	if response.Code != 400 || calls != 0 {
		t.Fatal("overwrite started without confirmation")
	}
	response = translationAPIRequest(router, "POST", "/jav/title-translation/batch", "")
	if response.Code != 202 {
		t.Fatalf("manual batch requires automatic translation: %s", response.Body)
	}
	status := waitTitleTranslationBatch(t)
	if status.Total != 1 || status.Translated != 1 || calls != 1 {
		t.Fatalf("missing batch: %+v calls=%d", status, calls)
	}
	var stored models.Jav
	deps.database.First(&stored, items[1].ID)
	if stored.ZhTitle != "人工校对" {
		t.Fatal("default batch overwrote manual title")
	}
}

func TestTitleTranslationManualPreviewInheritsSavedReasoning(t *testing.T) {
	deps := newTestDependencies()
	var modes []string
	router := titleTranslationRouter(deps, t, func(w http.ResponseWriter, r *http.Request) {
		var payload struct{ Thinking struct{ Type string } }
		json.NewDecoder(r.Body).Decode(&payload)
		modes = append(modes, payload.Thinking.Type)
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"示例译文"}}]}`))
	})
	dbpkg.Default().UpsertConfig(context.Background(), map[string]string{"jav_title_translation_enabled": "true", translation.ProfilesKey: `{"deepseek":{"api_key":"key","model":"model","thinking":true}}`})
	item := models.Jav{Code: "INHERIT-001", Title: "Original", ZhTitle: "Existing"}
	deps.database.Create(&item)
	for _, body := range []string{`{}`, `{"thinking":false}`} {
		response := translationAPIRequest(router, "POST", "/jav/items/"+strconv.FormatInt(item.ID, 10)+"/title-translation", body)
		if response.Code != 200 {
			t.Fatalf("preview=%d %s", response.Code, response.Body)
		}
	}
	if strings.Join(modes, ",") != "enabled,disabled" {
		t.Fatalf("modes=%v", modes)
	}
}

func TestTitleTranslationConfigKeepsProfileKeysPrivate(t *testing.T) {
	deps := newTestDependencies()
	router := titleTranslationRouter(deps, t, func(http.ResponseWriter, *http.Request) { t.Error("unexpected provider request") })
	legacy := map[string]string{
		"jav_title_translation_api_key":     "legacy-secret",
		"jav_title_translation_api_key_set": "true",
		"jav_title_translation_model":       "legacy-model",
		"jav_title_translation_thinking":    "true",
	}
	if err := dbpkg.Default().UpsertConfig(context.Background(), legacy); err != nil {
		t.Fatal(err)
	}
	checkPublic := func(t *testing.T, body []byte, model string, thinking, keySet bool) {
		t.Helper()
		var cfg map[string]string
		if err := json.Unmarshal(body, &cfg); err != nil {
			t.Fatal(err)
		}
		for key := range legacy {
			if _, exists := cfg[key]; exists {
				t.Fatalf("public config contains obsolete field %s", key)
			}
		}
		if strings.Contains(string(body), "secret") || cfg[translation.ProviderKey] != "deepseek" || cfg["jav_title_translation_catalog"] == "" || cfg["jav_title_translation_capabilities"] == "" {
			t.Fatalf("public config=%s", body)
		}
		var profiles map[string]translation.Profile
		if err := json.Unmarshal([]byte(cfg[translation.ProfilesKey]), &profiles); err != nil {
			t.Fatal(err)
		}
		if model == "" && (profiles == nil || len(profiles) != 0) {
			t.Fatalf("legacy fields created a profile: %+v", profiles)
		}
		p := profiles[cfg[translation.ProviderKey]]
		if p.Model != model || p.Thinking != thinking || p.APIKeySet != keySet || p.APIKey != "" {
			t.Fatalf("public profile=%+v", p)
		}
	}
	response := translationAPIRequest(router, "GET", "/config", "")
	if response.Code != http.StatusOK {
		t.Fatalf("config=%d %s", response.Code, response.Body)
	}
	checkPublic(t, response.Body.Bytes(), "", false, false)
	for _, tt := range []struct {
		name, patch, key, model string
		thinking                bool
	}{
		{"profile without key", `{"jav_title_translation_profiles":{"deepseek":{"model":"profile-model","thinking":false}}}`, "", "profile-model", false},
		{"save key", `{"jav_title_translation_profiles":{"deepseek":{"api_key":"profile-secret","thinking":true}}}`, "profile-secret", "profile-model", true},
		{"retain omitted key", `{"jav_title_translation_profiles":{"deepseek":{"model":"updated-model"}}}`, "profile-secret", "updated-model", true},
		{"clear key", `{"jav_title_translation_profiles":{"deepseek":{"api_key":"","thinking":false}}}`, "", "updated-model", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			response := translationAPIRequest(router, "PATCH", "/config", tt.patch)
			if response.Code != http.StatusOK {
				t.Fatalf("save=%d %s", response.Code, response.Body)
			}
			checkPublic(t, response.Body.Bytes(), tt.model, tt.thinking, tt.key != "")
			response = translationAPIRequest(router, "GET", "/config", "")
			if response.Code != http.StatusOK {
				t.Fatalf("config=%d %s", response.Code, response.Body)
			}
			checkPublic(t, response.Body.Bytes(), tt.model, tt.thinking, tt.key != "")
			stored, err := dbpkg.Default().ListConfig(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			p := translation.ActiveProfile(stored)
			if p.APIKey != tt.key || p.Model != tt.model || p.Thinking != tt.thinking {
				t.Fatalf("stored profile=%+v", p)
			}
		})
	}
}

func TestTitleTranslationConfigPromptAndDraftCapabilities(t *testing.T) {
	deps := newTestDependencies()
	router := titleTranslationRouter(deps, t, func(http.ResponseWriter, *http.Request) { t.Error("unexpected provider request") })
	response := translationAPIRequest(router, "PATCH", "/config", `{"jav_title_translation_prompt":" Custom prompt "}`)
	stored, err := dbpkg.Default().ListConfig(context.Background())
	if err != nil || response.Code != http.StatusOK || stored["jav_title_translation_prompt"] != "Custom prompt" {
		t.Fatalf("custom prompt=%d %s err=%v", response.Code, response.Body, err)
	}
	response = translationAPIRequest(router, "POST", "/jav/title-translation/models", `{"provider":"deepseek","profile":{"model":"deepseek-flash","reasoning":""},"discover":false}`)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"reasoning":"toggle"`) {
		t.Fatalf("draft capabilities=%d %s", response.Code, response.Body)
	}
	for _, prompt := range []string{"", strings.Repeat("x", 8193)} {
		body, err := json.Marshal(map[string]string{"jav_title_translation_prompt": prompt})
		if err != nil {
			t.Fatal(err)
		}
		response = translationAPIRequest(router, "PATCH", "/config", string(body))
		if prompt == "" {
			var cfg map[string]string
			if err := json.Unmarshal(response.Body.Bytes(), &cfg); err != nil || response.Code != http.StatusOK || cfg["jav_title_translation_prompt"] != translation.DefaultPrompt {
				t.Fatalf("default prompt=%d %s", response.Code, response.Body)
			}
		} else if response.Code != http.StatusBadRequest {
			t.Fatalf("oversized prompt=%d %s", response.Code, response.Body)
		}
	}
}

func TestTranslationMasterSwitchBlocksEveryTranslationEntry(t *testing.T) {
	deps := newTestDependencies()
	router := titleTranslationRouter(deps, t, func(w http.ResponseWriter, r *http.Request) { t.Error("disabled translation called provider") })
	configureGroupedTranslation(deps, t)
	item := models.Jav{Code: "DISABLED-001", Title: "Original"}
	series := models.JavSeries{Name: "Original series"}
	if err := deps.database.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	if err := deps.database.Create(&series).Error; err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"title", "series"} {
		t.Run(target, func(t *testing.T) {
			key := "jav_" + target + "_translation_enabled"
			if err := dbpkg.Default().UpsertConfig(context.Background(), map[string]string{key: "false", "jav_" + target + "_translation_auto_enabled": "true"}); err != nil {
				t.Fatal(err)
			}
			manual := "/jav/items/" + strconv.FormatInt(item.ID, 10) + "/title-translation"
			id := item.ID
			if target == "series" {
				manual = "/jav/series/" + strconv.FormatInt(series.ID, 10) + "/name-translation"
				id = series.ID
			}
			for _, entry := range []struct{ path, body string }{
				{manual, `{}`},
				{"/jav/" + target + "-translation/items", `{"ids":[` + strconv.FormatInt(id, 10) + `]}`},
				{"/jav/" + target + "-translation/batch", `{"scope":"missing"}`},
			} {
				response := translationAPIRequest(router, "POST", entry.path, entry.body)
				if response.Code != http.StatusBadRequest {
					t.Fatalf("%s: %d %s", entry.path, response.Code, response.Body)
				}
			}
		})
	}
}

func TestDisablingTranslationMasterStopsQueuedBatchWork(t *testing.T) {
	deps := newTestDependencies()
	for _, target := range []string{"title", "series"} {
		t.Run(target, func(t *testing.T) {
			entered, finish := make(chan struct{}), make(chan struct{})
			var calls atomic.Int32
			router := titleTranslationRouter(deps, t, func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) == 1 {
					close(entered)
					<-finish
				}
				_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"译文"}}]}`))
			})
			var release sync.Once
			defer release.Do(func() { close(finish) })
			configureGroupedTranslation(deps, t)
			key := "jav_" + target + "_translation_enabled"
			if err := dbpkg.Default().UpsertConfig(context.Background(), map[string]string{key: "true"}); err != nil {
				t.Fatal(err)
			}
			var err error
			if target == "title" {
				err = deps.database.Create(&[]models.Jav{{Code: "STOP-001", Title: "One"}, {Code: "STOP-002", Title: "Two"}}).Error
			} else {
				err = deps.database.Create(&[]models.JavSeries{{Name: "One"}, {Name: "Two"}}).Error
			}
			if err != nil {
				t.Fatal(err)
			}
			response := translationAPIRequest(router, "POST", "/jav/"+target+"-translation/batch", `{}`)
			if response.Code != http.StatusAccepted {
				t.Fatalf("start: %d %s", response.Code, response.Body)
			}
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("batch did not start")
			}
			response = translationAPIRequest(router, "PATCH", "/config", `{"`+key+`":false}`)
			if response.Code != 200 {
				t.Fatalf("disable: %s", response.Body)
			}
			release.Do(func() { close(finish) })
			job := false
			if target == "series" {
				job = true
			}
			waitTextTranslationBatch(t, job)
			if calls.Load() != 1 {
				t.Fatalf("provider calls after disabling = %d", calls.Load())
			}
		})
	}
}
