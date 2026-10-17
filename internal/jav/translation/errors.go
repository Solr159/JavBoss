package translation

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

// APIError contains only bounded, redacted provider diagnostics, never the raw body.
type APIError struct {
	StatusCode int
	Code       string
	Type       string
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("translation provider returned HTTP %d (%s): %s", e.StatusCode, e.Code, e.Message)
}

var providerErrorIdentifier = regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,100}$`)
var providerKeyPattern = regexp.MustCompile(`\bsk-[a-zA-Z0-9_.-]+`)

func readAPIError(resp *http.Response, req *http.Request, secrets []string) *APIError {
	defer resp.Body.Close()
	result := &APIError{StatusCode: resp.StatusCode}
	const maxBody = 16 << 10
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil || len(data) > maxBody {
		return result
	}
	var envelope struct {
		Error   json.RawMessage
		Message string
		Code    json.RawMessage
	}
	if json.Unmarshal(data, &envelope) != nil {
		return result
	}
	var detail struct {
		Message string
		Code    json.RawMessage
		Type    string
		Status  string
	}
	detail.Message, detail.Code = envelope.Message, envelope.Code
	if len(envelope.Error) > 0 && string(envelope.Error) != "null" {
		if json.Unmarshal(envelope.Error, &detail) != nil {
			if json.Unmarshal(envelope.Error, &detail.Message) != nil {
				return result
			}
		}
	}
	// Custom services may echo credentials supplied outside the standard key field.
	secrets = append([]string(nil), secrets...)
	for name, values := range req.Header {
		if strings.EqualFold(name, "Content-Type") || strings.EqualFold(name, "Accept") || strings.EqualFold(name, "User-Agent") {
			continue
		}
		for _, value := range values {
			secrets = append(secrets, value)
			if fields := strings.Fields(value); len(fields) == 2 && (strings.EqualFold(fields[0], "Bearer") || strings.EqualFold(fields[0], "Basic")) {
				secrets = append(secrets, fields[1])
			}
		}
	}
	for _, values := range req.URL.Query() {
		secrets = append(secrets, values...)
	}
	for _, secret := range append([]string(nil), secrets...) {
		secrets = append(secrets, url.QueryEscape(secret), url.PathEscape(secret))
	}
	sort.Slice(secrets, func(i, j int) bool { return len(secrets[i]) > len(secrets[j]) })
	redact := func(value string, limit int) string {
		for _, secret := range secrets {
			if secret != "" {
				value = strings.ReplaceAll(value, secret, "[redacted]")
			}
		}
		value = providerKeyPattern.ReplaceAllString(value, "[redacted]")
		value = strings.Join(strings.Fields(value), " ")
		runes := []rune(value)
		if len(runes) > limit {
			value = string(runes[:limit]) + "…"
		}
		return value
	}
	var code string
	if json.Unmarshal(detail.Code, &code) != nil {
		var number json.Number
		if json.Unmarshal(detail.Code, &number) == nil {
			code = number.String()
		}
	}
	if code = redact(code, 100); providerErrorIdentifier.MatchString(code) {
		result.Code = code
	}
	if detail.Type == "" {
		detail.Type = detail.Status
	}
	if kind := redact(detail.Type, 100); providerErrorIdentifier.MatchString(kind) {
		result.Type = kind
	}
	result.Message = redact(detail.Message, 1000)
	return result
}
