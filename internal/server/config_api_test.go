package server

import "testing"

func TestNormalizeProxyHost(t *testing.T) {
	tests := []struct {
		name string
		host string
		want string
		ok   bool
	}{
		{name: "ipv4", host: "192.168.1.10", want: "192.168.1.10", ok: true},
		{name: "hostname", host: "proxy.local", want: "proxy.local", ok: true},
		{name: "bracketed ipv6", host: "[::1]", want: "::1", ok: true},
		{name: "url host", host: "http://10.0.0.2", want: "10.0.0.2", ok: true},
		{name: "host with port", host: "10.0.0.2:7890", ok: false},
		{name: "path", host: "10.0.0.2/proxy", ok: false},
		{name: "space", host: "bad host", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := normalizeProxyHost(tt.host)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("normalizeProxyHost(%q) = (%q, %v), want (%q, %v)", tt.host, got, ok, tt.want, tt.ok)
			}
		})
	}
}
