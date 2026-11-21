package util

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// JavImageSource describes the headers one scraper origin expects when its
// images are proxied. Origin must be a scheme://host[:port] value.
type JavImageSource struct {
	Origin string
	Cookie string
}

var javImageSourceState = struct {
	sync.RWMutex
	sources []JavImageSource
}{}

// SetJavImageSources replaces the registered scraper origins used for image
// Referer and Cookie headers. Passing nil restores the built-in defaults.
func SetJavImageSources(sources []JavImageSource) {
	clean := make([]JavImageSource, 0, len(sources))
	for _, source := range sources {
		origin := strings.TrimRight(strings.TrimSpace(source.Origin), "/")
		host := imageSourceHost(origin)
		if host == "" || isOfficialJavImageHost(host) {
			continue
		}
		clean = append(clean, JavImageSource{Origin: origin, Cookie: source.Cookie})
	}
	javImageSourceState.Lock()
	javImageSourceState.sources = clean
	javImageSourceState.Unlock()
}

func javImageSources() []JavImageSource {
	javImageSourceState.RLock()
	sources := javImageSourceState.sources
	javImageSourceState.RUnlock()
	return sources
}

// SetJavImageRequestHeaders applies image-source headers using the request host.
func SetJavImageRequestHeaders(req *http.Request) {
	if req == nil || req.URL == nil {
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/*,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	host := strings.ToLower(req.URL.Hostname())
	if source, ok := configuredJavImageSource(host); ok {
		req.Header.Set("Referer", source.Origin+"/")
		if source.Cookie != "" {
			req.Header.Set("Cookie", source.Cookie)
		}
		return
	}
	switch {
	case host == "javbus.com" || strings.HasSuffix(host, ".javbus.com"):
		req.Header.Set("Referer", "https://www.javbus.com/")
		req.Header.Set("Cookie", "age=verified; existmag=mag")
	case host == "dmm.co.jp" || strings.HasSuffix(host, ".dmm.co.jp"):
		req.Header.Set("Referer", "https://www.dmm.co.jp/")
	}
}

// configuredJavImageSource matches the request host against configured origins,
// including subdomains such as an image CDN of the same site.
func configuredJavImageSource(host string) (JavImageSource, bool) {
	for _, source := range javImageSources() {
		originHost := imageSourceHost(source.Origin)
		if originHost == "" {
			continue
		}
		if host == originHost || strings.HasSuffix(host, "."+originHost) {
			return source, true
		}
	}
	return JavImageSource{}, false
}

func imageSourceHost(origin string) string {
	parsed, err := url.Parse(origin)
	if err != nil || parsed == nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}

// isOfficialJavImageHost keeps the built-in host rules authoritative so a
// default origin never overrides the official site behavior.
func isOfficialJavImageHost(host string) bool {
	switch {
	case host == "javbus.com" || strings.HasSuffix(host, ".javbus.com"):
		return true
	case host == "dmm.co.jp" || strings.HasSuffix(host, ".dmm.co.jp"):
		return true
	default:
		return false
	}
}
