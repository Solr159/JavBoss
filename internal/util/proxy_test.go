package util

import (
	"net/http"
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
