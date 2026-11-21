package javdatabase

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/jav/scrape/internal/baseurl"
	"javboss/internal/jav/scrape/internal/parseutil"
	"javboss/internal/jav/scrape/internal/ratelimit"
	"javboss/internal/jav/scrape/metadata"
)

// JavDatabaseClient retrieves metadata from javdatabase.
type JavDatabaseClient struct {
	httpClient *http.Client
	limiter    *ratelimit.Limiter
	baseURLMu  sync.RWMutex
	baseURL    string
}

var errNoActressLink = errors.New("javdatabase: actress link not found")

const javDatabaseRequestInterval = 500 * time.Millisecond

// DefaultBaseURL is the official JavDatabase origin used when no override is configured.
const DefaultBaseURL = "https://www.javdatabase.com"

// BaseURL returns the origin currently used by this client.
func (p *JavDatabaseClient) BaseURL() string {
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
func (p *JavDatabaseClient) SetBaseURL(raw string) {
	if p == nil {
		return
	}
	normalized := baseurl.NormalizeOrDefault(raw)
	p.baseURLMu.Lock()
	p.baseURL = normalized
	p.baseURLMu.Unlock()
}

// LookupActressByCode resolves a solo movie code to its actress profile.
func (p *JavDatabaseClient) LookupActressByCode(ctx context.Context, code string) (*metadata.ActressInfo, error) {
	return p.lookupActressByCode(ctx, code)
}

// LookupJavByCode fetches metadata for a given code.
func (p *JavDatabaseClient) LookupJavByCode(ctx context.Context, code string) (*metadata.JavInfo, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, metadata.ErrNotFound
	}

	base := p.BaseURL()
	movieURL := fmt.Sprintf("%s/movies/%s/", base, code)

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	doc, status, err := p.fetchJavDatabaseHTML(ctx, movieURL, base)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound || doc == nil {
		return nil, metadata.ErrNotFound
	}

	info := parseJavDatabaseMovieInfo(doc)
	if info == nil {
		return nil, metadata.ErrNotFound
	}
	if info.Code != "" && normalizeJavDatabaseCode(info.Code) != normalizeJavDatabaseCode(code) {
		logging.Info("javdatabase: requested code %s resolved to %s", code, info.Code)
		return nil, metadata.ErrNotFound
	}
	if info.Code == "" {
		info.Code = code
	}
	info.CoverURL = parseJavDatabaseCoverURL(doc, movieURL)
	info.SampleImages = parseutil.ParseSampleImages(doc, movieURL)
	return info, nil
}

func (p *JavDatabaseClient) lookupActressByCode(ctx context.Context, code string) (*metadata.ActressInfo, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, metadata.ErrNotFound
	}

	base := p.BaseURL()

	movieURL := fmt.Sprintf("%s/movies/%s", base, code)

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	doc, status, err := p.fetchJavDatabaseHTML(ctx, movieURL, base)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound || doc == nil {
		return nil, metadata.ErrNotFound
	}

	actressLink, err := findJavDatabaseActressLink(doc)
	if err != nil {
		if errors.Is(err, errNoActressLink) {
			return nil, metadata.ErrNotFound
		}
		return nil, err
	}
	if actressLink == "" {
		return nil, metadata.ErrNotFound
	}
	actressURL := parseutil.ResolveURL(movieURL, actressLink)
	if actressURL == "" {
		return nil, metadata.ErrNotFound
	}

	actressDoc, status, err := p.fetchJavDatabaseHTML(ctx, actressURL, movieURL)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound || actressDoc == nil {
		return nil, metadata.ErrNotFound
	}

	info := parseJavDatabaseActressInfo(actressDoc)
	if info == nil {
		return nil, metadata.ErrNotFound
	}
	info.ProfileURL = actressURL
	return info, nil
}

// New creates a provider using the supplied non-nil HTTP client.
func New(httpClient *http.Client) *JavDatabaseClient {
	return &JavDatabaseClient{httpClient: httpClient, limiter: ratelimit.New(javDatabaseRequestInterval)}
}
