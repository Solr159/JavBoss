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
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Hostname() == "" {
			continue
		}
		if strings.EqualFold(parsed.Scheme, "https") && imageOriginPort(parsed) == "443" &&
			isOfficialJavImageHost(strings.ToLower(parsed.Hostname())) {
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

// SetJavImageRequestHeaders applies image-source headers using the request URL.
func SetJavImageRequestHeaders(req *http.Request) {
	if req == nil || req.URL == nil {
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/*,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	host := strings.ToLower(req.URL.Hostname())
	if source, ok := configuredJavImageSource(req.URL); ok {
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

// Prefer an exact origin, then the closest matching CDN parent on the same
// scheme and port.
func configuredJavImageSource(target *url.URL) (JavImageSource, bool) {
	host := strings.ToLower(target.Hostname())
	port := imageOriginPort(target)
	var fallback JavImageSource
	parentLength := 0
	for _, source := range javImageSources() {
		origin, err := url.Parse(source.Origin)
		if err != nil || !strings.EqualFold(target.Scheme, origin.Scheme) || port != imageOriginPort(origin) {
			continue
		}
		originHost := strings.ToLower(origin.Hostname())
		if host == originHost {
			return source, true
		}
		if len(originHost) > parentLength && strings.HasSuffix(host, "."+originHost) {
			fallback = source
			parentLength = len(originHost)
		}
	}
	return fallback, parentLength > 0
}

func imageOriginPort(origin *url.URL) string {
	if port := origin.Port(); port != "" {
		return port
	}
	switch strings.ToLower(origin.Scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

// isOfficialJavImageHost identifies hosts covered by the built-in header rules.
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
