package avsox

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
	"javboss/internal/jav/metadata"
	"javboss/internal/util"
)

func (e avsoxStatusError) Error() string {
	if strings.TrimSpace(e.message) != "" {
		return fmt.Sprintf("avsox: %s code %d: %s", e.source, e.status, e.message)
	}
	return fmt.Sprintf("avsox: %s code %d", e.source, e.status)
}

func (p *Client) fetchAvsoxMovieByCode(ctx context.Context, code string) (*avsoxAPIMovie, error) {
	session, err := p.cachedAvsoxSession(ctx, code)
	if err != nil {
		return nil, err
	}
	movie, err := p.fetchAvsoxMovieWithSession(ctx, session, code)
	if !isAvsoxSessionAuthError(err) {
		return movie, err
	}

	p.invalidateCachedAvsoxSession(session)
	logging.Info("avsox session expired, refreshing")
	session, err = p.refreshAvsoxSession(ctx, code)
	if err != nil {
		return nil, err
	}
	return p.fetchAvsoxMovieWithSession(ctx, session, code)
}

func (p *Client) fetchAvsoxMovieWithSession(ctx context.Context, session avsoxSession, code string) (*avsoxAPIMovie, error) {
	searchPayload := []any{
		map[string]string{
			"search": code,
			"lang":   avsoxAPILanguage,
		},
		avsoxAPISearchLimit,
		1,
	}
	var searchResults []avsoxAPIMovie
	if err := p.postAvsoxAPI(ctx, session, "/javu/data/api/search", searchPayload, &searchResults); err != nil {
		return nil, err
	}

	result := findAvsoxAPISearchResult(searchResults, code)
	if result == nil {
		return nil, metadata.ErrNotFound
	}
	if strings.TrimSpace(result.MovieID) == "" {
		return nil, metadata.ErrNotFound
	}
	detailPayload := []any{result.MovieID, avsoxAPILanguage}
	var movie avsoxAPIMovie
	if err := p.postAvsoxAPI(ctx, session, "/javu/data/api/getMovie", detailPayload, &movie); err != nil {
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

func (p *Client) cachedAvsoxSession(ctx context.Context, code string) (avsoxSession, error) {
	now := time.Now()
	p.sessionCache.Lock()
	session := p.sessionCache.session
	if session.csrfToken != "" && session.cookie != "" && now.Before(p.sessionCache.expiresAt) {
		p.sessionCache.Unlock()
		return session, nil
	}
	p.sessionCache.Unlock()
	return p.refreshAvsoxSession(ctx, code)
}

func (p *Client) refreshAvsoxSession(ctx context.Context, code string) (avsoxSession, error) {
	session, err := p.fetchAvsoxSession(ctx, code)
	if err != nil {
		return avsoxSession{}, err
	}
	p.sessionCache.Lock()
	p.sessionCache.session = session
	p.sessionCache.expiresAt = time.Now().Add(avsoxSessionTTL)
	p.sessionCache.Unlock()
	return session, nil
}

func (p *Client) invalidateCachedAvsoxSession(session avsoxSession) {
	p.sessionCache.Lock()
	if p.sessionCache.session.csrfToken == session.csrfToken && p.sessionCache.session.cookie == session.cookie {
		p.sessionCache.session = avsoxSession{}
		p.sessionCache.expiresAt = time.Time{}
	}
	p.sessionCache.Unlock()
}

func isAvsoxSessionAuthError(err error) bool {
	var statusErr avsoxStatusError
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

func (p *Client) fetchAvsoxSession(ctx context.Context, code string) (avsoxSession, error) {
	pageURL := fmt.Sprintf("%s/%s/search/%s", avsoxBaseURL, avsoxAPILanguage, url.PathEscape(code))
	req, err := buildAvsoxRequest(ctx, pageURL, avsoxBaseURL)
	if err != nil {
		return avsoxSession{}, err
	}
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Dest", "document")

	logging.Info("avsox request: %s", pageURL)
	resp, err := p.doAvsoxRequest(req)
	if err != nil {
		return avsoxSession{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return avsoxSession{}, err
	}
	logging.Info("avsox response status: %s, length: %d bytes", resp.Status, len(body))

	if resp.StatusCode == http.StatusNotFound {
		return avsoxSession{}, metadata.ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return avsoxSession{}, fmt.Errorf("avsox: http %d", resp.StatusCode)
	}

	token := avshared.ExtractCSRFToken(string(body))
	cookie := avshared.CookieHeader(resp.Cookies())
	if token == "" || cookie == "" {
		return avsoxSession{}, errors.New("avsox: missing csrf session")
	}
	return avsoxSession{
		csrfToken: token,
		cookie:    cookie,
		referer:   pageURL,
	}, nil
}

func (p *Client) postAvsoxAPI(ctx context.Context, session avsoxSession, path string, payload any, out any) error {
	var lastErr error
	for attempt := 1; attempt <= avsoxAPITries; attempt++ {
		err := p.postAvsoxAPIOnce(ctx, session, path, payload, out)
		if err == nil || errors.Is(err, metadata.ErrNotFound) {
			return err
		}
		lastErr = err
		if attempt == avsoxAPITries || !shouldRetryAvsoxAPIError(err) {
			break
		}
		logging.Info("avsox api retry after error: %v", err)
		timer := time.NewTimer(avsoxAPIRetryDelay)
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

func shouldRetryAvsoxAPIError(err error) bool {
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
	var statusErr avsoxStatusError
	if errors.As(err, &statusErr) {
		return statusErr.status == http.StatusTooManyRequests || statusErr.status >= http.StatusInternalServerError
	}
	return false
}

func (p *Client) postAvsoxAPIOnce(ctx context.Context, session avsoxSession, path string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	targetURL := avsoxBaseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", avsoxUserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", avsoxBaseURL)
	req.Header.Set("Referer", session.referer)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("X-CSRF-Token", session.csrfToken)
	req.Header.Set("Cookie", session.cookie)

	logging.Info("avsox request: %s", targetURL)
	resp, err := p.doAvsoxRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	logging.Info("avsox response status: %s, length: %d bytes", resp.Status, len(raw))

	if resp.StatusCode == http.StatusNotFound {
		return metadata.ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return avsoxStatusError{source: "http", status: resp.StatusCode}
	}

	var envelope avsoxAPIEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("avsox: parse api response: %w", err)
	}
	if envelope.Code == http.StatusNotFound {
		return metadata.ErrNotFound
	}
	if envelope.Code != http.StatusOK {
		return avsoxStatusError{source: "api", status: envelope.Code, message: envelope.Message}
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return metadata.ErrNotFound
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("avsox: parse api data: %w", err)
	}
	return nil
}

func (p *Client) doAvsoxRequest(req *http.Request) (*http.Response, error) {
	if err := p.limiter.Wait(req.Context()); err != nil {
		return nil, err
	}
	return p.defaultAvsoxHTTPClient().Do(req)
}

func (p *Client) defaultAvsoxHTTPClient() *http.Client {
	p.httpOnce.Do(func() {
		p.httpClient = util.NewHTTPClientWithTransport(avsoxHTTPTimeout, func(t *http.Transport) {
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

func buildAvsoxRequest(ctx context.Context, targetURL, referer string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", avsoxUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	return req, nil
}

type avsoxSession struct {
	csrfToken string
	cookie    string
	referer   string
}

type avsoxAPIEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type avsoxStatusError struct {
	source  string
	status  int
	message string
}

// CheckConnectivity requests the site using its normal headers and transport, without lookup caching.
// The caller owns the response body.
func (p *Client) CheckConnectivity(ctx context.Context) (*http.Response, error) {
	req, err := buildAvsoxRequest(ctx, avsoxBaseURL+"/", avsoxBaseURL)
	if err != nil {
		return nil, err
	}
	return p.doAvsoxRequest(req)
}
