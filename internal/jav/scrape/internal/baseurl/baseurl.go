// Package baseurl validates and normalizes the configurable site origin used by
// scraper providers.
package baseurl

import (
	"fmt"
	"net/url"
	"strings"
)

// Normalize validates a configured site origin. Only an origin is accepted so
// providers keep control of their own paths, queries, and fragments. Trailing
// slashes are ignored and the host is lowercased.
func Normalize(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("site url is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil || parsed.Host == "" {
		return "", fmt.Errorf("site url must be a valid origin")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("site url must use http or https")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("site url must not contain credentials")
	}
	if strings.TrimRight(parsed.Path, "/") != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("site url must not contain a path, query, or fragment")
	}
	return scheme + "://" + strings.ToLower(parsed.Host), nil
}

// NormalizeOrDefault returns the normalized origin, or an empty string when raw
// is empty or invalid. Callers treat the empty string as "use the site default".
func NormalizeOrDefault(raw string) string {
	normalized, err := Normalize(raw)
	if err != nil {
		return ""
	}
	return normalized
}

// Host returns the hostname of a normalized origin, or an empty string when the
// origin cannot be parsed.
func Host(origin string) string {
	parsed, err := url.Parse(strings.TrimSpace(origin))
	if err != nil || parsed == nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}
