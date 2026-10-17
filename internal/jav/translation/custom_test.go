package translation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
