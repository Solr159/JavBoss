package avmoo

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"javboss/internal/jav/internal/ratelimit"
	"javboss/internal/jav/metadata"
)

// AvmooClient retrieves metadata from avmoo.
type AvmooClient struct {
	httpClient   *http.Client
	sessionCache struct {
		sync.Mutex
		session   avmooSession
		expiresAt time.Time
	}
	limiter *ratelimit.Limiter
}

const (
	avmooBaseURL         = "https://avmoo.shop"
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

	info := avmooMovieInfoFromAPI(movie)
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
