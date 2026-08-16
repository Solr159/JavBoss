package jav

import (
	"context"
	"errors"
	"io"
	"javboss/internal/util"
	"net/http"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func resetJavBusRateLimiterForTest() {
	javBusRateLimiter.Lock()
	javBusRateLimiter.next = time.Time{}
	javBusRateLimiter.Unlock()
}

func TestJavBusRateLimiterSpacesRequests(t *testing.T) {
	resetJavBusRateLimiterForTest()
	t.Cleanup(resetJavBusRateLimiterForTest)

	start := time.Now()
	for i := 0; i < 5; i++ {
		if err := waitForJavBusRateLimit(context.Background()); err != nil {
			t.Fatalf("waitForJavBusRateLimit() request %d: %v", i+1, err)
		}
	}

	if elapsed := time.Since(start); elapsed < (4*javBusRequestInterval - 50*time.Millisecond) {
		t.Fatalf("rate limiter allowed 5 requests in %s", elapsed)
	}
}

func TestJavBusRateLimiterHonorsContext(t *testing.T) {
	resetJavBusRateLimiterForTest()
	t.Cleanup(resetJavBusRateLimiterForTest)

	javBusRateLimiter.Lock()
	javBusRateLimiter.next = time.Now().Add(time.Hour)
	javBusRateLimiter.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := waitForJavBusRateLimit(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waitForJavBusRateLimit() err = %v, want context deadline exceeded", err)
	}
}

func TestParseJavBusCoverURL(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
		<html>
			<head>
				<meta property="og:image" content="/pics/cover/c85j_b.jpg">
			</head>
			<body>
				<a class="bigImage" href="https://www.javbus.com/pics/cover/fallback_b.jpg">
					<img src="/pics/cover/fallback_b.jpg">
				</a>
			</body>
		</html>`))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	got := parseJavBusCoverURL(doc, "https://www.javbus.com/ABC-001")
	if got != "https://www.javbus.com/pics/cover/c85j_b.jpg" {
		t.Fatalf("unexpected cover url: %q", got)
	}
}

func TestParseJavBusGenreCategories(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
		<html><body>
			<h4>主題</h4>
			<div class="row genre-box">
				<a href="https://www.javbus.com/genre/62">折磨</a>
				<a href="/genre/59">觸手</a>
				<a href="/star/not-a-genre">忽略</a>
			</div>
			<h4>服裝</h4>
			<div class="row genre-box">
				<a href="/genre/4f">制服</a>
			</div>
		</body></html>`))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	got := parseJavBusGenreCategories(doc, "/genre/")
	want := []JavBusGenreCategory{
		{Name: "折磨", Category: "主题"},
		{Name: "觸手", Category: "主题"},
		{Name: "制服", Category: "服装"},
	}
	if len(got) != len(want) {
		t.Fatalf("categories length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("category %d = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestParseJavBusUncensoredGenreCategoriesOnlyUsesRequestedPath(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
		<html><body>
			<h4>場景</h4>
			<div class="genre-box">
				<a href="/uncensored/genre/abc">室外</a>
				<a href="/genre/abc">有码标签</a>
			</div>
		</body></html>`))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	got := parseJavBusGenreCategories(doc, "/uncensored/genre/")
	if len(got) != 1 || got[0].Name != "室外" || got[0].Category != "场景" {
		t.Fatalf("unexpected categories: %#v", got)
	}
}

func TestParseJavBusMovieInfoIncludesCoverURL(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
		<html>
			<head>
				<meta property="og:image" content="https://www.javbus.com/pics/cover/c85j_b.jpg">
			</head>
			<body>
				<h3>ABC-001 Test Title</h3>
				<p><span>識別碼:</span><span>ABC-001</span></p>
				<div id="sample-waterfall">
					<a href="https://pics.dmm.co.jp/abc001jp-1.jpg">
						<img src="https://www.javbus.com/pics/sample/abc001-1.jpg">
					</a>
				</div>
			</body>
		</html>`))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	info := parseDocument(doc)
	if info == nil {
		t.Fatal("expected info, got nil")
	}
	if info.CoverURL != "https://www.javbus.com/pics/cover/c85j_b.jpg" {
		t.Fatalf("unexpected cover url: %q", info.CoverURL)
	}
	if len(info.SampleImages) != 1 ||
		info.SampleImages[0].ThumbnailURL != "https://www.javbus.com/pics/sample/abc001-1.jpg" ||
		info.SampleImages[0].DetailURL != "https://pics.dmm.co.jp/abc001jp-1.jpg" {
		t.Fatalf("unexpected sample images: %#v", info.SampleImages)
	}
}

func TestParseJavBusMovieInfoIncludesSeries(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
		<html>
			<body>
				<h3>ABC-001 Test Title</h3>
				<p><span>識別碼:</span><span>ABC-001</span></p>
				<p><span>系列:</span><span><a href="/series/abc">测试系列</a></span></p>
			</body>
		</html>`))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	info := parseDocument(doc)
	if info == nil {
		t.Fatal("expected info, got nil")
	}
	if info.Series != "测试系列" {
		t.Fatalf("unexpected series: %q", info.Series)
	}
}

func TestJavBusLookupCodeRewritesSpecialPrefixes(t *testing.T) {
	cases := []struct {
		code string
		want string
	}{
		{code: "gana-001", want: "200gana-001"},
		{code: "MIUM-001", want: "300mium-001"},
		{code: "luxu-001", want: "259luxu-001"},
	}
	for _, tc := range cases {
		got, rewrite := javBusLookupCode(tc.code)
		if got != tc.want || rewrite == nil {
			t.Fatalf("javBusLookupCode(%q) = %q, %#v; want %q with rewrite", tc.code, got, rewrite, tc.want)
		}
	}

	got, rewrite := javBusLookupCode("ABC-001")
	if got != "ABC-001" || rewrite != nil {
		t.Fatalf("javBusLookupCode(ABC-001) = %q, %#v; want ABC-001 without rewrite", got, rewrite)
	}
}

func TestJavBusLookupJavByCodeRewritesSpecialPrefixes(t *testing.T) {
	client := util.DefaultHTTPClient()
	originalTransport := client.Transport
	t.Cleanup(func() {
		client.Transport = originalTransport
		resetJavBusRateLimiterForTest()
	})

	var requestedPaths []string
	client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestedPaths = append(requestedPaths, req.URL.Path)
		code := strings.TrimPrefix(req.URL.Path, "/")
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`
				<html>
					<head>
						<meta property="og:image" content="/pics/cover/test_b.jpg">
					</head>
					<body>
						<h3>` + code + ` Test Title</h3>
						<p><span>識別碼:</span><span>` + code + `</span></p>
					</body>
				</html>`)),
			Request: req,
		}, nil
	})

	cases := []struct {
		code     string
		wantPath string
	}{
		{code: "gana-001", wantPath: "/200gana-001"},
		{code: "MIUM-001", wantPath: "/300mium-001"},
		{code: "luxu-001", wantPath: "/259luxu-001"},
	}
	for _, tc := range cases {
		resetJavBusRateLimiterForTest()
		info, err := (javBus{}).LookupJavByCode(tc.code)
		if err != nil {
			t.Fatalf("LookupJavByCode(%q) error: %v", tc.code, err)
		}
		if info == nil || info.CoverURL != "https://www.javbus.com/pics/cover/test_b.jpg" {
			t.Fatalf("LookupJavByCode(%q) info = %#v, want cover URL", tc.code, info)
		}
	}

	if len(requestedPaths) != len(cases) {
		t.Fatalf("requested paths = %v, want %d requests", requestedPaths, len(cases))
	}
	for i, tc := range cases {
		if requestedPaths[i] != tc.wantPath {
			t.Errorf("request %d path = %q, want %q", i, requestedPaths[i], tc.wantPath)
		}
	}
}

func TestNormalizeJavBusRewrittenInfoRemovesRequestPrefix(t *testing.T) {
	cases := []struct {
		code      string
		title     string
		wantCode  string
		wantTitle string
	}{
		{code: "200GANA-001", title: "200GANA-001 Test Title", wantCode: "GANA-001", wantTitle: "Test Title"},
		{code: "300MIUM-001", title: "300MIUM-001 Test Title", wantCode: "MIUM-001", wantTitle: "Test Title"},
		{code: "259LUXU-001", title: "259LUXU-001 Test Title", wantCode: "LUXU-001", wantTitle: "Test Title"},
	}
	for _, tc := range cases {
		_, rewrite := javBusLookupCode(tc.wantCode)
		info := &JavInfo{
			Code:  tc.code,
			Title: tc.title,
		}

		normalizeJavBusRewrittenInfo(info, rewrite)

		if info.Code != tc.wantCode {
			t.Fatalf("unexpected code for %s: %q", tc.code, info.Code)
		}
		if info.Title != tc.wantTitle {
			t.Fatalf("unexpected title for %s: %q", tc.code, info.Title)
		}
	}
}

func TestParseJavBusUncensoredFromFixture(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
		<html>
			<head>
				<title>051526-001 Test Title - JavBus</title>
			</head>
			<body>
				<ul class="nav navbar-nav">
					<li><a href="https://www.javbus.com/">有碼</a></li>
					<li class="active"><a href="https://www.javbus.com/uncensored">無碼</a></li>
				</ul>
				<div class="movie row">
					<h3>051526-001 Test Title</h3>
					<p><span>識別碼:</span><span>051526-001</span></p>
				</div>
			</body>
		</html>`))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	info := parseDocument(doc)
	if info == nil {
		t.Fatal("expected info, got nil")
	}
	if info.Code != "051526-001" {
		t.Fatalf("unexpected code: %q", info.Code)
	}
	if info.IsUncensored == nil || !*info.IsUncensored {
		t.Fatal("expected uncensored javbus page")
	}
}

func TestParseJavBusCensoredNavIsNotUncensored(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
		<html>
			<body>
				<ul class="nav navbar-nav">
					<li class="active"><a href="https://www.javbus.com/">有碼</a></li>
					<li><a href="https://www.javbus.com/uncensored">無碼</a></li>
				</ul>
			</body>
		</html>`))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	if parseJavBusIsUncensored(doc) {
		t.Fatal("did not expect censored javbus page to be marked uncensored")
	}
}
