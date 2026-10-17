package translation

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestReadAPIError(t *testing.T) {
	for _, tt := range []struct{ name, body, code, kind, message string }{
		{"openai", `{"error":{"code":"credit_balance_exhausted","type":"insufficient_quota","message":"Add credits."}}`, "credit_balance_exhausted", "insufficient_quota", "Add credits."},
		{"anthropic", `{"error":{"type":"rate_limit_error","message":"Too many requests."}}`, "", "rate_limit_error", "Too many requests."},
		{"google", `{"error":{"code":429,"status":"RESOURCE_EXHAUSTED","message":"Quota exceeded."}}`, "429", "RESOURCE_EXHAUSTED", "Quota exceeded."},
		{"custom", `{"code":"model_missing","message":"Select a model."}`, "model_missing", "", "Select a model."},
		{"string", `{"error":"Service unavailable."}`, "", "", "Service unavailable."},
		{"html", `<html>Gateway unavailable</html>`, "", "", ""},
		{"invalid", `{"error":[]}`, "", "", ""},
		{"oversized", `{"error":"` + strings.Repeat("x", 17<<10) + `"}`, "", "", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "https://example.com/v1/responses", nil)
			resp := &http.Response{StatusCode: 429, Body: io.NopCloser(strings.NewReader(tt.body))}
			got := readAPIError(resp, req, nil)
			if got.StatusCode != 429 || got.Code != tt.code || got.Type != tt.kind || got.Message != tt.message {
				t.Fatalf("unexpected error: %+v", got)
			}
		})
	}
}

func TestReadAPIErrorRedactsCredentialsBeforeTruncation(t *testing.T) {
	key, headerKey, queryKey := "configured-secret", "header-secret", "query+secret/value"
	req := httptest.NewRequest("POST", "https://example.com?token="+url.QueryEscape(queryKey), nil)
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("X-Custom-Key", headerKey)
	message := "Invalid key: " + strings.Join([]string{key, headerKey, queryKey, url.QueryEscape(queryKey), url.PathEscape(queryKey), "sk-partiallymasked123"}, " ") + "\n" + strings.Repeat("错", 1100)
	body, _ := json.Marshal(map[string]any{"error": map[string]string{"code": key, "type": headerKey, "message": message}})
	got := readAPIError(&http.Response{StatusCode: 401, Body: io.NopCloser(strings.NewReader(string(body)))}, req, []string{key})
	for _, secret := range []string{key, headerKey, queryKey, url.QueryEscape(queryKey), url.PathEscape(queryKey), "sk-partiallymasked123"} {
		if strings.Contains(got.Error(), secret) {
			t.Fatalf("credential leaked: %s", got)
		}
	}
	if got.Code != "" || got.Type != "" || !strings.Contains(got.Message, "[redacted]") || len([]rune(got.Message)) != 1001 || strings.Contains(got.Message, "\n") {
		t.Fatalf("unexpected sanitized error: %+v", got)
	}
}
