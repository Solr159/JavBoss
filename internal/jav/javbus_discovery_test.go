package jav

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"javboss/internal/util"

	"golang.org/x/net/html"
)

func TestFetchJavBusActressWorksReturnsBoundedWindow(t *testing.T) {
	client := util.DefaultHTTPClient()
	originalTransport := client.Transport
	t.Cleanup(func() {
		client.Transport = originalTransport
		resetJavBusRateLimiterForTest()
	})

	var fixture strings.Builder
	fixture.WriteString(`<html><body>`)
	for index := 1; index <= 15; index++ {
		code := fmt.Sprintf("ABC-%03d", index)
		releaseDate := fmt.Sprintf("2026-08-%02d", 18-index)
		fixture.WriteString(`<a class="movie-box" href="/` + code + `"><img title="` + code + `"><date>` + code + `</date><date>` + releaseDate + `</date></a>`)
	}
	fixture.WriteString(`</body></html>`)
	client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/uncensored/star/abc123" {
			t.Errorf("actress listing path = %q", req.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(fixture.String())),
			Request:    req,
		}, nil
	})

	resetJavBusRateLimiterForTest()
	items, err := FetchJavBusActressWorks(
		context.Background(),
		"/uncensored/star/abc123",
		"葵つかさ",
		JavBusActressWorksOptions{Offset: 2, Limit: 10},
	)
	if err != nil {
		t.Fatalf("fetch actress works: %v", err)
	}
	if len(items) != 10 || items[0].Code != "ABC-003" || items[9].Code != "ABC-012" {
		t.Fatalf("items = %#v", items)
	}

	releaseBoundary, err := time.Parse("2006-01-02", "2026-08-10")
	if err != nil {
		t.Fatalf("parse release boundary: %v", err)
	}
	resetJavBusRateLimiterForTest()
	newItems, err := FetchJavBusActressWorks(
		context.Background(),
		"/uncensored/star/abc123",
		"葵つかさ",
		JavBusActressWorksOptions{Limit: 100, ReleasedNotBefore: releaseBoundary.Unix()},
	)
	if err != nil {
		t.Fatalf("fetch new actress works: %v", err)
	}
	if len(newItems) != 8 || newItems[0].Code != "ABC-001" || newItems[7].Code != "ABC-008" {
		t.Fatalf("new items = %#v", newItems)
	}

	resetJavBusRateLimiterForTest()
	historyItems, err := FetchJavBusActressWorks(
		context.Background(),
		"/uncensored/star/abc123",
		"葵つかさ",
		JavBusActressWorksOptions{Limit: 10, ReleasedBefore: releaseBoundary.Unix()},
	)
	if err != nil {
		t.Fatalf("fetch historical actress works: %v", err)
	}
	if len(historyItems) != 7 || historyItems[0].Code != "ABC-009" || historyItems[6].Code != "ABC-015" {
		t.Fatalf("history items = %#v", historyItems)
	}
}

func TestFetchJavBusDiscoveryItemDetailsIncludesMagnetLinks(t *testing.T) {
	client := util.DefaultHTTPClient()
	originalTransport := client.Transport
	t.Cleanup(func() {
		client.Transport = originalTransport
		resetJavBusRateLimiterForTest()
	})

	requests := 0
	client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		var body string
		switch req.URL.Path {
		case "/ABC-001":
			body = `<html><body>
				<h3>ABC-001 Test title</h3>
				<p><span>識別碼:</span><span>ABC-001</span></p>
				<script>var gid = 12345; var uc = 0; var img = '/pics/cover/test.jpg';</script>
			</body></html>`
		case "/ajax/uncledatoolsbyajax.php":
			if req.URL.Query().Get("gid") != "12345" ||
				req.URL.Query().Get("uc") != "0" ||
				req.URL.Query().Get("img") != "/pics/cover/test.jpg" {
				t.Errorf("magnet query = %q", req.URL.RawQuery)
			}
			if req.Header.Get("Referer") != "https://www.javbus.com/ABC-001" {
				t.Errorf("magnet referer = %q", req.Header.Get("Referer"))
			}
			body = `<tr>
				<td><a href="magnet:?xt=urn:btih:ABCDEF123456&amp;dn=ABC-001-HD">ABC-001-HD</a><span title="包含高清HD的磁力連結">高清</span><span title="包含字幕的磁力連結">字幕</span></td>
				<td>2.50GB</td><td>2026-08-18</td>
			</tr>`
		default:
			t.Fatalf("unexpected request URL: %s", req.URL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	})

	resetJavBusRateLimiterForTest()
	details, err := FetchJavBusDiscoveryItemDetails(context.Background(), "ABC-001")
	if err != nil {
		t.Fatalf("fetch discovery details: %v", err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want detail and magnet requests", requests)
	}
	if len(details.MagnetLinks) != 1 {
		t.Fatalf("magnet links = %#v", details.MagnetLinks)
	}
	link := details.MagnetLinks[0]
	if link.Name != "ABC-001-HD" || link.Size != "2.50GB" || link.ShareDate != "2026-08-18" ||
		!link.HD || !link.Subtitled || !strings.HasPrefix(link.URL, "magnet:?xt=urn:btih:") {
		t.Fatalf("magnet link = %#v", link)
	}
}

func TestResolveJavBusActressSubscriptionUsesExactCodeOnly(t *testing.T) {
	client := util.DefaultHTTPClient()
	originalTransport := client.Transport
	t.Cleanup(func() {
		client.Transport = originalTransport
		resetJavBusRateLimiterForTest()
	})

	returnedCode := "ABC-001"
	client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`
				<html><body>
					<h3>` + returnedCode + ` Test Title</h3>
					<p><span>識別碼:</span><span>` + returnedCode + `</span></p>
					<div class="movie row"><a href="/uncensored/star/abc123">葵つかさ</a></div>
				</body></html>`)),
			Request: req,
		}, nil
	})

	resetJavBusRateLimiterForTest()
	resolved, err := ResolveJavBusActressSubscription(context.Background(), "abc-001")
	if err != nil {
		t.Fatalf("resolve subscription: %v", err)
	}
	if resolved.Name != "葵つかさ" || resolved.ProviderLocator != "/uncensored/star/abc123" {
		t.Fatalf("resolved subscription = %#v", resolved)
	}

	returnedCode = "ABC-002"
	resetJavBusRateLimiterForTest()
	if _, err := ResolveJavBusActressSubscription(context.Background(), "ABC-001"); err != ResourceNotFonud {
		t.Fatalf("mismatched code error = %v, want %v", err, ResourceNotFonud)
	}
}

func TestParseJavBusActressLinks(t *testing.T) {
	doc := mustParseJavBusDiscoveryHTML(t, `
		<html><body><div class="movie row">
			<a href="/star/abc123">葵つかさ</a>
			<a href="/star/abc123"><img alt=""></a>
		</div></body></html>`)

	got := parseJavBusActressLinks(doc)
	if len(got) != 1 {
		t.Fatalf("links len = %d, want 1", len(got))
	}
	if got[0].Name != "葵つかさ" || javBusActressLocator(got[0].URL) != "/star/abc123" {
		t.Fatalf("unexpected link: %#v", got[0])
	}
}

func TestParseJavBusDiscoveryItems(t *testing.T) {
	doc := mustParseJavBusDiscoveryHTML(t, `
		<html><body>
			<a class="movie-box" href="/ABP-123">
				<div class="photo-frame"><img title="Sample title" data-original="/pics/cover.jpg"></div>
				<div class="photo-info"><date>ABP-123</date><date>2026-07-29</date></div>
			</a>
		</body></html>`)

	got := parseJavBusDiscoveryItems(doc, "https://www.javbus.com/star/abc", "葵つかさ")
	if len(got) != 1 {
		t.Fatalf("items len = %d, want 1", len(got))
	}
	if got[0].Code != "ABP-123" || got[0].Title != "Sample title" {
		t.Fatalf("unexpected item: %#v", got[0])
	}
	if got[0].ReleaseUnix == 0 {
		t.Fatalf("release unix was not parsed")
	}
	if got[0].CoverURL != "https://www.javbus.com/pics/cover.jpg" {
		t.Fatalf("cover url = %q", got[0].CoverURL)
	}
}

func TestParseJavBusNextActressPageURLRejectsOtherStar(t *testing.T) {
	doc := mustParseJavBusDiscoveryHTML(t, `<html><body><ul><li class="next"><a href="/star/other/2">Next</a></li></ul></body></html>`)
	if got := parseJavBusNextActressPageURL(doc, "https://www.javbus.com/star/abc", "/star/abc"); got != "" {
		t.Fatalf("next url = %q, want empty", got)
	}
}

func TestParseJavBusNextActressPageURLRejectsOtherNamespaceWithSameKey(t *testing.T) {
	doc := mustParseJavBusDiscoveryHTML(t, `<html><body><ul><li class="next"><a href="/star/abc/2">Next</a></li></ul></body></html>`)
	if got := parseJavBusNextActressPageURL(doc, "https://www.javbus.com/uncensored/star/abc", "/uncensored/star/abc"); got != "" {
		t.Fatalf("next url = %q, want empty", got)
	}
}

func TestJavBusActressLocatorPreservesNamespace(t *testing.T) {
	tests := map[string]string{
		"https://www.javbus.com/star/abc123":             "/star/abc123",
		"https://www.javbus.com/uncensored/star/abc123/": "/uncensored/star/abc123",
		"/uncensored/star/abc123":                        "/uncensored/star/abc123",
		"https://example.com/uncensored/star/abc123":     "",
		"/uncensored/star/abc123/2":                      "",
	}
	for input, want := range tests {
		if got := javBusActressLocator(input); got != want {
			t.Errorf("javBusActressLocator(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestIsJavBusVerificationPage(t *testing.T) {
	doc := mustParseJavBusDiscoveryHTML(t, `<html><head><title>Age Verification JavBus</title></head><body></body></html>`)
	if !isJavBusVerificationPage(doc) {
		t.Fatal("expected age verification page to be detected")
	}
}

func mustParseJavBusDiscoveryHTML(t *testing.T, fixture string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return doc
}
