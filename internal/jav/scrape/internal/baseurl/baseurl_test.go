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
		{name: "international domain", raw: "https://例子.测试/", want: "https://xn--fsqu00a.xn--0zwm56d"},
		{name: "trailing dns root dot", raw: "https://mirror.test.", want: "https://mirror.test"},
		{name: "empty", raw: "   ", wantErr: true},
		{name: "nonstandard port", raw: "https://mirror.test:8443", wantErr: true},
		{name: "explicit default port", raw: "https://mirror.test:443", wantErr: true},
		{name: "empty explicit port", raw: "https://mirror.test:", wantErr: true},
		{name: "ipv4", raw: "https://127.0.0.1", wantErr: true},
		{name: "short ipv4", raw: "https://127.1", wantErr: true},
		{name: "numeric ipv4", raw: "https://0x7f000001", wantErr: true},
		{name: "ipv6", raw: "https://[::1]", wantErr: true},
		{name: "scoped ipv6", raw: "https://[fe80::1%25eth0]", wantErr: true},
		{name: "invalid domain", raw: "https://invalid_domain.test", wantErr: true},
		{name: "no scheme", raw: "javbus.com", wantErr: true},
		{name: "http", raw: "http://javbus.com", wantErr: true},
		{name: "unsupported scheme", raw: "ftp://javbus.com", wantErr: true},
		{name: "credentials", raw: "https://user:pass@javbus.com", wantErr: true},
		{name: "path", raw: "https://javbus.com/mirror", wantErr: true},
		{name: "query", raw: "https://javbus.com/?a=1", wantErr: true},
		{name: "empty query", raw: "https://javbus.com/?", wantErr: true},
		{name: "fragment", raw: "https://javbus.com/#x", wantErr: true},
		{name: "empty fragment", raw: "https://javbus.com/#", wantErr: true},
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
			if again, err := Normalize(got); err != nil || again != got {
				t.Fatalf("Normalize(%q) = %q, error %v, want the same origin", got, again, err)
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
