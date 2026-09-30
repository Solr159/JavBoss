package util

import (
	"net/http"
	"sync"
)

// HTTPProbe owns a fresh HTTP client and records its response status.
// Isolation is established by the caller creating a new client and transport,
// not by switching clients while a request is being executed.
type HTTPProbe struct {
	Client     *http.Client
	mu         sync.Mutex
	lastStatus int
}

// NewHTTPProbe takes ownership of a freshly created, uncached client and adds
// status recording to its transport. The caller must supply an independent
// transport and must not already be using the client.
func NewHTTPProbe(client *http.Client) *HTTPProbe {
	p := &HTTPProbe{Client: client}
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	client.Transport = &probeTransport{base: transport, probe: p}
	return p
}

func (p *HTTPProbe) HTTPStatus() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastStatus
}

// Close releases the owned client's idle connections.
func (p *HTTPProbe) Close() { p.Client.CloseIdleConnections() }

type probeTransport struct {
	base  http.RoundTripper
	probe *HTTPProbe
}

func (t *probeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if resp != nil {
		t.probe.mu.Lock()
		t.probe.lastStatus = resp.StatusCode
		t.probe.mu.Unlock()
	}
	return resp, err
}

func (t *probeTransport) CloseIdleConnections() {
	if transport, ok := t.base.(interface{ CloseIdleConnections() }); ok {
		transport.CloseIdleConnections()
	}
}
