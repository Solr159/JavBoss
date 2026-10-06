package translation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDeepSeekModelDiscoveryAndTranslation(t *testing.T) {
	expectedThinking, expectedMaxTokens := "enabled", 8192
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Error("missing bearer key")
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/models" {
			_, _ = w.Write([]byte(`{"data":[{"id":"new-model","name":"New official model"},{"id":"new-model"},{"id":"other-model"}]}`))
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
			t.Error("unexpected endpoint")
		}
		var req struct {
			Model     string                           `json:"model"`
			Messages  []struct{ Role, Content string } `json:"messages"`
			Thinking  struct{ Type string }            `json:"thinking"`
			MaxTokens int                              `json:"max_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Model != "new-model" || req.Messages[0].Content != "Custom prompt" || req.Messages[1].Content != "雨の日の出会い" || req.Thinking.Type != expectedThinking || req.MaxTokens != expectedMaxTokens {
			t.Errorf("unexpected translation request: %+v", req)
		}
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"reasoning_content":"Compare possible translations before choosing a title.","content":" 雨天相遇 "}}]}`))
	}))
	defer upstream.Close()
	client := &Client{BaseURL: upstream.URL, HTTP: upstream.Client()}
	models, err := client.Models(context.Background(), "test-secret")
	if err != nil || len(models) != 2 || models[0].Name != "New official model" || models[1].Name != "other-model" {
		t.Fatalf("models=%v err=%v", models, err)
	}
	title, err := client.Translate(context.Background(), "test-secret", "new-model", "Custom prompt", "雨の日の出会い", true)
	if err != nil || title != "雨天相遇" {
		t.Fatalf("title=%q err=%v", title, err)
	}
	expectedThinking, expectedMaxTokens = "disabled", 1024
	title, err = client.Translate(context.Background(), "test-secret", "new-model", "Custom prompt", "雨の日の出会い", false)
	if err != nil || title != "雨天相遇" {
		t.Fatalf("title=%q err=%v", title, err)
	}
}

func TestDeepSeekRejectsInvalidOrIncompleteResponses(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		status     int
	}{
		{"invalid key", `{"error":{"message":"test-secret"}}`, 401},
		{"truncated title", `{"choices":[{"finish_reason":"length","message":{"content":"部分标题"}}]}`, 200},
		{"empty title", `{"choices":[{"finish_reason":"stop","message":{"content":" "}}]}`, 200},
		{"invalid json", `not json`, 200},
	} {
		t.Run(tt.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer upstream.Close()
			client := &Client{BaseURL: upstream.URL, HTTP: upstream.Client()}
			_, err := client.Translate(context.Background(), "test-secret", "model", "", "source", true)
			if err == nil || strings.Contains(err.Error(), "test-secret") {
				t.Fatalf("error=%v", err)
			}
			if tt.status == 401 {
				var apiError *APIError
				if !errors.As(err, &apiError) || apiError.StatusCode != 401 {
					t.Fatalf("error=%v", err)
				}
			}
		})
	}
}
