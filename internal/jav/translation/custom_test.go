package translation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCustomTranslationResponseExtraction(t *testing.T) {
	const multipleOutput = `{"status":"completed","output":[{"type":"reasoning","content":[{"type":"output_text","text":"hidden reasoning"}]},{"type":"message","content":[{"type":"output_text","text":" 雨"},{"type":"other","text":"ignored"},{"type":"output_text","text":"天"}]},{"type":"message","content":[{"type":"output_text","text":"晴 "}]}],"custom":{"a/b":["自定义"]}}`
	for _, tc := range []struct {
		name, response, want string
		fields               map[string]any
		wantErr              bool
	}{
		{name: "responses reasoning first omitted path", response: responsesResponse, want: "雨天"},
		{name: "responses reasoning first empty path", response: responsesResponse, fields: map[string]any{"response_path": ""}, want: "雨天"},
		{name: "responses all output text", response: multipleOutput, want: "雨天晴"},
		{name: "chat default", response: chatResponse, want: "雨天"},
		{name: "anthropic default", response: claudeResponse, want: "雨天"},
		{name: "gemini default", response: geminiResponse, want: "雨天"},
		{name: "responses incomplete", response: strings.Replace(multipleOutput, `"completed"`, `"incomplete"`, 1), wantErr: true},
		{name: "responses missing status", response: `{"output":[{"type":"message","content":[{"type":"output_text","text":"partial"}]}]}`, wantErr: true},
		{name: "responses reasoning only", response: `{"status":"completed","output":[{"type":"reasoning","content":[{"type":"output_text","text":"hidden"}]}]}`, wantErr: true},
		{name: "responses no output text", response: `{"status":"completed","output":[{"type":"message","content":[{"type":"refusal","refusal":"refused"}]}]}`, wantErr: true},
		{name: "responses blank text", response: strings.Replace(responsesResponse, "雨天", " ", 1), wantErr: true},
		{name: "responses oversized text", response: strings.Replace(responsesResponse, "雨天", strings.Repeat("x", 8193), 1), wantErr: true},
		{name: "explicit pointer selects one part", response: multipleOutput, fields: map[string]any{"response_path": "/output/1/content/2/text"}, want: "天"},
		{name: "explicit arbitrary pointer in standard envelope", response: multipleOutput, fields: map[string]any{"response_path": "/custom/a~1b/0"}, want: "自定义"},
		{name: "explicit pointer not rewritten", response: responsesResponse, fields: map[string]any{"response_path": "/output/0/content/0/text"}, wantErr: true},
		{name: "explicit pointer must return text", response: multipleOutput, fields: map[string]any{"response_path": "/output"}, wantErr: true},
		{name: "standard completion check passes", response: multipleOutput, fields: map[string]any{"completion_path": "/status", "completion_value": "completed"}, want: "雨天晴"},
		{name: "standard completion check fails", response: multipleOutput, fields: map[string]any{"completion_path": "/status", "completion_value": "finished"}, wantErr: true},
		{name: "standard completion check missing value", response: multipleOutput, fields: map[string]any{"completion_path": "/status"}, wantErr: true},
		{name: "explicit pointer completion check fails", response: multipleOutput, fields: map[string]any{"response_path": "/custom/a~1b/0", "completion_path": "/status", "completion_value": "finished"}, wantErr: true},
		{name: "custom object needs explicit pointer", response: `{"done":true,"text":"雨天"}`, fields: map[string]any{"completion_path": "/done", "completion_value": true}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tc.response))
			}))
			defer upstream.Close()
			request := map[string]any{"url": upstream.URL, "body": map[string]any{"title": "{{title}}"}}
			for key, value := range tc.fields {
				request[key] = value
			}
			profile := Profile{Model: "custom-model", Request: request}
			title, err := (&Client{HTTP: upstream.Client()}).Translate(context.Background(), "custom", profile, "Translate", "Rainy day", nil)
			if (err != nil) != tc.wantErr || title != tc.want {
				t.Fatalf("title=%q err=%v; want title=%q error=%v", title, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestCustomTranslationRequiresCompleteResponse(t *testing.T) {
	for _, tc := range []struct {
		name, response, path, completionPath string
		completionValue                      any
		valid                                bool
	}{
		{"chat complete", chatResponse, "/choices/0/message/content", "", nil, true},
		{"chat truncated", `{"choices":[{"finish_reason":"length","message":{"content":"partial"}}]}`, "/choices/0/message/content", "", nil, false},
		{"chat missing status", `{"choices":[{"message":{"content":"partial"}}]}`, "/choices/0/message/content", "", nil, false},
		{"chat filtered", `{"choices":[{"finish_reason":"content_filter","message":{"content":"partial"}}]}`, "/choices/0/message/content", "", nil, false},
		{"responses complete", responsesResponse, "/output/1/content/0/text", "", nil, true},
		{"responses incomplete", `{"status":"incomplete","output":[{"type":"message","content":[{"type":"output_text","text":"partial"}]}]}`, "/output/0/content/0/text", "", nil, false},
		{"claude complete", claudeResponse, "/content/1/text", "", nil, true},
		{"claude truncated", `{"stop_reason":"max_tokens","content":[{"type":"text","text":"partial"}]}`, "/content/0/text", "", nil, false},
		{"gemini complete", geminiResponse, "/candidates/0/content/parts/1/text", "", nil, true},
		{"gemini truncated", `{"candidates":[{"finishReason":"MAX_TOKENS","content":{"parts":[{"text":"partial"}]}}]}`, "/candidates/0/content/parts/0/text", "", nil, false},
		{"custom complete", `{"done":true,"text":"雨天"}`, "/text", "/done", true, true},
		{"custom incomplete", `{"done":false,"text":"partial"}`, "/text", "/done", true, false},
		{"custom no status", `{"text":"partial"}`, "/text", "/done", true, false},
		{"custom no completion check", `{"text":"partial"}`, "/text", "", nil, false},
		{"custom missing expected status", `{"done":true,"text":"partial"}`, "/text", "/done", nil, false},
		{"custom strict status type", `{"done":"true","text":"partial"}`, "/text", "/done", true, false},
		{"custom string status", `{"status":"finished","text":"雨天"}`, "/text", "/status", "finished", true},
		{"empty output", `{"done":true,"text":" "}`, "/text", "/done", true, false},
		{"cannot bypass truncation", `{"done":true,"choices":[{"finish_reason":"length","message":{"content":"partial"}}]}`, "/choices/0/message/content", "/done", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tc.response))
			}))
			defer upstream.Close()
			profile := Profile{Model: "custom-model", Request: map[string]any{
				"url": upstream.URL, "body": map[string]any{"title": "{{title}}"},
				"response_path": tc.path, "completion_path": tc.completionPath, "completion_value": tc.completionValue,
			}}
			title, err := (&Client{HTTP: upstream.Client()}).Translate(context.Background(), "custom", profile, "Translate", "Rainy day", nil)
			if tc.valid && (err != nil || title != "雨天") {
				t.Fatalf("title=%q err=%v", title, err)
			}
			if !tc.valid && (err == nil || title != "") {
				t.Fatalf("incomplete response accepted: title=%q err=%v", title, err)
			}
		})
	}
}
