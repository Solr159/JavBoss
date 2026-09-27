package avmoo

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/jav/internal/avshared"
	"javboss/internal/jav/internal/htmlutil"
	"javboss/internal/jav/metadata"
	"javboss/internal/util"

	"golang.org/x/net/html"
)

func (e avmooStatusError) Error() string {
	if strings.TrimSpace(e.message) != "" {
		return fmt.Sprintf("avmoo: %s code %d: %s", e.source, e.status, e.message)
	}
	return fmt.Sprintf("avmoo: %s code %d", e.source, e.status)
}

func (p *Client) fetchAvmooMovieByCode(ctx context.Context, code string) (*avmooAPIMovie, error) {
	session, err := p.cachedAvmooSession(ctx, code)
	if err != nil {
		return nil, err
	}
	movie, err := p.fetchAvmooMovieWithSession(ctx, session, code)
	if !isAvmooSessionAuthError(err) {
		return movie, err
	}

	p.invalidateCachedAvmooSession(session)
	logging.Info("avmoo session expired, refreshing")
	session, err = p.refreshAvmooSession(ctx, code)
	if err != nil {
		return nil, err
	}
	return p.fetchAvmooMovieWithSession(ctx, session, code)
}

func (p *Client) fetchAvmooMovieWithSession(ctx context.Context, session avmooSession, code string) (*avmooAPIMovie, error) {
	searchPayload := []any{
		map[string]string{
			"search": code,
			"lang":   avmooAPILanguage,
		},
		avmooAPISearchLimit,
		1,
	}
	var searchResults []avmooAPIMovie
	if err := p.postAvmooAPI(ctx, session, "/jav/data/api/search", searchPayload, &searchResults); err != nil {
		return nil, err
	}

	result := findAvmooAPISearchResult(searchResults, code)
	if result == nil {
		return nil, metadata.ErrNotFound
	}
	if strings.TrimSpace(result.MovieID) == "" {
		return nil, metadata.ErrNotFound
	}
	detailPayload := []any{result.MovieID, avmooAPILanguage}
	var movie avmooAPIMovie
	if err := p.postAvmooAPI(ctx, session, "/jav/data/api/getMovie", detailPayload, &movie); err != nil {
		return nil, err
	}
	if strings.TrimSpace(movie.MovieFanHao) == "" {
		movie.MovieFanHao = result.MovieFanHao
	}
	if strings.TrimSpace(movie.MovieID) == "" {
		movie.MovieID = result.MovieID
	}
	return &movie, nil
}

func (p *Client) cachedAvmooSession(ctx context.Context, code string) (avmooSession, error) {
	now := time.Now()
	p.sessionCache.Lock()
	session := p.sessionCache.session
	if session.csrfToken != "" && session.cookie != "" && now.Before(p.sessionCache.expiresAt) {
		p.sessionCache.Unlock()
		return session, nil
	}
	p.sessionCache.Unlock()
	return p.refreshAvmooSession(ctx, code)
}

func (p *Client) refreshAvmooSession(ctx context.Context, code string) (avmooSession, error) {
	session, err := p.fetchAvmooSession(ctx, code)
	if err != nil {
		return avmooSession{}, err
	}
	p.sessionCache.Lock()
	p.sessionCache.session = session
	p.sessionCache.expiresAt = time.Now().Add(avmooSessionTTL)
	p.sessionCache.Unlock()
	return session, nil
}

func (p *Client) invalidateCachedAvmooSession(session avmooSession) {
	p.sessionCache.Lock()
	if p.sessionCache.session.csrfToken == session.csrfToken && p.sessionCache.session.cookie == session.cookie {
		p.sessionCache.session = avmooSession{}
		p.sessionCache.expiresAt = time.Time{}
	}
	p.sessionCache.Unlock()
}

func isAvmooSessionAuthError(err error) bool {
	var statusErr avmooStatusError
	if !errors.As(err, &statusErr) {
		return false
	}
	switch statusErr.status {
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, 419:
		return true
	default:
		return false
	}
}

func (p *Client) fetchAvmooSession(ctx context.Context, code string) (avmooSession, error) {
	pageURL := fmt.Sprintf("%s/%s/search/%s", avmooBaseURL, avmooAPILanguage, url.PathEscape(code))
	req, err := buildAvmooRequest(ctx, pageURL, avmooBaseURL)
	if err != nil {
		return avmooSession{}, err
	}
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Dest", "document")

	logging.Info("avmoo request: %s", pageURL)
	resp, err := p.doAvmooRequest(req)
	if err != nil {
		return avmooSession{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return avmooSession{}, err
	}
	logging.Info("avmoo response status: %s, length: %d bytes", resp.Status, len(body))

	if resp.StatusCode == http.StatusNotFound {
		return avmooSession{}, metadata.ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return avmooSession{}, fmt.Errorf("avmoo: http %d", resp.StatusCode)
	}

	token := avshared.ExtractCSRFToken(string(body))
	cookie := avshared.CookieHeader(resp.Cookies())
	if token == "" || cookie == "" {
		return avmooSession{}, errors.New("avmoo: missing csrf session")
	}
	return avmooSession{
		csrfToken: token,
		cookie:    cookie,
		referer:   pageURL,
	}, nil
}

func (p *Client) postAvmooAPI(ctx context.Context, session avmooSession, path string, payload any, out any) error {
	var lastErr error
	for attempt := 1; attempt <= avmooAPITries; attempt++ {
		err := p.postAvmooAPIOnce(ctx, session, path, payload, out)
		if err == nil || errors.Is(err, metadata.ErrNotFound) {
			return err
		}
		lastErr = err
		if attempt == avmooAPITries || !shouldRetryAvmooAPIError(err) {
			break
		}
		logging.Info("avmoo api retry after error: %v", err)
		timer := time.NewTimer(avmooAPIRetryDelay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
	return lastErr
}

func shouldRetryAvmooAPIError(err error) bool {
	if err == nil || errors.Is(err, metadata.ErrNotFound) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	var statusErr avmooStatusError
	if errors.As(err, &statusErr) {
		return statusErr.status == http.StatusTooManyRequests || statusErr.status >= http.StatusInternalServerError
	}
	return false
}

func (p *Client) postAvmooAPIOnce(ctx context.Context, session avmooSession, path string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	targetURL := avmooBaseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", avmooUserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-TW,zh;q=0.9,en;q=0.8")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", avmooBaseURL)
	req.Header.Set("Referer", session.referer)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("X-CSRF-Token", session.csrfToken)
	req.Header.Set("Cookie", session.cookie)

	logging.Info("avmoo request: %s", targetURL)
	resp, err := p.doAvmooRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	logging.Info("avmoo response status: %s, length: %d bytes", resp.Status, len(raw))

	if resp.StatusCode == http.StatusNotFound {
		return metadata.ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return avmooStatusError{source: "http", status: resp.StatusCode}
	}

	var envelope avmooAPIEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("avmoo: parse api response: %w", err)
	}
	if envelope.Code == http.StatusNotFound {
		return metadata.ErrNotFound
	}
	if envelope.Code != http.StatusOK {
		return avmooStatusError{source: "api", status: envelope.Code, message: envelope.Message}
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return metadata.ErrNotFound
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("avmoo: parse api data: %w", err)
	}
	return nil
}

func (p *Client) fetchAvmooDetailByCode(ctx context.Context, code string) (*html.Node, string, error) {
	searchURL := fmt.Sprintf("%s/tw/search/%s", avmooBaseURL, url.PathEscape(code))
	searchDoc, status, err := p.fetchAvmooHTML(ctx, searchURL, avmooBaseURL)
	if err != nil {
		return nil, "", err
	}
	if status == http.StatusNotFound || searchDoc == nil {
		return nil, "", metadata.ErrNotFound
	}

	detailURL := findAvmooSearchResultURL(searchDoc, code, searchURL)
	if detailURL == "" {
		return nil, "", metadata.ErrNotFound
	}

	detailDoc, status, err := p.fetchAvmooHTML(ctx, detailURL, searchURL)
	if err != nil {
		return nil, "", err
	}
	if status == http.StatusNotFound || detailDoc == nil {
		return nil, "", metadata.ErrNotFound
	}
	return detailDoc, detailURL, nil
}

func (p *Client) fetchAvmooHTML(ctx context.Context, targetURL, referer string) (*html.Node, int, error) {
	req, err := buildAvmooRequest(ctx, targetURL, referer)
	if err != nil {
		return nil, 0, err
	}

	logging.Info("avmoo request: %s", targetURL)
	resp, err := p.doAvmooRequest(req)
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

	logging.Info("avmoo response status: %s, length: %d bytes", resp.Status, len(body))
	if resp.StatusCode == http.StatusNotFound {
		return nil, resp.StatusCode, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("avmoo: http %d", resp.StatusCode)
	}

	doc, err := htmlutil.ParseHTMLDocument(body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("avmoo: parse html: %w", err)
	}
	return doc, resp.StatusCode, nil
}

func (p *Client) doAvmooRequest(req *http.Request) (*http.Response, error) {
	if err := p.limiter.Wait(req.Context()); err != nil {
		return nil, err
	}
	return p.defaultAvmooHTTPClient().Do(req)
}

func (p *Client) defaultAvmooHTTPClient() *http.Client {
	p.httpOnce.Do(func() {
		p.httpClient = util.NewHTTPClientWithTransport(avmooHTTPTimeout, func(t *http.Transport) {
			t.ForceAttemptHTTP2 = false
			t.DisableCompression = true
			t.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS13}
			t.MaxIdleConns = 50
			t.MaxIdleConnsPerHost = 5
			t.MaxConnsPerHost = 5
		})
	})
	return p.httpClient
}

func buildAvmooRequest(ctx context.Context, targetURL, referer string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", avmooUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-TW,zh;q=0.9,en;q=0.8")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	return req, nil
}

type avmooSession struct {
	csrfToken string
	cookie    string
	referer   string
}

type avmooAPIEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type avmooStatusError struct {
	source  string
	status  int
	message string
}

// CheckConnectivity requests the site using its normal headers and transport, without lookup caching.
// The caller owns the response body.
func (p *Client) CheckConnectivity(ctx context.Context) (*http.Response, error) {
	req, err := buildAvmooRequest(ctx, p.ConnectivityURL()+"/", p.ConnectivityURL())
	if err != nil {
		return nil, err
	}
	return p.doAvmooRequest(req)
}

// ConnectivityURL identifies the origin used for connectivity checks.
func (p *Client) ConnectivityURL() string { return avmooBaseURL }
