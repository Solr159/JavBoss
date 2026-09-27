package javbus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"javboss/internal/common/logging"
	"javboss/internal/jav/internal/htmlutil"
	"javboss/internal/jav/metadata"
	"javboss/internal/util"

	"golang.org/x/net/html"
)

func (p *Client) fetchJavBusDocument(ctx context.Context, code string) (*html.Node, string, error) {
	base := "https://www.javbus.com"

	url := fmt.Sprintf("%s/%s", base, code)

	req, err := buildRequest(ctx, url)
	if err != nil {
		return nil, "", err
	}

	logging.Info("javbus request: %s", url)

	resp, err := p.doJavBusRequest(req)
	// TODO: Should not return here, try curl fallback.
	if err != nil {
		if errors.Is(err, util.ErrCachedNotFound) {
			logging.Info("javbus: cached 404 for %s", url)
			return nil, "", metadata.ErrNotFound
		}
		return nil, "", err
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, "", err
	}

	logging.Info("javbus response status: %s, length: %d bytes", resp.Status, len(body))

	if resp.StatusCode == http.StatusNotFound {
		logging.Info("javbus: %s 404 not found", url)
		return nil, "", metadata.ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		logging.Info("javbus: non-200 status on %s: %s", url, resp.Status)
		return nil, "", errors.New("javbus: non-200 response")
	}

	doc, err := htmlutil.ParseHTMLDocument(body)
	if err != nil {
		logging.Error("parse javbus html: %s", err.Error())
		return nil, "", metadata.ErrNotFound
	}
	return doc, url, nil
}

func (p *Client) fetchJavBusGenreDocument(ctx context.Context, targetURL string) (*html.Node, error) {
	req, err := buildRequest(ctx, targetURL)
	if err != nil {
		return nil, err
	}
	logging.Info("javbus genre request: %s", targetURL)
	resp, err := p.doJavBusRequest(req)
	if err != nil {
		return nil, fmt.Errorf("fetch javbus genre page: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("javbus genre page returned %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read javbus genre page: %w", err)
	}
	doc, err := htmlutil.ParseHTMLDocument(body)
	if err != nil {
		return nil, fmt.Errorf("parse javbus genre page: %w", err)
	}
	if resp.Request != nil && resp.Request.URL != nil && strings.Contains(resp.Request.URL.Path, "driver-verify") {
		return nil, errors.New("javbus requires browser verification before its genre pages can be read")
	}
	return doc, nil
}

func (p *Client) doJavBusRequest(req *http.Request) (*http.Response, error) {
	if err := p.limiter.Wait(req.Context()); err != nil {
		return nil, err
	}
	return util.DoRequest(req)
}

func buildRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Referer", "https://www.javbus.com/")
	req.Header.Set("Cookie", "age=verified; existmag=mag")
	return req, nil
}
