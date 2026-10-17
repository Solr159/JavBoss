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

func TestTitleTranslationErrorReasons(t *testing.T) {
	for _, tt := range []struct{ code, kind, want string }{
		{"credit_balance_exhausted", "insufficient_quota", "余额已耗尽"},
		{"organization_spend_limit_exceeded", "insufficient_quota", "组织支出"},
		{"project_spend_limit_exceeded", "insufficient_quota", "项目支出"},
		{"organization_usage_limit_exceeded", "insufficient_quota", "组织用量"},
		{"", "insufficient_quota", "可用额度不足"},
		{"slow_down", "rate_limit_error", "请求过于频繁"},
		{"rate_limit_exceeded", "", "请求过于频繁"},
		{"", "rate_limit_error", "请求过于频繁"},
		{"unknown", "", "请求受到限制"},
	} {
		t.Run(tt.code+tt.kind, func(t *testing.T) {
			status, zh, en := titleTranslationErrorDetails(&translation.APIError{StatusCode: 429, Code: tt.code, Type: tt.kind, Message: "Provider explanation."})
			if status != 429 || !strings.Contains(zh, tt.want) || !strings.Contains(zh, "HTTP 429") || !strings.Contains(zh, "Provider explanation.") || !strings.Contains(en, "Provider explanation.") {
				t.Fatalf("status=%d zh=%s en=%s", status, zh, en)
			}
		})
	}
}

func TestTitleTranslationQuotaErrorReachesSingleAndBatchClients(t *testing.T) {
	router := titleTranslationRouter(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "credit_balance_exhausted", "type": "insufficient_quota", "message": "Add credits for secret-key."}})
	})
	if err := dbpkg.UpsertConfig(context.Background(), map[string]string{
		"jav_title_translation_enabled": "true", "jav_title_translation_api_key": "secret-key", "jav_title_translation_model": "model",
	}); err != nil {
		t.Fatal(err)
	}
	item := models.Jav{Code: "QUOTA-001", Title: "Original title", ZhTitle: "原有译文"}
	if err := common.DB.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	path := "/jav/items/" + strconv.FormatInt(item.ID, 10) + "/title-translation"
	response := translationAPIRequest(router, "POST", path, `{"refresh":true}`)
	if response.Code != 429 {
		t.Fatalf("single=%d %s", response.Code, response.Body)
	}
	check := func(message string) {
		t.Helper()
		for _, part := range []string{"credit_balance_exhausted", "余额已耗尽", "Add credits for [redacted]."} {
			if !strings.Contains(message, part) {
				t.Fatalf("missing %q: %s", part, message)
			}
		}
		if strings.Contains(message, "secret-key") || strings.Contains(message, "请求过于频繁") {
			t.Fatalf("incorrect diagnostic: %s", message)
		}
	}
	check(response.Body.String())
	response = translationAPIRequest(router, "POST", "/jav/title-translation/batch", "")
	if response.Code != http.StatusAccepted {
		t.Fatalf("batch=%d %s", response.Code, response.Body)
	}
	status := waitTitleTranslationBatch(t)
	check(status.ErrorZH)
	var stored models.Jav
	if err := common.DB.First(&stored, item.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.ZhTitle != item.ZhTitle {
		t.Fatal("failed request replaced existing translation")
	}
}
