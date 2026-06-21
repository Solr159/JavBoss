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
