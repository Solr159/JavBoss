package parseutil

import (
	"net/url"
)

func ResolveURL(baseURL, href string) string {
	if href == "" {
		return ""
	}
	reference, err := url.Parse(href)
	if err != nil {
		return ""
	}
	if reference.IsAbs() {
		return reference.String()
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	return base.ResolveReference(reference).String()
}
