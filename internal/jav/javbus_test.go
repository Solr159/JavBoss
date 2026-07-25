package jav

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"
)

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
