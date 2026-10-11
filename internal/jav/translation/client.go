package translation

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"javboss/internal/util"
)

type Model struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type APIError struct{ StatusCode int }

func (e *APIError) Error() string {
	return fmt.Sprintf("translation provider returned HTTP %d", e.StatusCode)
}

type ConfigError struct{ Message string }

func (e *ConfigError) Error() string { return e.Message }

type Client struct {
	BaseURL string // Test server override; production endpoints come from the selected profile.
	HTTP    *http.Client
}

func NewClient() *Client {
	client := util.NewHTTPClient(3 * time.Minute)
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{HTTP: client}
}
func (c *Client) request(ctx context.Context, provider string, p Profile, method, path string, payload any) (*http.Response, error) {
	base := p.Endpoint(provider)
	if c.BaseURL != "" {
		base = c.BaseURL
	}
	var reader io.Reader
	if payload != nil {
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode translation request")
		}
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, reader)
	if err != nil {
		return nil, fmt.Errorf("invalid translation endpoint")
	}
	switch p.WireProtocol(provider) {
	case "anthropic":
		req.Header.Set("x-api-key", p.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	case "gemini":
		req.Header.Set("x-goog-api-key", p.APIKey)
	default:
		if p.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+p.APIKey)
		}
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req)
}

// Keep network errors generic so credentials in custom URLs or headers never reach the UI.
func (c *Client) do(req *http.Request) (*http.Response, error) {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("translation request failed")
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, &APIError{StatusCode: resp.StatusCode}
	}
	return resp, nil
}
func readJSON(resp *http.Response, result any) error {
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return fmt.Errorf("invalid translation response size")
	}
	if err = json.Unmarshal(data, result); err != nil {
		return fmt.Errorf("invalid translation response")
	}
	return nil
}

// Models uses official list endpoints, including Claude/Gemini pagination. A failed discovery
// never replaces a manually entered model name.
func (c *Client) Models(ctx context.Context, provider string, p Profile) ([]Model, error) {
	if err := p.Validate(provider); err != nil {
		return nil, err
	}
	protocol := p.WireProtocol(provider)
	path := "/models"
	if protocol == "anthropic" {
		path += "?limit=1000"
	}
	if protocol == "gemini" {
		path += "?pageSize=1000"
	}
	models := []Model{}
	seen := map[string]bool{}
	for page := 0; page < 50; page++ {
		resp, err := c.request(ctx, provider, p, http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}
		var result struct {
			Data []struct {
				ID          string
				Name        string
				DisplayName string `json:"display_name"`
			} `json:"data"`
			Models []struct {
				Name                       string
				DisplayName                string
				SupportedGenerationMethods []string
			} `json:"models"`
			HasMore       bool   `json:"has_more"`
			LastID        string `json:"last_id"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err = readJSON(resp, &result); err != nil {
			return nil, err
		}
		add := func(id, name string) {
			id = strings.TrimPrefix(strings.TrimSpace(id), "models/")
			if id == "" || seen[id] {
				return
			}
			seen[id] = true
			if name == "" {
				name = id
			}
			models = append(models, Model{ID: id, Name: name})
		}
		for _, m := range result.Data {
			name := m.Name
			if name == "" {
				name = m.DisplayName
			}
			add(m.ID, name)
		}
		for _, m := range result.Models {
			if slices.Contains(m.SupportedGenerationMethods, "generateContent") {
				add(m.Name, m.DisplayName)
			}
		}
		switch {
		case protocol == "gemini" && result.NextPageToken != "":
			path = "/models?pageSize=1000&pageToken=" + url.QueryEscape(result.NextPageToken)
		case protocol == "anthropic" && result.HasMore && result.LastID != "":
			path = "/models?limit=1000&after_id=" + url.QueryEscape(result.LastID)
		default:
			if len(models) == 0 {
				return nil, fmt.Errorf("no translation models returned")
			}
			return models, nil
		}
	}
	return nil, fmt.Errorf("too many model pages")
}

func (c *Client) Translate(ctx context.Context, provider string, p Profile, prompt, title string, override *bool) (string, error) {
	if err := p.Validate(provider); err != nil {
		return "", &ConfigError{err.Error()}
	}
	if p.Model == "" || (p.Endpoint(provider) == "" && provider != "custom") || (p.APIKey == "" && provider != "custom") {
		return "", &ConfigError{"请先配置 API 地址、密钥和模型 / Configure an endpoint, API key and model"}
	}
	caps := ModelCapabilities(provider, p)
	if caps.Reasoning == "unknown" {
		return "", &ConfigError{"尚未识别此模型的参数能力，请使用自定义服务商配置 / Configure this model with the custom provider"}
	}
	thinking := p.Thinking
	if caps.Reasoning == "always" {
		thinking = true
	}
	if override != nil {
		thinking = *override
		if !thinking && caps.Reasoning == "always" {
			return "", &ConfigError{"此模型不能关闭推理，请选择支持快速翻译的模型 / This model cannot disable reasoning"}
		}
	}
	if caps.Reasoning == "none" {
		thinking = false
	}
	if p.Temperature != nil && *p.Temperature > caps.TemperatureMax {
		return "", &ConfigError{"温度超出模型支持范围 / Temperature exceeds the model range"}
	}
	if thinking && p.Effort != "" && !slices.Contains(caps.Efforts, p.Effort) && provider != "custom" {
		return "", &ConfigError{"此模型不支持所选推理强度 / Unsupported reasoning effort"}
	}
	if strings.TrimSpace(prompt) == "" {
		prompt = DefaultPrompt
	}
	if provider == "custom" {
		return c.translateCustom(ctx, p, prompt, title, thinking)
	}
	protocol := p.WireProtocol(provider)
	maxTokens := 1024
	if thinking {
		maxTokens = 8192
	}
	var payload map[string]any
	path := "/chat/completions"
	generation := map[string]any{}
	switch protocol {
	case "responses":
		path = "/responses"
		payload = map[string]any{"model": p.Model, "instructions": prompt, "input": title, "max_output_tokens": maxTokens, "store": false}
	case "anthropic":
		path = "/messages"
		payload = map[string]any{"model": p.Model, "system": prompt, "messages": []map[string]string{{"role": "user", "content": title}}, "max_tokens": maxTokens}
	case "gemini":
		path = "/models/" + url.PathEscape(strings.TrimPrefix(p.Model, "models/")) + ":generateContent"
		generation["maxOutputTokens"] = maxTokens
		payload = map[string]any{"systemInstruction": map[string]any{"parts": []map[string]string{{"text": prompt}}}, "contents": []map[string]any{{"role": "user", "parts": []map[string]string{{"text": title}}}}, "generationConfig": generation}
	default:
		payload = map[string]any{"model": p.Model, "messages": []map[string]string{{"role": "system", "content": prompt}, {"role": "user", "content": title}}, "max_tokens": maxTokens, "stream": false}
	}
	if p.Temperature != nil && caps.Temperature && (!thinking || caps.TemperatureWithThinking) {
		if protocol == "gemini" {
			generation["temperature"] = *p.Temperature
		} else {
			payload["temperature"] = *p.Temperature
		}
	}
	effort := p.Effort
	budget := 4096
	switch effort {
	case "low":
		budget = 1024
	case "high", "xhigh", "max":
		budget = 6144
	}
	switch caps.Style {
	case "thinking":
		mode := "disabled"
		if thinking {
			mode = "enabled"
		}
		payload["thinking"] = map[string]any{"type": mode}
		if thinking && effort != "" {
			payload["reasoning_effort"] = effort
		}
	case "effort":
		if !thinking {
			effort = "none"
		} else if effort == "" && caps.Reasoning == "toggle" {
			// Some switchable models default to no reasoning unless an effort is supplied.
			effort = "medium"
			if !slices.Contains(caps.Efforts, effort) {
				effort = "high"
			}
		}
		if effort != "" {
			if protocol == "responses" {
				payload["reasoning"] = map[string]any{"effort": effort}
			} else {
				payload["reasoning_effort"] = effort
			}
		}
	case "adaptive":
		mode := "disabled"
		if thinking {
			mode = "adaptive"
		}
		payload["thinking"] = map[string]any{"type": mode}
		if thinking && effort != "" {
			payload["output_config"] = map[string]any{"effort": effort}
		}
	case "budget":
		if protocol == "gemini" {
			if !thinking {
				budget = 0
			} else if effort == "" {
				budget = -1
			}
			generation["thinkingConfig"] = map[string]any{"thinkingBudget": budget}
		} else {
			if thinking {
				payload["thinking"] = map[string]any{"type": "enabled", "budget_tokens": budget}
			} else {
				payload["thinking"] = map[string]any{"type": "disabled"}
			}
		}
	case "level":
		if effort != "" {
			generation["thinkingConfig"] = map[string]any{"thinkingLevel": effort}
		}
	case "qwen_budget":
		if caps.Reasoning == "toggle" {
			payload["enable_thinking"] = thinking
		}
		if thinking && effort != "" {
			payload["thinking_budget"] = budget
		}
	}
	// Some Qwen thinking models require SSE; consume the final answer server-side.
	streaming := provider == "qwen" && thinking
	if streaming {
		payload["stream"] = true
	}
	resp, err := c.request(ctx, provider, p, http.MethodPost, path, payload)
	if err != nil {
		return "", err
	}
	var translated string
	if streaming {
		translated, err = readChatStream(resp)
	} else {
		translated, err = readTranslation(resp, protocol)
	}
	if err != nil {
		return "", err
	}
	translated = strings.TrimSpace(translated)
	if translated == "" || len(translated) > 8192 {
		return "", fmt.Errorf("provider returned an empty or oversized title")
	}
	return translated, nil
}

func readTranslation(resp *http.Response, protocol string) (string, error) {
	var result struct {
		Choices []struct {
			Message      struct{ Content string }
			FinishReason string `json:"finish_reason"`
		}
		Content []struct {
			Type string
			Text string
		}
		StopReason string `json:"stop_reason"`
		Status     string
		Output     []struct {
			Type    string
			Content []struct {
				Type string
				Text string
			}
		}
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text    string
					Thought bool
				}
			}
			FinishReason string
		}
	}
	if err := readJSON(resp, &result); err != nil {
		return "", err
	}
	var out strings.Builder
	switch protocol {
	case "responses":
		if result.Status != "completed" {
			return "", fmt.Errorf("incomplete translation")
		}
		for _, item := range result.Output {
			if item.Type == "message" {
				for _, part := range item.Content {
					if part.Type == "output_text" {
						out.WriteString(part.Text)
					}
				}
			}
		}
	case "anthropic":
		if result.StopReason != "end_turn" {
			return "", fmt.Errorf("incomplete translation")
		}
		for _, part := range result.Content {
			if part.Type == "text" {
				out.WriteString(part.Text)
			}
		}
	case "gemini":
		if len(result.Candidates) == 0 || result.Candidates[0].FinishReason != "STOP" {
			return "", fmt.Errorf("incomplete translation")
		}
		for _, part := range result.Candidates[0].Content.Parts {
			if !part.Thought {
				out.WriteString(part.Text)
			}
		}
	default:
		if len(result.Choices) == 0 || result.Choices[0].FinishReason != "stop" {
			return "", fmt.Errorf("incomplete translation")
		}
		out.WriteString(result.Choices[0].Message.Content)
	}
	return out.String(), nil
}
func readChatStream(resp *http.Response) (string, error) {
	defer resp.Body.Close()
	scanner := bufio.NewScanner(io.LimitReader(resp.Body, (1<<20)+1))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	var out strings.Builder
	finished := false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta        struct{ Content string }
				FinishReason *string `json:"finish_reason"`
			}
			Error json.RawMessage
		}
		if json.Unmarshal([]byte(data), &chunk) != nil || len(chunk.Error) > 0 {
			return "", fmt.Errorf("invalid translation stream")
		}
		if len(chunk.Choices) > 0 {
			choice := chunk.Choices[0]
			out.WriteString(choice.Delta.Content)
			if choice.FinishReason != nil {
				if *choice.FinishReason != "stop" {
					return "", fmt.Errorf("incomplete translation")
				}
				finished = true
			}
		}
	}
	if scanner.Err() != nil || !finished {
		return "", fmt.Errorf("incomplete translation stream")
	}
	return out.String(), nil
}
