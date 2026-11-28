package avmoo

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
	p.SetBaseURL("https://mirror.avmoo.test/")

	if _, err := p.fetchAvmooSession(context.Background(), p.BaseURL(), "ABC-001"); err == nil {
		t.Fatal("expected not found error")
	}
	if len(transport.requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(transport.requests))
	}
	if got := transport.requests[0].URL.Host; got != "mirror.avmoo.test" {
		t.Fatalf("request host = %q", got)
	}
	if got := transport.requests[0].Header.Get("Referer"); got != "https://mirror.avmoo.test" {
		t.Fatalf("referer = %q", got)
	}
	if got := avmooMovieInfoFromAPI(&avmooAPIMovie{MovieFanHao: "ABC-001", Title: "Title", SampleSmall: []string{"/pics/sample.jpg"}}, p.BaseURL()); len(got.SampleImages) != 1 || !strings.HasPrefix(got.SampleImages[0].ThumbnailURL, "https://mirror.avmoo.test/") {
		t.Fatalf("sample images = %+v", got.SampleImages)
	}
}

func TestBaseURLFallsBackOnInvalidValue(t *testing.T) {
	p := New(&http.Client{})
	p.SetBaseURL("ftp://mirror.avmoo.test")
	if got := p.BaseURL(); got != DefaultBaseURL {
		t.Fatalf("BaseURL() = %q, want default", got)
	}
}
