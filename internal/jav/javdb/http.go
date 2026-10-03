package javdb

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/jav/internal/htmlutil"
	"javboss/internal/jav/metadata"
	"javboss/internal/util"

	"golang.org/x/net/html"
)

func (p *JavDBClient) fetchJavDBDetailByCode(ctx context.Context, code string) (*html.Node, string, error) {
	searchURL := javDBSearchURL(code)
	searchDoc, status, err := p.fetchJavDBHTML(ctx, searchURL, javDBBaseURL)
	if err != nil {
		return nil, "", err
	}
	if status == http.StatusNotFound || searchDoc == nil {
		return nil, "", metadata.ErrNotFound
	}

	detailURL := findJavDBSearchResultURL(searchDoc, code, searchURL)
	if detailURL == "" {
		return nil, "", metadata.ErrNotFound
	}

	detailDoc, status, err := p.fetchJavDBHTML(ctx, detailURL, searchURL)
	if err != nil {
		return nil, "", err
	}
	if status == http.StatusNotFound || detailDoc == nil {
		return nil, "", metadata.ErrNotFound
	}
	return detailDoc, detailURL, nil
}

func (p *JavDBClient) fetchJavDBHTML(ctx context.Context, targetURL, referer string) (*html.Node, int, error) {
	req, err := buildJavDBRequest(ctx, targetURL, referer)
	if err != nil {
		return nil, 0, err
	}

	logging.Info("javdb request: %s", targetURL)
	resp, err := p.doJavDBRequest(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	logging.Info("javdb response status: %s, length: %d bytes", resp.Status, len(body))
	if resp.StatusCode == http.StatusNotFound {
		return nil, resp.StatusCode, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("javdb: http %d", resp.StatusCode)
	}

	doc, err := htmlutil.ParseHTMLDocument(body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("javdb: parse html: %w", err)
	}
	return doc, resp.StatusCode, nil
}

func (p *JavDBClient) doJavDBRequest(req *http.Request) (*http.Response, error) {
	if err := p.limiter.Wait(req.Context()); err != nil {
		return nil, err
	}
	return p.httpClient.Do(req)
}

// NewHTTPClient creates a fresh proxy-aware HTTP client with this site's transport settings.
func NewHTTPClient() *http.Client {
	return util.NewHTTPClientWithTransport(15*time.Second, func(t *http.Transport) {
		t.ForceAttemptHTTP2 = true
		t.TLSClientConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			MaxVersion: tls.VersionTLS13,
			NextProtos: []string{"h2", "http/1.1"},
		}
		t.MaxIdleConns = 200
		t.MaxIdleConnsPerHost = 20
		t.MaxConnsPerHost = 50
	})
}

func buildJavDBRequest(ctx context.Context, targetURL, referer string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", javDBUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cookie", "over18=1")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	return req, nil
}

// OriginURL identifies the origin used for availability checks.
func (p *JavDBClient) OriginURL() string { return javDBBaseURL }
