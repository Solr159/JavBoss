package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"javboss/internal/common"
	dbpkg "javboss/internal/db"
	"javboss/internal/jav/translation"
	"javboss/internal/models"
)

func TestTitleTranslationDraftTestDoesNotSaveSettingsOrTitles(t *testing.T) {
	calls := 0
	router := titleTranslationRouter(t, func(w http.ResponseWriter, r *http.Request) {
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
	if err := dbpkg.UpsertConfig(context.Background(), map[string]string{"jav_title_translation_api_key": "saved-key", "jav_title_translation_model": "saved-model", "jav_title_translation_enabled": "false"}); err != nil {
		t.Fatal(err)
	}
	item := models.Jav{Code: "SAMPLE-001", Title: "Original", ZhTitle: "人工译名"}
	common.DB.Create(&item)
	before, _ := dbpkg.ListConfig(context.Background())
	response := translationAPIRequest(router, "POST", "/jav/title-translation/test", `{"provider":"deepseek","profile":{"model":"draft-model","thinking":true},"title":"Sample","prompt":"Draft prompt"}`)
	if response.Code != 200 || !strings.Contains(response.Body.String(), "示例译文") || strings.Contains(response.Body.String(), "saved-key") {
		t.Fatalf("test=%d %s", response.Code, response.Body)
	}
	after, _ := dbpkg.ListConfig(context.Background())
	a, _ := json.Marshal(before)
	b, _ := json.Marshal(after)
	if string(a) != string(b) {
		t.Fatal("sample saved draft configuration")
	}
	var stored models.Jav
	common.DB.First(&stored, item.ID)
	if stored.ZhTitle != item.ZhTitle {
		t.Fatal("sample changed library title")
	}
	response = translationAPIRequest(router, "POST", "/jav/title-translation/test", `{"provider":"deepseek","profile":{"model":"draft-model","api_key":""},"title":"Sample"}`)
	if response.Code != 400 || calls != 1 {
		t.Fatal("explicitly cleared key was reused")
	}
}

func TestTitleTranslationConfigDisplayPreferenceIsIndependentAndDefaultsOn(t *testing.T) {
	router := titleTranslationRouter(t, func(w http.ResponseWriter, r *http.Request) { t.Error("configuration should not request translation") })
	response := translationAPIRequest(router, "GET", "/config", "")
	var cfg map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &cfg); err != nil || response.Code != 200 || cfg["jav_title_prefer_chinese"] != "true" {
		t.Fatalf("default config: %d %s", response.Code, response.Body)
	}
	for _, preference := range []bool{false, true} {
		body, _ := json.Marshal(map[string]bool{"jav_title_prefer_chinese": preference})
		response = translationAPIRequest(router, "PATCH", "/config", string(body))
		if response.Code != 200 {
			t.Fatalf("preference: %d %s", response.Code, response.Body)
		}
		for _, enabled := range []bool{true, false} {
			body, _ := json.Marshal(map[string]bool{"jav_title_translation_enabled": enabled})
			response = translationAPIRequest(router, "PATCH", "/config", string(body))
			if err := json.Unmarshal(response.Body.Bytes(), &cfg); err != nil || response.Code != 200 || cfg["jav_title_translation_enabled"] != strconv.FormatBool(enabled) || cfg["jav_title_prefer_chinese"] != strconv.FormatBool(preference) || cfg["jav_title_translation_default_prompt"] != translation.DefaultPrompt {
				t.Fatalf("config: %d %s", response.Code, response.Body)
			}
			response = translationAPIRequest(router, "GET", "/config", "")
			if err := json.Unmarshal(response.Body.Bytes(), &cfg); err != nil || cfg["jav_title_prefer_chinese"] != strconv.FormatBool(preference) {
				t.Fatalf("persisted preference: %s", response.Body)
			}
		}
	}
}

func TestTitleTranslationBatchDefaultsToMissingAndRequiresOverwriteConfirmation(t *testing.T) {
	calls := 0
	router := titleTranslationRouter(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"新译文"}}]}`))
	})
	dbpkg.UpsertConfig(context.Background(), map[string]string{"jav_title_translation_enabled": "false", "jav_title_translation_api_key": "key", "jav_title_translation_model": "model"})
	items := []models.Jav{{Code: "MISSING-001", Title: "Missing"}, {Code: "MANUAL-002", Title: "Manual", ZhTitle: "人工校对"}, {Code: "EMPTY-003", Title: " "}}
	if err := common.DB.Create(&items).Error; err != nil {
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
	common.DB.First(&stored, items[1].ID)
	if stored.ZhTitle != "人工校对" {
		t.Fatal("default batch overwrote manual title")
	}
}

func TestTitleTranslationManualPreviewInheritsSavedReasoning(t *testing.T) {
	var modes []string
	router := titleTranslationRouter(t, func(w http.ResponseWriter, r *http.Request) {
		var payload struct{ Thinking struct{ Type string } }
		json.NewDecoder(r.Body).Decode(&payload)
		modes = append(modes, payload.Thinking.Type)
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"示例译文"}}]}`))
	})
	dbpkg.UpsertConfig(context.Background(), map[string]string{"jav_title_translation_api_key": "key", "jav_title_translation_model": "model", "jav_title_translation_thinking": "true"})
	item := models.Jav{Code: "INHERIT-001", Title: "Original", ZhTitle: "Existing"}
	common.DB.Create(&item)
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

func TestTitleTranslationConfigReadsLegacySettingsButOnlyWritesProfiles(t *testing.T) {
	router := titleTranslationRouter(t, func(http.ResponseWriter, *http.Request) { t.Error("unexpected provider request") })
	legacy := map[string]string{
		"jav_title_translation_api_key":  "legacy-secret",
		"jav_title_translation_model":    "legacy-model",
		"jav_title_translation_thinking": "true",
	}
	if err := dbpkg.UpsertConfig(context.Background(), legacy); err != nil {
		t.Fatal(err)
	}
	checkPublic := func(body []byte, model string, thinking bool) {
		t.Helper()
		var cfg map[string]string
		if err := json.Unmarshal(body, &cfg); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), "secret") || cfg["jav_title_translation_api_key_set"] != "true" || cfg["jav_title_translation_model"] != model || cfg["jav_title_translation_thinking"] != strconv.FormatBool(thinking) || cfg["jav_title_translation_catalog"] == "" || cfg["jav_title_translation_capabilities"] == "" {
			t.Fatalf("public config=%s", body)
		}
		var profiles map[string]translation.Profile
		if err := json.Unmarshal([]byte(cfg[translation.ProfilesKey]), &profiles); err != nil {
			t.Fatal(err)
		}
		p := profiles["deepseek"]
		if p.Model != model || p.Thinking != thinking || !p.APIKeySet || p.APIKey != "" {
			t.Fatalf("public profile=%+v", p)
		}
	}
	response := translationAPIRequest(router, "GET", "/config", "")
	if response.Code != http.StatusOK {
		t.Fatalf("legacy config=%d %s", response.Code, response.Body)
	}
	checkPublic(response.Body.Bytes(), "legacy-model", true)
	// Like other unknown configuration keys, obsolete write fields are ignored.
	response = translationAPIRequest(router, "PATCH", "/config", `{"jav_title_translation_api_key":"ignored-secret","jav_title_translation_model":"ignored-model","jav_title_translation_thinking":false}`)
	if response.Code != http.StatusOK {
		t.Fatalf("obsolete fields=%d %s", response.Code, response.Body)
	}
	checkPublic(response.Body.Bytes(), "legacy-model", true)
	response = translationAPIRequest(router, "PATCH", "/config", `{"jav_title_translation_profiles":{"deepseek":{"model":"profile-model","thinking":false}},"jav_title_translation_prompt":" Custom prompt "}`)
	if response.Code != http.StatusOK {
		t.Fatalf("profile fields=%d %s", response.Code, response.Body)
	}
	checkPublic(response.Body.Bytes(), "profile-model", false)
	stored, err := dbpkg.ListConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range legacy {
		if stored[key] != value {
			t.Fatalf("legacy field %s changed", key)
		}
	}
	p := translation.ActiveProfile(stored)
	if p.APIKey != "legacy-secret" || p.Model != "profile-model" || p.Thinking || stored["jav_title_translation_prompt"] != "Custom prompt" {
		t.Fatal("profile or prompt was not saved correctly")
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
