//go:build updatetest

package update

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// Only explicitly tagged test builds can redirect updater traffic. Keep the
// official request URLs and redirect validation; replace only their transport.
// Missing/invalid configuration fails closed instead of contacting GitHub.
func newUpdateHTTPClient() *http.Client {
	target, err := url.Parse(os.Getenv("JAVBOSS_UPDATE_TEST_URL"))
	if err == nil && (target.Scheme != "http" || target.Hostname() != "127.0.0.1" || target.Port() == "" ||
		target.User != nil || target.Path != "" || target.RawQuery != "" || target.Fragment != "") {
		err = fmt.Errorf("JAVBOSS_UPDATE_TEST_URL must be http://127.0.0.1:<port>")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &http.Client{Transport: &localUpdateTransport{target: target, err: err, transport: transport}}
}

type localUpdateTransport struct {
	target    *url.URL
	err       error
	transport *http.Transport
}

func (t *localUpdateTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.err != nil {
		return nil, t.err
	}
	if req.URL.Scheme != "https" || req.URL.Host != "github.com" || req.URL.User != nil ||
		!strings.HasPrefix(req.URL.Path, "/"+repository+"/releases/") {
		return nil, fmt.Errorf("unexpected local update test request: %s", req.URL)
	}
	local := req.Clone(req.Context())
	local.URL.Scheme, local.URL.Host = t.target.Scheme, t.target.Host
	local.Host = t.target.Host
	return t.transport.RoundTrip(local)
}

func (t *localUpdateTransport) CloseIdleConnections() { t.transport.CloseIdleConnections() }
