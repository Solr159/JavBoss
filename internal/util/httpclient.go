package util

import (
	"crypto/tls"
	"errors"
	"net/http"
	"sync"
	"time"
)

const negativeURLCacheTTL = 7 * 24 * time.Hour

var (
	defaultHTTPClientOnce       sync.Once
	defaultHTTPClient           *http.Client
	defaultCachedHTTPClientOnce sync.Once
	defaultCachedHTTPClient     *http.Client
	negativeURLCache            sync.Map // URL -> negativeURLCacheEntry
)

// ErrCachedNotFound indicates the URL was previously requested and returned 404.
var ErrCachedNotFound = errors.New("cached not found")

// ErrCachedForbidden indicates a GET or HEAD URL previously returned 403.
var ErrCachedForbidden = errors.New("cached forbidden")

type negativeURLCacheEntry struct {
	statusCode int
	expiresAt  time.Time
}

// WithNegativeCache returns a shallow copy of client whose transport caches 404
// responses and GET/HEAD 403 responses for seven days. It shares the original
// connection pool and cookie jar. POST authentication retries bypass cached 403s.
// Configure this once before using the returned client; do not use it for checks
// that must reach the network regardless of previous responses.
func WithNegativeCache(client *http.Client) *http.Client {
	copy := *client
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	copy.Transport = &negativeCacheTransport{base: transport}
	return &copy
}

type negativeCacheTransport struct {
	base http.RoundTripper
}

func (t *negativeCacheTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	url := req.URL.String()
	cacheForbidden := req.Method == "" || req.Method == http.MethodGet || req.Method == http.MethodHead
	if cached, ok := negativeURLCache.Load(url); ok {
		entry := cached.(negativeURLCacheEntry)
		if time.Now().Before(entry.expiresAt) {
			switch entry.statusCode {
			case http.StatusNotFound:
				return nil, ErrCachedNotFound
			case http.StatusForbidden:
				if cacheForbidden {
					return nil, ErrCachedForbidden
				}
			}
		} else {
			// Preserve a newer entry if another request refreshed it concurrently.
			negativeURLCache.CompareAndDelete(url, cached)
		}
	}
	resp, err := t.base.RoundTrip(req)
	if err == nil && resp != nil && (resp.StatusCode == http.StatusNotFound ||
		(cacheForbidden && resp.StatusCode == http.StatusForbidden)) {
		negativeURLCache.Store(url, negativeURLCacheEntry{
			statusCode: resp.StatusCode,
			expiresAt:  time.Now().Add(negativeURLCacheTTL),
		})
	}
	return resp, err
}

func (t *negativeCacheTransport) CloseIdleConnections() {
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

// DefaultCachedHTTPClient returns the shared client with a seven-day URL
// cache for 404 and GET/HEAD 403 responses. It is initialized once and shares DefaultHTTPClient's connection pool.
func DefaultCachedHTTPClient() *http.Client {
	defaultCachedHTTPClientOnce.Do(func() {
		defaultCachedHTTPClient = WithNegativeCache(DefaultHTTPClient())
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
