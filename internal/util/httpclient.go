package util

import (
	"crypto/tls"
	"net/http"
	"strconv"
	"sync"
	"time"

	"javboss/internal/common/logging"
)

const negativeURLCacheTTL = 7 * 24 * time.Hour

var (
	defaultHTTPClientOnce       sync.Once
	defaultHTTPClient           *http.Client
	defaultCachedHTTPClientOnce sync.Once
	defaultCachedHTTPClient     *http.Client
	negativeURLCache            sync.Map // URL -> negativeURLCacheEntry
)

type negativeURLCacheEntry struct {
	statusCode int
	expiresAt  time.Time
}

// WithNegativeCache returns a shallow copy of client whose transport caches
// GET/HEAD 403 and 404 responses for seven days. It shares the original
// connection pool and cookie jar. Other methods, including POST lookups and
// authentication retries, bypass this URL-only cache.
// Cache hits return an HTTP response with the cached status and an empty body,
// so callers handle cached and network statuses through the same code path.
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
	if req.Method != "" && req.Method != http.MethodGet && req.Method != http.MethodHead {
		return t.base.RoundTrip(req)
	}
	if cached, ok := negativeURLCache.Load(url); ok {
		entry := cached.(negativeURLCacheEntry)
		if time.Now().Before(entry.expiresAt) {
			if entry.statusCode == http.StatusNotFound || entry.statusCode == http.StatusForbidden {
				logging.Info("http response status: %d %s (cached) method=%s url=%s expires_at=%s",
					entry.statusCode, http.StatusText(entry.statusCode), req.Method, url, entry.expiresAt.Format(time.RFC3339))
				return &http.Response{
					StatusCode: entry.statusCode,
					Status:     strconv.Itoa(entry.statusCode) + " " + http.StatusText(entry.statusCode),
					Proto:      "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
					Header: make(http.Header), Body: http.NoBody, ContentLength: 0,
					Request: req,
				}, nil
			}
		} else {
			// Preserve a newer entry if another request refreshed it concurrently.
			negativeURLCache.CompareAndDelete(url, cached)
		}
	}
	resp, err := t.base.RoundTrip(req)
	if err == nil && resp != nil && (resp.StatusCode == http.StatusNotFound ||
		resp.StatusCode == http.StatusForbidden) {
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
// cache for GET/HEAD 403 and 404 responses. It is initialized once and shares DefaultHTTPClient's connection pool.
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
