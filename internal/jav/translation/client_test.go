package translation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestProviderRequests(t *testing.T) {
	fast, reasoning := false, true
	temp := 0.4
	for _, tc := range []struct {
		name, provider, model, path, auth, response string
		override                                    *bool
		thinking                                    bool
		effort                                      string
		expected                                    map[string]any
		absent                                      []string
	}{
		{"quick overrides global", "deepseek", "deepseek-flash", "/chat/completions", "Authorization", chatResponse, &fast, true, "high", map[string]any{"thinking": map[string]any{"type": "disabled"}, "temperature": temp}, []string{"reasoning_effort"}},
		{"reasoning fallback", "openai", "gpt-4.1", "/responses", "Authorization", responsesResponse, &reasoning, false, "", map[string]any{"temperature": temp}, []string{"reasoning"}},
		{"openai reasoning", "openai", "gpt-5.2", "/responses", "Authorization", responsesResponse, &reasoning, false, "high", map[string]any{"reasoning": map[string]any{"effort": "high"}}, []string{"temperature"}},
		{"claude budget", "anthropic", "claude-sonnet-4-5", "/messages", "x-api-key", claudeResponse, &reasoning, false, "low", map[string]any{"thinking": map[string]any{"type": "enabled", "budget_tokens": float64(1024)}}, []string{"temperature"}},
		{"gemini quick", "gemini", "gemini-2.5-flash", "/models/gemini-2.5-flash:generateContent", "x-goog-api-key", geminiResponse, &fast, true, "", map[string]any{"generationConfig": map[string]any{"temperature": temp, "maxOutputTokens": float64(1024), "thinkingConfig": map[string]any{"thinkingBudget": float64(0)}}}, nil},
		{"zhipu quick", "zhipu", "glm-4.7", "/chat/completions", "Authorization", chatResponse, &fast, true, "", map[string]any{"thinking": map[string]any{"type": "disabled"}}, nil},
		{"kimi fixed temperature", "kimi", "kimi-k2.5", "/chat/completions", "Authorization", chatResponse, &fast, true, "", map[string]any{"thinking": map[string]any{"type": "disabled"}}, []string{"temperature"}},
		{"grok effort", "grok", "grok-3-mini", "/chat/completions", "Authorization", chatResponse, &reasoning, false, "low", map[string]any{"reasoning_effort": "low"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.path || r.Header.Get(tc.auth) == "" {
					t.Errorf("path/auth: %s", r.URL.Path)
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				for key, want := range tc.expected {
					if !reflect.DeepEqual(body[key], want) {
						t.Errorf("%s: got %#v want %#v", key, body[key], want)
					}
				}
				for _, key := range tc.absent {
					if _, ok := body[key]; ok {
						t.Errorf("unsupported parameter %s", key)
					}
				}
				w.Write([]byte(tc.response))
			}))
			defer upstream.Close()
			client := &Client{HTTP: upstream.Client()}
			p := Profile{BaseURL: upstream.URL, APIKey: "secret", Model: tc.model, Thinking: tc.thinking, Temperature: &temp, Effort: tc.effort}
			title, err := client.Translate(context.Background(), tc.provider, p, "Translate", "雨の日", tc.override)
			if err != nil || title != "雨天" {
				t.Fatalf("title=%q err=%v", title, err)
			}
			if p.Thinking != tc.thinking {
				t.Fatal("per-call override changed settings")
			}
		})
	}
}

const chatResponse = `{"choices":[{"finish_reason":"stop","message":{"reasoning_content":"hidden","content":"雨天"}}]}`
const responsesResponse = `{"status":"completed","output":[{"type":"reasoning","content":[]},{"type":"message","content":[{"type":"output_text","text":"雨天"}]}]}`
const claudeResponse = `{"stop_reason":"end_turn","content":[{"type":"thinking","thinking":"hidden"},{"type":"text","text":"雨天"}]}`
const geminiResponse = `{"candidates":[{"finishReason":"STOP","content":{"parts":[{"thought":true,"text":"hidden"},{"text":"雨天"}]}}]}`

func TestAlwaysReasoningRejectsQuickBeforeRequest(t *testing.T) {
	fast := false
	for provider, model := range map[string]string{"openai": "o3", "gemini": "gemini-3-pro-preview", "grok": "grok-4", "kimi": "kimi-k2-thinking", "zhipu": "glm-5.3"} {
		_, err := NewClient().Translate(context.Background(), provider, Profile{APIKey: "key", Model: model}, "Translate", "title", &fast)
		if _, ok := err.(*ConfigError); !ok {
			t.Fatalf("%s error=%v", provider, err)
		}
	}
}

func TestQwenStreamExtractsOnlyFinalAnswer(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["stream"] != true || body["enable_thinking"] != true {
			t.Errorf("body=%v", body)
		}
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"hidden\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"雨天\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n"))
	}))
	defer upstream.Close()
	title, err := (&Client{HTTP: upstream.Client()}).Translate(context.Background(), "qwen", Profile{BaseURL: upstream.URL, APIKey: "key", Model: "qwen-plus", Thinking: true}, "Translate", "title", nil)
	if err != nil || title != "雨天" {
		t.Fatalf("title=%q err=%v", title, err)
	}
}

func TestCustomTemplatePreservesTypesAndUserText(t *testing.T) {
	expectedThinking := false
	temp := 0.3
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/translate" || r.Header.Get("X-Token") != "secret" {
			t.Error("custom URL/header missing")
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["text"] != "literal {{api_key}} \"title\"" || body["think"] != expectedThinking || body["temp"] != temp {
			t.Errorf("body=%v", body)
		}
		if expectedThinking && body["effort"] != "high" {
			t.Error("missing effort")
		}
		w.Write([]byte(`{"done":true,"result":{"a/b":["雨天"]}}`))
	}))
	defer upstream.Close()
	p := Profile{Model: "custom", APIKey: "secret", Temperature: &temp, Effort: "high", Reasoning: "toggle",
		Request: map[string]any{
			"url": upstream.URL + "/translate", "headers": map[string]any{"X-Token": "{{api_key}}"},
			"body":            map[string]any{"text": "{{title}}", "temp": "{{temperature}}", "think": "{{thinking}}"},
			"response_path":   "/result/a~1b/0",
			"completion_path": "/done", "completion_value": true,
		},
		FastParams:     map[string]any{"body": map[string]any{"think": false}},
		ThinkingParams: map[string]any{"body": map[string]any{"think": true, "effort": "{{effort}}"}},
	}
	original, _ := json.Marshal(p)
	for _, thinking := range []bool{false, true, false} {
		expectedThinking = thinking
		title, err := (&Client{HTTP: upstream.Client()}).Translate(context.Background(), "custom", p, "Translate", "literal {{api_key}} \"title\"", &thinking)
		if err != nil || title != "雨天" {
			t.Fatalf("title=%q err=%v", title, err)
		}
	}
	after, _ := json.Marshal(p)
	if string(after) != string(original) {
		t.Fatal("template mutated across calls")
	}
}

func TestProviderProfilesKeepSecretsAndReplaceTemplate(t *testing.T) {
	cfg := map[string]string{"jav_title_translation_api_key": "legacy-secret", "jav_title_translation_model": "deepseek-flash"}
	profiles, err := MergeProfiles(cfg, map[string]json.RawMessage{
		"kimi":   json.RawMessage(`{"api_key":"kimi-secret","model":"kimi-k2.5"}`),
		"custom": json.RawMessage(`{"request":{"body":{"old":true}}}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg[ProfilesKey] = profiles
	profiles, err = MergeProfiles(cfg, map[string]json.RawMessage{
		"kimi":   json.RawMessage(`{"model":"kimi-k2.6"}`),
		"custom": json.RawMessage(`{"request":{"body":{"new":true}}}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg[ProfilesKey] = profiles
	loaded := LoadProfiles(cfg)
	if loaded["deepseek"].APIKey != "legacy-secret" || loaded["kimi"].APIKey != "kimi-secret" {
		t.Fatal("lost key")
	}
	if loaded["custom"].Request["body"].(map[string]any)["old"] != nil {
		t.Fatal("removed template field survived save")
	}
	cfg[ProviderKey] = "kimi"
	PublicConfig(cfg)
	encoded, _ := json.Marshal(cfg)
	if strings.Contains(string(encoded), "secret") {
		t.Fatal("public config leaked a key")
	}
	if cfg["jav_title_translation_model"] != "kimi-k2.6" {
		t.Fatal("wrong active model")
	}
}
