package avmoo

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"javboss/internal/jav/scrape/internal/baseurl"
	"javboss/internal/jav/scrape/internal/ratelimit"
	"javboss/internal/jav/scrape/metadata"
)

// AvmooClient retrieves metadata from avmoo.
type AvmooClient struct {
	httpClient   *http.Client
	baseURLMu    sync.RWMutex
	baseURL      string
	sessionCache struct {
		sync.Mutex
		session   avmooSession
		expiresAt time.Time
	}
	limiter *ratelimit.Limiter
}

const (
	avmooUserAgent       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	avmooRequestInterval = 1500 * time.Millisecond
	avmooAPILanguage     = "tw"
	avmooAPISearchLimit  = 30
	avmooLookupTimeout   = 90 * time.Second
	avmooHTTPTimeout     = 30 * time.Second
	avmooAPITries        = 3
	avmooAPIRetryDelay   = 2 * time.Second
	avmooSessionTTL      = 30 * time.Minute
)

// DefaultBaseURL is the official Avmoo origin used when no override is configured.
const DefaultBaseURL = "https://avmoo.shop"

// BaseURL returns the origin currently used by this client.
func (p *AvmooClient) BaseURL() string {
	if p == nil {
		return DefaultBaseURL
	}
	p.baseURLMu.RLock()
	value := p.baseURL
	p.baseURLMu.RUnlock()
	if strings.TrimSpace(value) == "" {
		return DefaultBaseURL
	}
	return value
}

// SetBaseURL replaces the origin used for requests. Invalid values fall back to
// the default origin so a bad persisted setting cannot break scraping.
func (p *AvmooClient) SetBaseURL(raw string) {
	if p == nil {
		return
	}
	normalized := baseurl.NormalizeOrDefault(raw)
	p.baseURLMu.Lock()
	p.baseURL = normalized
	p.baseURLMu.Unlock()
}

// LookupJavByCode fetches metadata for a given code.
func (p *AvmooClient) LookupJavByCode(ctx context.Context, code string) (*metadata.JavInfo, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, metadata.ErrNotFound
	}

	ctx, cancel := context.WithTimeout(ctx, avmooLookupTimeout)
	defer cancel()

	movie, err := p.fetchAvmooMovieByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	info := avmooMovieInfoFromAPI(movie, p.BaseURL())
	if info == nil {
		return nil, metadata.ErrNotFound
	}
	if info.Code == "" {
		info.Code = code
	}
	return info, nil
}

// New creates a provider using the supplied non-nil HTTP client.
func New(httpClient *http.Client) *AvmooClient {
	return &AvmooClient{httpClient: httpClient, limiter: ratelimit.New(avmooRequestInterval)}
}
