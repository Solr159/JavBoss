package util

import (
	"net/http"
	"strings"
)

// SetJavImageRequestHeaders applies image-source headers using the request host.
func SetJavImageRequestHeaders(req *http.Request) {
	if req == nil || req.URL == nil {
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/*,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	host := strings.ToLower(req.URL.Hostname())
	switch {
	case host == "javbus.com" || strings.HasSuffix(host, ".javbus.com"):
		req.Header.Set("Referer", "https://www.javbus.com/")
		req.Header.Set("Cookie", "age=verified; existmag=mag")
	case host == "dmm.co.jp" || strings.HasSuffix(host, ".dmm.co.jp"):
		req.Header.Set("Referer", "https://www.dmm.co.jp/")
	}
}
