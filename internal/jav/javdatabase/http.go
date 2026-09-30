package javdatabase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"javboss/internal/common/logging"
	"javboss/internal/jav/internal/htmlutil"
	"javboss/internal/util"

	"golang.org/x/net/html"
)

func (p *JavDatabaseClient) fetchJavDatabaseHTML(ctx context.Context, targetURL, referer string) (*html.Node, int, error) {
	req, err := buildJavDatabaseRequest(ctx, targetURL, referer)
	if err != nil {
		return nil, 0, err
	}

	logging.Info("javdatabase request: %s", targetURL)
	resp, err := p.doJavDatabaseRequest(req)
	if err != nil {
		if errors.Is(err, util.ErrCachedNotFound) {
			return nil, http.StatusNotFound, nil
		}
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	logging.Info("javdatabase response status: %s, length: %d bytes", resp.Status, len(body))
	if resp.StatusCode == http.StatusNotFound {
		return nil, resp.StatusCode, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("javdatabase: http %d", resp.StatusCode)
	}

	doc, err := htmlutil.ParseHTMLDocument(body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("javdatabase: parse html: %w", err)
	}
	return doc, resp.StatusCode, nil
}

func (p *JavDatabaseClient) doJavDatabaseRequest(req *http.Request) (*http.Response, error) {
	if err := p.limiter.Wait(req.Context()); err != nil {
		return nil, err
	}
	return p.httpClient.Do(req)
}

func buildJavDatabaseRequest(ctx context.Context, targetURL, referer string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	return req, nil
}

// OriginURL identifies the origin used for availability checks.
func (p *JavDatabaseClient) OriginURL() string { return "https://www.javdatabase.com" }
