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
