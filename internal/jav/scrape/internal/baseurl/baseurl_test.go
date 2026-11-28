package baseurl

import "testing"

func TestNormalize(t *testing.T) {
	for _, tc := range []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "official", raw: "https://www.javbus.com", want: "https://www.javbus.com"},
		{name: "trims and lowercases", raw: "  HTTPS://JavBus.COM/ ", want: "https://javbus.com"},
		{name: "keeps port", raw: "http://127.0.0.1:8080", want: "http://127.0.0.1:8080"},
		{name: "keeps ipv6", raw: "http://[::1]:8080", want: "http://[::1]:8080"},
		{name: "empty", raw: "   ", wantErr: true},
		{name: "port without hostname", raw: "https://:8443", wantErr: true},
		{name: "no scheme", raw: "javbus.com", wantErr: true},
		{name: "unsupported scheme", raw: "ftp://javbus.com", wantErr: true},
		{name: "credentials", raw: "https://user:pass@javbus.com", wantErr: true},
		{name: "path", raw: "https://javbus.com/mirror", wantErr: true},
		{name: "query", raw: "https://javbus.com/?a=1", wantErr: true},
		{name: "fragment", raw: "https://javbus.com/#x", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Normalize(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Normalize(%q) = %q, want error", tc.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Normalize(%q) error: %v", tc.raw, err)
			}
			if got != tc.want {
				t.Fatalf("Normalize(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestNormalizeOrDefault(t *testing.T) {
	if got := NormalizeOrDefault("  "); got != "" {
		t.Fatalf("NormalizeOrDefault(empty) = %q, want empty", got)
	}
	if got := NormalizeOrDefault("not a url"); got != "" {
		t.Fatalf("NormalizeOrDefault(invalid) = %q, want empty", got)
	}
	if got := NormalizeOrDefault("https://avsox.click/"); got != "https://avsox.click" {
		t.Fatalf("NormalizeOrDefault(valid) = %q", got)
	}
}

func TestHost(t *testing.T) {
	if got := Host("https://www.JavBus.com"); got != "www.javbus.com" {
		t.Fatalf("Host() = %q", got)
	}
	if got := Host("not a url"); got != "" {
		t.Fatalf("Host(invalid) = %q, want empty", got)
	}
}
