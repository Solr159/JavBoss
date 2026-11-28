package javmenu

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type baseURLTransport struct{ requests []*http.Request }

func (t *baseURLTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.requests = append(t.requests, req)
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Status:     "404 Not Found",
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("")),
	}, nil
}

func TestBaseURLOverride(t *testing.T) {
	transport := &baseURLTransport{}
	p := New(&http.Client{Transport: transport})
	if p.OriginURL() != DefaultBaseURL {
		t.Fatalf("default origin = %q", p.OriginURL())
	}
	p.SetBaseURL("https://mirror.javmenu.test/")

	if _, _, err := p.fetchJavMenuDetailByCode(context.Background(), "abc-001"); err == nil {
		t.Fatal("expected not found error")
	}
	if len(transport.requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(transport.requests))
	}
	if got := transport.requests[0].URL.String(); got != "https://mirror.javmenu.test/ABC-001" {
		t.Fatalf("request url = %q", got)
	}
}

func TestBaseURLFallsBackOnInvalidValue(t *testing.T) {
	p := New(&http.Client{})
	p.SetBaseURL("")
	if got := p.OriginURL(); got != DefaultBaseURL {
		t.Fatalf("OriginURL() = %q, want default", got)
	}
}
