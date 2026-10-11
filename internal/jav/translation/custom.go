package translation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var templatePlaceholder = regexp.MustCompile(`(?s)\{\{.*?\}\}`)

// Templates are JSON data, never executable code. Exact placeholders retain their JSON type.
func expandTemplate(value any, vars map[string]any) any {
	switch v := value.(type) {
	case string:
		if strings.HasPrefix(v, "{{") && strings.HasSuffix(v, "}}") {
			if replacement, ok := vars[v[2:len(v)-2]]; ok {
				return replacement
			}
		}
		// Replacement text is not scanned again, including placeholder-like text in titles.
		return templatePlaceholder.ReplaceAllStringFunc(v, func(token string) string {
			if replacement, ok := vars[token[2:len(token)-2]]; ok {
				return fmt.Sprint(replacement)
			}
			return token
		})
	case map[string]any:
		result := make(map[string]any, len(v))
		for key, item := range v {
			result[key] = expandTemplate(item, vars)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = expandTemplate(item, vars)
		}
		return result
	default:
		return value
	}
}

func (c *Client) translateCustom(ctx context.Context, p Profile, prompt, title string, thinking bool) (string, error) {
	if len(p.Request) == 0 {
		return "", &ConfigError{"请填写自定义请求模板 / Enter a custom request template"}
	}
	if p.Reasoning == "toggle" && (len(p.FastParams) == 0 || len(p.ThinkingParams) == 0) {
		return "", &ConfigError{"请分别配置关闭和开启推理的请求覆盖参数 / Configure both quick and reasoning request overrides"}
	}
	template := map[string]any{}
	mergeParams(template, p.Request)
	if thinking {
		mergeParams(template, p.ThinkingParams)
	} else {
		mergeParams(template, p.FastParams)
	}
	var temperature any
	if p.Temperature != nil {
		temperature = *p.Temperature
	}
	vars := map[string]any{"api_key": p.APIKey, "base_url": p.Endpoint("custom"), "model": p.Model, "prompt": prompt, "title": title, "temperature": temperature, "thinking": thinking, "effort": p.Effort}
	expanded := expandTemplate(template, vars)
	data, err := json.Marshal(expanded)
	if err != nil {
		return "", &ConfigError{"自定义请求模板无效 / Invalid custom request template"}
	}
	var request struct {
		URL          string
		Method       string
		Headers      map[string]string
		Body         json.RawMessage
		ResponsePath string `json:"response_path"`
	}
	if json.Unmarshal(data, &request) != nil {
		return "", &ConfigError{"自定义请求模板字段类型无效 / Invalid custom request template fields"}
	}
	u, err := url.Parse(request.URL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
		return "", &ConfigError{"自定义请求地址无效 / Invalid custom request URL"}
	}
	method := strings.ToUpper(request.Method)
	if method == "" {
		method = http.MethodPost
	}
	if method != http.MethodPost && method != http.MethodGet {
		return "", &ConfigError{"自定义请求仅支持 GET 或 POST / Custom requests support GET or POST"}
	}
	req, err := http.NewRequestWithContext(ctx, method, request.URL, bytes.NewReader(request.Body))
	if err != nil {
		return "", &ConfigError{"自定义请求无效 / Invalid custom request"}
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range request.Headers {
		req.Header.Set(key, value)
	}
	resp, err := c.do(req)
	if err != nil {
		return "", err
	}
	var result any
	if err = readJSON(resp, &result); err != nil {
		return "", err
	}
	result, err = jsonPointer(result, request.ResponsePath)
	if err != nil {
		return "", err
	}
	translated, ok := result.(string)
	translated = strings.TrimSpace(translated)
	if !ok || translated == "" || len(translated) > 8192 {
		return "", fmt.Errorf("custom response path did not return a complete title")
	}
	return translated, nil
}

// RFC 6901 paths handle arbitrary object keys and array indexes without evaluation.
func jsonPointer(value any, path string) (any, error) {
	if path == "" {
		return value, nil
	}
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("response path must be a JSON Pointer")
	}
	for _, part := range strings.Split(path[1:], "/") {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		switch node := value.(type) {
		case map[string]any:
			var ok bool
			value, ok = node[part]
			if !ok {
				return nil, fmt.Errorf("response path not found")
			}
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(node) {
				return nil, fmt.Errorf("invalid response array index")
			}
			value = node[index]
		default:
			return nil, fmt.Errorf("response path not found")
		}
	}
	return value, nil
}

// JSON objects merge recursively. Null deletes a default parameter.
func mergeParams(dst, src map[string]any) {
	for key, value := range src {
		if value == nil {
			delete(dst, key)
			continue
		}
		object, ok := value.(map[string]any)
		current, currentOK := dst[key].(map[string]any)
		if ok {
			if !currentOK {
				current = map[string]any{}
				dst[key] = current
			}
			mergeParams(current, object)
		} else {
			dst[key] = value
		}
	}
}
