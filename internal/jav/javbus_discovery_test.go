package jav

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

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
	if got[0].Name != "葵つかさ" || javBusStarKey(got[0].URL) != "abc123" {
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
	if got := parseJavBusNextActressPageURL(doc, "https://www.javbus.com/star/abc", "abc"); got != "" {
		t.Fatalf("next url = %q, want empty", got)
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
