package util

import (
	"net/http"
	"sync"
)

// proxyTransport lazily rebuilds its connection pool when proxy settings change.
// Each client remembers its own version; no client can clear another's update flag.
type proxyTransport struct {
	mu        sync.Mutex
	version   uint64
	transport *http.Transport
	create    func() *http.Transport
}

func (t *proxyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	version := proxyVersion.Load()
	if t.transport == nil || t.version != version {
		if t.transport != nil {
			t.transport.CloseIdleConnections()
		}
		t.transport = t.create()
		t.version = version
	}
	transport := t.transport
	t.mu.Unlock()
	return transport.RoundTrip(req)
}

func (t *proxyTransport) CloseIdleConnections() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.transport != nil {
		t.transport.CloseIdleConnections()
	}
}
