package avshared

import (
	"net/http"
	"regexp"
	"strings"
)

func ExtractCSRFToken(body string) string {
	re := regexp.MustCompile(`<meta\s+name=["']csrf-token["']\s+content=["']([^"']+)["']`)
	match := re.FindStringSubmatch(body)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func CookieHeader(cookies []*http.Cookie) string {
	var parts []string
	for _, cookie := range cookies {
		if cookie == nil || strings.TrimSpace(cookie.Name) == "" {
			continue
		}
		parts = append(parts, cookie.Name+"="+cookie.Value)
	}
	return strings.Join(parts, "; ")
}
