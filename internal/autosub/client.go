// Package autosub provides an HTTP-based client for the AutoSub subtitle
// gateway so the host app can download and translate subtitles without
// depending on a GUI session.
package autosub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client talks to the AutoSub gateway over loopback HTTP.
type Client struct {
	HTTP    *http.Client
	BaseURL string
}

// NewClient returns a Client pointed at 127.0.0.1:8765 by default.
func NewClient() *Client {
	return &Client{
		HTTP:    &http.Client{Timeout: 120 * time.Second},
		BaseURL: "http://127.0.0.1:8765",
	}
}

// DownloadRequest asks the gateway to fetch subtitles for one video.
type DownloadRequest struct {
	Video    Video  `json:"video"`
	Language string `json:"language"`
}

// DownloadResult mirrors the gateway's DownloadResult payload.
type DownloadResult struct {
	Success       bool   `json:"success"`
	SubtitlePath  string `json:"subtitle_path"`
	Source        string `json:"source"`
	Error         string `json:"error"`
	Language      string `json:"language"`
	Failures      []string `json:"failures"`
	FailureDetail string   `json:"failure_detail"`
}

// TranslateRequest asks the gateway to translate a subtitle file. Params maps
// directly onto the gateway's target_language / source_language / engine keys.
type TranslateRequest struct {
	Video  Video             `json:"video"`
	Params map[string]string `json:"params,omitempty"`
}

// TranslateResult mirrors the gateway's translate response payload.
type TranslateResult struct {
	Success      bool   `json:"success"`
	SubtitlePath string `json:"subtitle_path"`
	Language     string `json:"language"`
	Error        string `json:"error"`
}

// Ping checks that the gateway is reachable.
func (c *Client) Ping(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/ping", nil)
	if err != nil {
		return fmt.Errorf("build ping request: %w", err)
	}
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return fmt.Errorf("gateway is unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gateway ping returned %d", resp.StatusCode)
	}
	return nil
}

// Translate asks the gateway to translate a subtitle file.
func (c *Client) Translate(ctx context.Context, req TranslateRequest) (TranslateResult, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return TranslateResult{}, fmt.Errorf("encode translate request: %w", err)
	}

	url := c.BaseURL + "/translate"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return TranslateResult{}, fmt.Errorf("build translate request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return TranslateResult{}, fmt.Errorf("translate request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return TranslateResult{}, fmt.Errorf("read translate response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return TranslateResult{}, fmt.Errorf("gateway returned %d: %s", resp.StatusCode, trim(data))
	}

	var out TranslateResult
	if err := json.Unmarshal(data, &out); err != nil {
		return TranslateResult{}, fmt.Errorf("parse translate response: %w", err)
	}
	return out, nil
}

// Download asks the gateway to download subtitles for the given video.
func (c *Client) Download(ctx context.Context, req DownloadRequest) (DownloadResult, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("encode download request: %w", err)
	}

	url := c.BaseURL + "/download"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return DownloadResult{}, fmt.Errorf("build download request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return DownloadResult{}, fmt.Errorf("read download response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return DownloadResult{}, fmt.Errorf("gateway returned %d: %s", resp.StatusCode, trim(data))
	}

	var out DownloadResult
	if err := json.Unmarshal(data, &out); err != nil {
		return DownloadResult{}, fmt.Errorf("parse download response: %w", err)
	}
	return out, nil
}

func trim(data []byte) string {
	if len(data) > 512 {
		return string(data[:512]) + "..."
	}
	return string(data)
}
