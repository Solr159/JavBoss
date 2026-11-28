package util_test

import (
	"net/http"
	"strings"
	"testing"

	"javboss/internal/util"
)

func TestSetJavImageRequestHeaders(t *testing.T) {
	for _, tc := range []struct {
		host, referer string
		cookie        bool
	}{
		{"javbus.com", "https://www.javbus.com/", true},
		{"www.javbus.com", "https://www.javbus.com/", true},
		{"IMG.JAVBUS.COM", "https://www.javbus.com/", true},
		{"dmm.co.jp", "https://www.dmm.co.jp/", false},
		{"awsimgsrc.dmm.co.jp", "https://www.dmm.co.jp/", false},
		{"pics.dmm.co.jp", "https://www.dmm.co.jp/", false},
		{"tp.spfcas.com", "", false},
		{"notjavbus.com", "", false},
		{"javbus.com.example.org", "", false},
		{"notdmm.co.jp", "", false},
	} {
		t.Run(tc.host, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "https://"+tc.host+"/image.jpg", nil)
			if err != nil {
				t.Fatal(err)
			}
			util.SetJavImageRequestHeaders(req)
			if got := req.Header.Get("Referer"); got != tc.referer {
				t.Fatalf("Referer = %q, want %q", got, tc.referer)
			}
			if got := strings.Contains(req.Header.Get("Cookie"), "age=verified"); got != tc.cookie {
				t.Fatalf("age cookie = %v, want %v", got, tc.cookie)
			}
			if req.Header.Get("User-Agent") == "" || !strings.Contains(req.Header.Get("Accept"), "image/") {
				t.Fatal("missing image request headers")
			}
		})
	}
}

func TestSetJavImageSources(t *testing.T) {
	util.SetJavImageSources([]util.JavImageSource{
		{Origin: "https://mirror.javbus.test/", Cookie: "age=verified; existmag=mag"},
		{Origin: "https://mirror.avmoo.test"},
		{Origin: "https://www.javbus.com", Cookie: "ignored=1"},
		{Origin: "not a url"},
	})
	t.Cleanup(func() { util.SetJavImageSources(nil) })

	for _, tc := range []struct {
		name, host, referer, cookie string
	}{
		{name: "configured origin", host: "mirror.javbus.test", referer: "https://mirror.javbus.test/", cookie: "age=verified"},
		{name: "configured subdomain", host: "img.mirror.avmoo.test", referer: "https://mirror.avmoo.test/"},
		{name: "official host keeps built-in rules", host: "www.javbus.com", referer: "https://www.javbus.com/", cookie: "age=verified"},
		{name: "unrelated host", host: "cdn.example.test"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "https://"+tc.host+"/image.jpg", nil)
			if err != nil {
				t.Fatal(err)
			}
			util.SetJavImageRequestHeaders(req)
			if got := req.Header.Get("Referer"); got != tc.referer {
				t.Fatalf("Referer = %q, want %q", got, tc.referer)
			}
			if got := strings.Contains(req.Header.Get("Cookie"), tc.cookie); tc.cookie != "" && !got {
				t.Fatalf("Cookie = %q, want %q", req.Header.Get("Cookie"), tc.cookie)
			}
			if tc.cookie == "" && strings.Contains(req.Header.Get("Cookie"), "ignored=1") {
				t.Fatalf("official javbus origin must not be registered: %q", req.Header.Get("Cookie"))
			}
		})
	}
}

func TestImageHeadersDistinguishMirrorOrigins(t *testing.T) {
	util.SetJavImageSources([]util.JavImageSource{
		{Origin: "https://mirror.test:8443", Cookie: "age=verified"},
		{Origin: "https://mirror.test:9443"},
		{Origin: "https://mirror.test", Cookie: "age=verified"},
		{Origin: "https://img.mirror.test"},
		{Origin: "https://www.javbus.com:8443"},
	})
	t.Cleanup(func() { util.SetJavImageSources(nil) })

	for _, tc := range []struct{ origin, referer, cookie string }{
		{"https://mirror.test:8443", "https://mirror.test:8443/", "age=verified"},
		{"https://mirror.test:9443", "https://mirror.test:9443/", ""},
		{"https://mirror.test:4443", "", ""},
		{"http://mirror.test:8443", "", ""},
		{"https://mirror.test:443", "https://mirror.test/", "age=verified"},
		{"https://img.mirror.test", "https://img.mirror.test/", ""},
		{"https://cdn.mirror.test:9443", "https://mirror.test:9443/", ""},
		{"https://www.javbus.com:8443", "https://www.javbus.com:8443/", ""},
	} {
		t.Run(tc.origin, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, tc.origin+"/cover.jpg", nil)
			if err != nil {
				t.Fatal(err)
			}
			util.SetJavImageRequestHeaders(req)
			if got := req.Header.Get("Referer"); got != tc.referer {
				t.Errorf("Referer = %q, want %q", got, tc.referer)
			}
			if got := req.Header.Get("Cookie"); got != tc.cookie {
				t.Errorf("Cookie = %q, want %q", got, tc.cookie)
			}
		})
	}
}
