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
