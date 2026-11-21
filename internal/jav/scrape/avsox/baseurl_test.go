package avsox

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
	p.SetBaseURL("https://mirror.avsox.test/")

	if _, err := p.fetchAvsoxSession(context.Background(), "030919_047"); err == nil {
		t.Fatal("expected not found error")
	}
	if len(transport.requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(transport.requests))
	}
	if got := transport.requests[0].URL.Host; got != "mirror.avsox.test" {
		t.Fatalf("request host = %q", got)
	}
	detailURL := avsoxMovieDetailURL(&avsoxAPIMovie{MovieID: "abc123"}, p.BaseURL())
	if detailURL != "https://mirror.avsox.test/cn/movie/abc123" {
		t.Fatalf("detail url = %q", detailURL)
	}
}

func TestBaseURLFallsBackOnInvalidValue(t *testing.T) {
	p := New(&http.Client{})
	p.SetBaseURL("https://mirror.avsox.test/cn")
	if got := p.BaseURL(); got != DefaultBaseURL {
		t.Fatalf("BaseURL() = %q, want default", got)
	}
}
