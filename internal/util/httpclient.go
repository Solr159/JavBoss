package util

import (
	"crypto/tls"
	"errors"
	"net/http"
	"sync"
	"time"
)

const notFoundURLCacheTTL = 7 * 24 * time.Hour

var (
	defaultHTTPClientOnce sync.Once
	defaultHTTPClient     *http.Client
	notFoundURLCache      sync.Map // URL -> expiration time.Time
)

// ErrCachedNotFound indicates the URL was previously requested and returned 404.
var ErrCachedNotFound = errors.New("cached not found")

// DoRequest caches missing URLs for seven days, then allows another request.
func DoRequest(req *http.Request) (*http.Response, error) {
	if req == nil || req.URL == nil {
		return nil, errors.New("nil request")
	}
	url := req.URL.String()
	if url != "" {
		if expiresAt, ok := notFoundURLCache.Load(url); ok {
			if time.Now().Before(expiresAt.(time.Time)) {
				return nil, ErrCachedNotFound
			}
			// Preserve a newer entry if another request refreshed it concurrently.
			notFoundURLCache.CompareAndDelete(url, expiresAt)
		}
	}
	resp, err := DefaultHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound && url != "" {
		notFoundURLCache.Store(url, time.Now().Add(notFoundURLCacheTTL))
	}
	return resp, nil
}

// DefaultHTTPClient returns the shared proxy-aware HTTP client used across the app.
// It is initialized once with sane defaults similar to curl.
func DefaultHTTPClient() *http.Client {
	defaultHTTPClientOnce.Do(func() {
		defaultHTTPClient = NewHTTPClientWithTransport(10*time.Second, func(t *http.Transport) {
			t.ForceAttemptHTTP2 = false // closer to curl defaults
			t.DisableCompression = true // avoid implicit gzip
			t.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS13}
			t.MaxIdleConns = 200
			t.MaxIdleConnsPerHost = 20
			t.MaxConnsPerHost = 50
		})
	})
	return defaultHTTPClient
}

// NewHTTPClient returns an http.Client with a proxy-aware transport and the provided timeout.
func NewHTTPClient(timeout time.Duration) *http.Client {
	return NewHTTPClientWithTransport(timeout, nil)
}

// NewHTTPClientWithTransport allows customizing the base transport while keeping proxy auto-detection.
func NewHTTPClientWithTransport(timeout time.Duration, configure func(*http.Transport)) *http.Client {
	transport := &http.Transport{
		Proxy: DetectProxyFunc(),
	}
	if configure != nil {
		configure(transport)
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}
