package util

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"
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

func TestResolveProxyMode(t *testing.T) {
	for _, tt := range []struct{ mode, port, want string }{
		{"", "", ProxyModeAuto},
		{"", "0", ProxyModeAuto},
		{"", "7890", ProxyModeManual},
		{"", "65536", ProxyModeAuto},
		{"auto", "7890", ProxyModeAuto},
		{"direct", "7890", ProxyModeDirect},
		{"manual", "7890", ProxyModeManual},
		{" DIRECT ", "", ProxyModeDirect},
		{"invalid", "7890", ProxyModeAuto},
	} {
		if got := ResolveProxyMode(tt.mode, tt.port); got != tt.want {
			t.Errorf("ResolveProxyMode(%q, %q)=%q want=%q", tt.mode, tt.port, got, tt.want)
		}
	}
}

func TestProxyModesApplyToExistingClient(t *testing.T) {
	t.Setenv("JAVBOSS_PROXY_HOST_GATEWAY", "0")
	t.Cleanup(func() { SetProxyPort(0) })
	server := func(label string) *httptest.Server {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, label)
		}))
		t.Cleanup(srv.Close)
		return srv
	}
	origin, automatic, manual := server("direct"), server("auto"), server("manual")
	autoURL, _ := url.Parse(automatic.URL)
	manualURL, _ := url.Parse(manual.URL)
	manualPort, _ := strconv.Atoi(manualURL.Port())
	fallbackCalls := 0
	client := NewHTTPClientWithTransport(time.Second, func(transport *http.Transport) {
		transport.Proxy = configuredProxy(func(*http.Request) (*url.URL, error) {
			fallbackCalls++
			return autoURL, nil
		})
	})
	defer client.CloseIdleConnections()
	for _, mode := range []string{ProxyModeAuto, ProxyModeDirect, ProxyModeManual, ProxyModeAuto, ProxyModeDirect} {
		SetProxySettings(mode, manualURL.Hostname(), strconv.Itoa(manualPort))
		before := fallbackCalls
		response, err := client.Get(origin.URL)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || string(body) != mode {
			t.Fatalf("mode=%s route=%q err=%v", mode, body, err)
		}
		if mode != ProxyModeAuto && fallbackCalls != before {
			t.Fatalf("%s unexpectedly consulted the environment/system proxy", mode)
		}
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
