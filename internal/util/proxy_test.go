package util

import (
	"net/http"
	"net/url"
	"testing"
)

func TestSetProxyFromStringsUsesConfiguredHost(t *testing.T) {
	t.Cleanup(func() {
		SetProxyPort(0)
	})

	SetProxyFromStrings("192.168.1.10", "7890")

	u, err := DetectProxyFunc()(&http.Request{})
	if err != nil {
		t.Fatalf("DetectProxyFunc returned error: %v", err)
	}
	if u == nil {
		t.Fatal("DetectProxyFunc returned nil proxy")
	}
	if got, want := u.String(), "http://192.168.1.10:7890"; got != want {
		t.Fatalf("proxy URL = %q, want %q", got, want)
	}
}

func TestSetProxyFromStringsDefaultsHostForPortOnlyConfig(t *testing.T) {
	t.Cleanup(func() {
		SetProxyPort(0)
	})

	SetProxyFromStrings("", "7890")

	u, err := DetectProxyFunc()(&http.Request{})
	if err != nil {
		t.Fatalf("DetectProxyFunc returned error: %v", err)
	}
	if u == nil {
		t.Fatal("DetectProxyFunc returned nil proxy")
	}
	if got, want := u.String(), "http://127.0.0.1:7890"; got != want {
		t.Fatalf("proxy URL = %q, want %q", got, want)
	}
}

func TestMapProxyURLForContainerMapsLoopbackHosts(t *testing.T) {
	t.Setenv("JAVBOSS_PROXY_HOST_GATEWAY", "1")

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "ipv4", raw: "http://127.0.0.1:7890", want: "http://host.docker.internal:7890"},
		{name: "localhost", raw: "http://localhost:7890", want: "http://host.docker.internal:7890"},
		{name: "ipv6", raw: "http://[::1]:7890", want: "http://host.docker.internal:7890"},
		{name: "remote", raw: "http://192.168.1.10:7890", want: "http://192.168.1.10:7890"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.raw)
			if err != nil {
				t.Fatalf("parse proxy URL: %v", err)
			}
			got := mapProxyURLForContainer(u)
			if got.String() != tt.want {
				t.Fatalf("mapped proxy URL = %q, want %q", got.String(), tt.want)
			}
		})
	}
}

func TestMapProxyURLForContainerDoesNotMapForLocalDockerMode(t *testing.T) {
	t.Setenv("JAVBOSS_CONTAINER", "1")

	u, err := url.Parse("http://127.0.0.1:7890")
	if err != nil {
		t.Fatalf("parse proxy URL: %v", err)
	}
	got := mapProxyURLForContainer(u)
	if got.String() != "http://127.0.0.1:7890" {
		t.Fatalf("mapped proxy URL = %q, want loopback unchanged", got.String())
	}
}
