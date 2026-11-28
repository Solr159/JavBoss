// Package baseurl validates and normalizes the configurable site origin used by
// scraper providers.
package baseurl

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/idna"
)

var (
	domainProfile = idna.New(idna.MapForLookup(), idna.BidiRule(), idna.VerifyDNSLength(true))
	ipv4Literal   = regexp.MustCompile(`^(?:[0-9]+|0x[0-9a-f]*)(?:\.(?:[0-9]+|0x[0-9a-f]*)){0,3}$`)
)

// Normalize accepts only an HTTPS domain without an explicit port. It removes an
// optional trailing slash and normalizes the domain for DNS lookup.
func Normalize(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" ||
		parsed.Host != parsed.Hostname() || parsed.User != nil ||
		(parsed.Path != "" && parsed.Path != "/") || parsed.ForceQuery ||
		parsed.RawQuery != "" || strings.Contains(raw, "#") {
		return "", fmt.Errorf("site url must use https://domain without a port, credentials, path, query, or fragment")
	}
	host, err := domainProfile.ToASCII(parsed.Host)
	host = strings.TrimSuffix(host, ".")
	if err != nil || host == "" || ipv4Literal.MatchString(host) {
		return "", fmt.Errorf("site url must contain a valid domain name")
	}
	return "https://" + host, nil
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
