package avsox

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"javboss/internal/jav/internal/ratelimit"
	"javboss/internal/jav/metadata"
)

// AvsoxClient retrieves metadata from avsox.
type AvsoxClient struct {
	httpClient   *http.Client
	sessionCache struct {
		sync.Mutex
		session   avsoxSession
		expiresAt time.Time
	}
	limiter *ratelimit.Limiter
}

const (
	avsoxBaseURL         = "https://avsox.click"
	avsoxUserAgent       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	avsoxRequestInterval = 1500 * time.Millisecond
	avsoxAPILanguage     = "cn"
	avsoxAPISearchLimit  = 60
	avsoxLookupTimeout   = 90 * time.Second
	avsoxHTTPTimeout     = 30 * time.Second
	avsoxAPITries        = 3
	avsoxAPIRetryDelay   = 2 * time.Second
	avsoxSessionTTL      = 30 * time.Minute
)

// LookupJavByCode fetches metadata for a given code.
func (p *AvsoxClient) LookupJavByCode(ctx context.Context, code string) (*metadata.JavInfo, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, metadata.ErrNotFound
	}

	ctx, cancel := context.WithTimeout(ctx, avsoxLookupTimeout)
	defer cancel()

	movie, err := p.fetchAvsoxMovieByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	info := avsoxMovieInfoFromAPI(movie)
	if info == nil {
		return nil, metadata.ErrNotFound
	}
	if info.Code == "" {
		info.Code = code
	}
	return info, nil
}

// LookupMovieURLByCode resolves a movie code to its Avsox detail page.
func (p *AvsoxClient) LookupMovieURLByCode(ctx context.Context, code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", metadata.ErrNotFound
	}

	ctx, cancel := context.WithTimeout(ctx, avsoxLookupTimeout)
	defer cancel()

	movie, err := p.fetchAvsoxMovieByCode(ctx, code)
	if err != nil {
		return "", err
	}
	detailURL := avsoxMovieDetailURL(movie)
	if detailURL == "" {
		return "", metadata.ErrNotFound
	}
	return detailURL, nil
}

// New creates a provider using the supplied non-nil HTTP client.
func New(httpClient *http.Client) *AvsoxClient {
	return &AvsoxClient{httpClient: httpClient, limiter: ratelimit.New(avsoxRequestInterval)}
}
