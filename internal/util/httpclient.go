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
	defaultHTTPClientOnce       sync.Once
	defaultHTTPClient           *http.Client
	defaultCachedHTTPClientOnce sync.Once
	defaultCachedHTTPClient     *http.Client
	notFoundURLCache            sync.Map // URL -> expiration time.Time
)

// ErrCachedNotFound indicates the URL was previously requested and returned 404.
var ErrCachedNotFound = errors.New("cached not found")

// WithNotFoundCache returns a shallow copy of client whose transport caches 404
// URLs for seven days. It shares the original connection pool and cookie jar.
// Configure this once before using the returned client; do not use it for checks
// that must reach the network regardless of previous responses.
func WithNotFoundCache(client *http.Client) *http.Client {
	copy := *client
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	copy.Transport = &notFoundCacheTransport{base: transport}
	return &copy
}

type notFoundCacheTransport struct {
	base http.RoundTripper
}

func (t *notFoundCacheTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	url := req.URL.String()
	if expiresAt, ok := notFoundURLCache.Load(url); ok {
		if time.Now().Before(expiresAt.(time.Time)) {
			return nil, ErrCachedNotFound
		}
		// Preserve a newer entry if another request refreshed it concurrently.
		notFoundURLCache.CompareAndDelete(url, expiresAt)
	}
	resp, err := t.base.RoundTrip(req)
	if err == nil && resp != nil && resp.StatusCode == http.StatusNotFound {
		notFoundURLCache.Store(url, time.Now().Add(notFoundURLCacheTTL))
	}
	return resp, err
}

func (t *notFoundCacheTransport) CloseIdleConnections() {
	if transport, ok := t.base.(interface{ CloseIdleConnections() }); ok {
		transport.CloseIdleConnections()
	}
}

// DefaultHTTPClient returns the shared proxy-aware HTTP client used across the app.
// It is initialized once with sane defaults similar to curl.
func DefaultHTTPClient() *http.Client {
	defaultHTTPClientOnce.Do(func() {
		defaultHTTPClient = NewDefaultHTTPClient()
	})
	return defaultHTTPClient
}

// DefaultCachedHTTPClient returns the shared client with a seven-day URL 404
// cache. It is initialized once and shares DefaultHTTPClient's connection pool.
func DefaultCachedHTTPClient() *http.Client {
	defaultCachedHTTPClientOnce.Do(func() {
		defaultCachedHTTPClient = WithNotFoundCache(DefaultHTTPClient())
	})
	return defaultCachedHTTPClient
}

// NewDefaultHTTPClient creates a fresh client with the application's default
// timeout and transport settings. It has its own pool and no URL cache.
func NewDefaultHTTPClient() *http.Client {
	return NewHTTPClientWithTransport(10*time.Second, func(t *http.Transport) {
		t.ForceAttemptHTTP2 = false // closer to curl defaults
		t.DisableCompression = true // avoid implicit gzip
		t.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS13}
		t.MaxIdleConns = 200
		t.MaxIdleConnsPerHost = 20
		t.MaxConnsPerHost = 50
	})
}

// NewHTTPClient returns an http.Client with a proxy-aware transport and the provided timeout.
func NewHTTPClient(timeout time.Duration) *http.Client {
	return NewHTTPClientWithTransport(timeout, nil)
}

// NewHTTPClientWithTransport configures a transport on first use and rebuilds it
// after proxy settings change. The outer client retains its timeout and redirect policy.
func NewHTTPClientWithTransport(timeout time.Duration, configure func(*http.Transport)) *http.Client {
	create := func() *http.Transport {
		transport := &http.Transport{
			Proxy: DetectProxyFunc(),
			// Retired transports may still have active requests. Let their connections
			// expire after becoming idle, without tracking or wrapping response bodies.
			IdleConnTimeout: 90 * time.Second,
		}
		if configure != nil {
			configure(transport)
		}
		return transport
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: &proxyTransport{create: create},
	}
}
