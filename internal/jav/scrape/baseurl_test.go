package scrape

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"javboss/internal/util"
)

func TestConfigurableBaseURLProviders(t *testing.T) {
	want := map[Provider]string{
		ProviderJavBus:      "javbus_base_url",
		ProviderJavDatabase: "javdatabase_base_url",
		ProviderJavDB:       "javdb_base_url",
		ProviderAvmoo:       "avmoo_base_url",
		ProviderAvsox:       "avsox_base_url",
		ProviderJavMenu:     "javmenu_base_url",
		ProviderJavModel:    "javmodel_base_url",
		ProviderMinnanoAV:   "minnanoav_base_url",
		ProviderAVWiki:      "avwiki_base_url",
	}
	providers := ConfigurableBaseURLProviders()
	if len(providers) != len(want) {
		t.Fatalf("providers = %v, want %d entries", providers, len(want))
	}
	for _, provider := range providers {
		if got := ProviderBaseURLConfigKey(provider); got != want[provider] {
			t.Fatalf("%s config key = %q, want %q", provider, got, want[provider])
		}
		if DefaultProviderBaseURL(provider) == "" {
			t.Fatalf("%s has no default base url", provider)
		}
	}
	for _, provider := range []Provider{ProviderJavDBAPI, ProviderThePornDB, ProviderUser, ProviderManualScrape, ProviderUnknown} {
		if key := ProviderBaseURLConfigKey(provider); key != "" {
			t.Fatalf("%s config key = %q, want empty", provider, key)
		}
		if _, err := NormalizeProviderBaseURL(provider, "https://mirror.test"); err == nil {
			t.Fatalf("%s accepted a custom base url", provider)
		}
	}
}

func TestNormalizeProviderBaseURL(t *testing.T) {
	got, err := NormalizeProviderBaseURL(ProviderJavBus, "  HTTPS://Mirror.JavBus.Test/ ")
	if err != nil || got != "https://mirror.javbus.test" {
		t.Fatalf("normalized = %q, err = %v", got, err)
	}
	if got, err := NormalizeProviderBaseURL(ProviderJavBus, "   "); err != nil || got != "" {
		t.Fatalf("blank normalized = %q, err = %v", got, err)
	}
	if _, err := NormalizeProviderBaseURL(ProviderJavBus, "https://mirror.javbus.test/genre"); err == nil {
		t.Fatal("path accepted")
	}
}

func TestSetProviderBaseURLAppliesAndResets(t *testing.T) {
	t.Cleanup(func() {
		_ = SetProviderBaseURL(ProviderJavBus, "")
		util.SetJavImageSources(nil)
	})
	defaultKey := defaultMetadataClient.lookupCacheKey(ProviderJavBus, "lookup_jav", "ABC-001")

	if err := SetProviderBaseURL(ProviderJavBus, "https://mirror.javbus.test/"); err != nil {
		t.Fatalf("SetProviderBaseURL: %v", err)
	}
	if got := ProviderBaseURL(ProviderJavBus); got != "https://mirror.javbus.test" {
		t.Fatalf("ProviderBaseURL = %q", got)
	}
	var listed AvailabilityProvider
	for _, provider := range AvailabilityProviders() {
		if provider.ID == ProviderJavBus {
			listed = provider
		}
	}
	if listed.BaseURL != "https://mirror.javbus.test" || listed.Domain != "mirror.javbus.test" {
		t.Fatalf("listed provider = %+v", listed)
	}
	if listed.DefaultBaseURL != DefaultProviderBaseURL(ProviderJavBus) {
		t.Fatalf("listed default base url = %q", listed.DefaultBaseURL)
	}
	if got := defaultMetadataClient.lookupCacheKey(ProviderJavBus, "lookup_jav", "ABC-001"); got == defaultKey || !strings.HasSuffix(got, ":https://mirror.javbus.test") {
		t.Fatalf("cache key %q does not encode the configured origin (default %q)", got, defaultKey)
	}

	if err := SetProviderBaseURL(ProviderJavBus, ""); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if got := ProviderBaseURL(ProviderJavBus); got != "" {
		t.Fatalf("ProviderBaseURL after reset = %q", got)
	}
	if got := defaultMetadataClient.lookupCacheKey(ProviderJavBus, "lookup_jav", "ABC-001"); got != defaultKey {
		t.Fatalf("cache key after reset = %q, want %q", got, defaultKey)
	}
	if err := SetProviderBaseURL(ProviderJavBus, "https://mirror.javbus.test/genre"); err == nil {
		t.Fatal("invalid base url accepted")
	}
}

func TestApplyProviderBaseURLConfig(t *testing.T) {
	t.Cleanup(func() {
		_ = SetProviderBaseURL(ProviderJavMenu, "")
		util.SetJavImageSources(nil)
	})
	ApplyProviderBaseURLConfig(map[string]string{
		"javmenu_base_url": "https://mirror.javmenu.test",
		"javbus_base_url":  "",
		"avmoo_base_url":   "https://mirror.avmoo.test",
	})
	if got := ProviderBaseURL(ProviderJavMenu); got != "https://mirror.javmenu.test" {
		t.Fatalf("javmenu base url = %q", got)
	}
	if got := ProviderBaseURL(ProviderAvmoo); got != "https://mirror.avmoo.test" {
		t.Fatalf("avmoo base url = %q", got)
	}
	if got := ProviderBaseURL(ProviderJavBus); got != "" {
		t.Fatalf("javbus base url = %q, want default", got)
	}
	_ = SetProviderBaseURL(ProviderAvmoo, "")
}

func TestCheckAvailabilityUsesConfiguredBaseURL(t *testing.T) {
	var requests atomic.Int32
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/wp-json/wp/v2/tags" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-WP-TotalPages", "1")
		description := `<div class="actress-data"><dl>
<dt>AV女優名：</dt><dd>三上悠亜（みかみゆあ）- Yua Mikami</dd>
<dt>生年月日：</dt><dd>1993年8月16日</dd>
<dt>サイズ：</dt><dd>T162 / B85 / W58 / H88 / Fカップ</dd>
</dl></div>`
		body, _ := json.Marshal([]map[string]string{{
			"name": "三上悠亜", "link": server.URL + "/av-actress/mikami-yua/", "description": description,
		}})
		_, _ = w.Write(body)
	}))
	defer server.Close()
	t.Cleanup(func() {
		_ = SetProviderBaseURL(ProviderAVWiki, "")
		util.SetJavImageSources(nil)
		InvalidateAvailabilityCache()
	})

	if err := SetProviderBaseURL(ProviderAVWiki, server.URL); err != nil {
		t.Fatalf("SetProviderBaseURL: %v", err)
	}
	result, err := CheckAvailability(context.Background(), ProviderAVWiki)
	if err != nil {
		t.Fatalf("CheckAvailability: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %q, want ok (requests=%d)", result.Status, requests.Load())
	}
	if requests.Load() == 0 {
		t.Fatal("availability check did not reach the configured origin")
	}
}
