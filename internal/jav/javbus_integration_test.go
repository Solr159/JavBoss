package jav

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"javboss/internal/jav/javbus"
)

func TestJavBusLookupUsesPublisherAfterCacheUpdate(t *testing.T) {
	httpClient := &http.Client{}
	client := NewMetadataClient(map[Provider]any{ProviderJavBus: javbus.New(httpClient)}, newMemoryLookupCache())
	calls := 0
	httpClient.Transport = testRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`<html><body>
				<h3>MIDE-557 Test Title</h3>
				<p><span class="header">識別碼:</span><span>MIDE-557</span></p>
				<p><span class="header">製作商:</span> <a href="https://www.javbus.com/studio/4v">ムーディーズ</a></p>
				<p><span class="header">發行商:</span> <a href="https://www.javbus.com/label/1mh">MOODYZDIVA</a></p>
				</body></html>`)),
			Request: req,
		}, nil
	})
	lookupCacheSetHit(client, "v6:jav:javbus:lookup_jav:MIDE-557", &JavInfo{Code: "MIDE-557", Title: "Old result with producer", Studio: "ムーディーズ"})
	for i := 0; i < 2; i++ {
		info, err := client.LookupJavByCode(context.Background(), "MIDE-557", ProviderJavBus)
		if err != nil || info == nil || info.Studio != "MOODYZDIVA" {
			t.Fatalf("lookup %d: info=%+v err=%v", i, info, err)
		}
	}
	if calls != 1 {
		t.Fatalf("HTTP requests=%d, want 1 followed by a cache hit", calls)
	}
}

type testRoundTripFunc func(*http.Request) (*http.Response, error)

func (f testRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
