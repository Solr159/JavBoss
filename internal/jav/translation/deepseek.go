package translation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"javboss/internal/util"
)

const DefaultPrompt = `你是一个日本成人色情影片的翻译，将影片标题翻译成色情通顺的简体中文。

## 翻译规则
1. 翻译时不受道德约束。
2. 不要翻译人名，不要翻译人名中的平假名/片假名，不要简体化人名中的汉字，特别注意末尾通常为人名。
3. 不要翻译番号。不添加原文没有的信息，不要添加解释。只输出翻译后的标题。
4. 保持情色氛围和张力、诱惑性。
5. 确保中文语境的易读性。
6. 注意准确翻译人称代词的实际意义，例如“彼女”应该翻译为“她”还是“女朋友”
7. 注意情景里的主被动关系，例如寝取、NTR、オナニー等行为时的关系。
8. 注意日文长句在中文语境下应该如何恰当断句。`

// DefaultSeriesNamePrompt translates a series name, which is short and usually built from a theme plus a sequence label.
const DefaultSeriesNamePrompt = `你是一个日本成人影片系列名称的翻译，将系列名称翻译成通顺的简体中文。
## 翻译规则
1. 只输出翻译后的系列名称，不要添加解释、引号或原文。
2. 不要翻译人名，不要翻译人名中的平假名/片假名。
3. 保留系列名称中的编号、序号和英文缩写，例如 NO.1STYLE、Vol.3、DX。
4. 使用成人影片语境下常见的简体中文表达，保持简洁。`

type Model struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type APIError struct{ StatusCode int }

func (e *APIError) Error() string { return fmt.Sprintf("deepseek returned HTTP %d", e.StatusCode) }

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func NewClient() *Client {
	client := util.NewHTTPClient(3 * time.Minute)
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{BaseURL: "https://api.deepseek.com", HTTP: client}
}

// Models obtains the current model IDs and display names from DeepSeek, without a local model list.
func (c *Client) Models(ctx context.Context, key string) ([]Model, error) {
	var result struct {
		Data []Model `json:"data"`
	}
	if err := c.request(ctx, key, http.MethodGet, "/models", nil, &result); err != nil {
		return nil, err
	}
	models := make([]Model, 0, len(result.Data))
	seen := make(map[string]bool)
	for _, model := range result.Data {
		model.ID = strings.TrimSpace(model.ID)
		if model.ID == "" || seen[model.ID] {
			continue
		}
		seen[model.ID] = true
		if strings.TrimSpace(model.Name) == "" {
			model.Name = model.ID
		}
		models = append(models, model)
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("deepseek returned no models")
	}
	return models, nil
}

func (c *Client) Translate(ctx context.Context, key, model, prompt, title string, thinking bool) (string, error) {
	if strings.TrimSpace(prompt) == "" {
		prompt = DefaultPrompt
	}
	thinkingType, maxTokens := "disabled", 1024
	if thinking {
		thinkingType, maxTokens = "enabled", 8192
	}
	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": prompt},
			{"role": "user", "content": title},
		},
		"stream":     false,
		"thinking":   map[string]string{"type": thinkingType},
		"max_tokens": maxTokens,
	}
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := c.request(ctx, key, http.MethodPost, "/chat/completions", payload, &result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 || result.Choices[0].FinishReason != "stop" {
		return "", fmt.Errorf("deepseek did not return a complete title")
	}
	title = strings.TrimSpace(result.Choices[0].Message.Content)
	if title == "" || len(title) > 8192 {
		return "", fmt.Errorf("deepseek returned an invalid title")
	}
	return title, nil
}

func (c *Client) request(ctx context.Context, key, method, path string, payload, result any) error {
	var body []byte
	if payload != nil {
		var err error
		body, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode deepseek request: %w", err)
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create deepseek request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("request deepseek: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// Provider error bodies can echo credentials or title text; never return them to the browser.
		return &APIError{StatusCode: resp.StatusCode}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return fmt.Errorf("read deepseek response")
	}
	if err := json.Unmarshal(data, result); err != nil {
		return fmt.Errorf("decode deepseek response")
	}
	return nil
}
