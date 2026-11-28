package scrape

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

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
	if err := SetProviderBaseURL(ProviderJavBus, ""); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if got := ProviderBaseURL(ProviderJavBus); got != "" {
		t.Fatalf("ProviderBaseURL after reset = %q", got)
	}
	if err := SetProviderBaseURL(ProviderJavBus, "https://mirror.javbus.test/genre"); err == nil {
		t.Fatal("invalid base url accepted")
	}
}

func TestAVWikiLookupsUseDefaultOrigin(t *testing.T) {
	for _, reset := range []bool{false, true} {
		httpClient := &http.Client{Transport: testRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Host != "av-wiki.net" || req.URL.Scheme != "https" {
				t.Errorf("request URL = %s, want AV Wiki default", req.URL)
			}
			body := `[{"name":"女優名","link":"https://av-wiki.net/av-actress/test/","description":"<dl class=actress-data><dt>AV女優名</dt><dd>女優名</dd></dl>"}]`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
		})}
		implementation, err := newProvider(ProviderAVWiki, httpClient)
		if err != nil {
			t.Fatal(err)
		}
		if reset {
			setter := implementation.(baseURLProvider)
			setter.SetBaseURL("https://mirror.avwiki.test")
			setter.SetBaseURL("")
		}
		result, err := implementation.(ActressNameLookup).LookupActressByName(context.Background(), "女優名")
		if err != nil || result == nil || result.ProfileURL != "https://av-wiki.net/av-actress/test/" {
			t.Fatalf("reset=%v: lookup=%+v, err=%v", reset, result, err)
		}
	}
}

type lookupCacheWithReadHook struct {
	*memoryLookupCache
	onRead func()
}

func (c lookupCacheWithReadHook) Get(key string, now time.Time) ([]byte, bool, error) {
	c.onRead()
	return c.memoryLookupCache.Get(key, now)
}

func TestLookupCacheUsesOriginOfCapturedProvider(t *testing.T) {
	const (
		oldMirror = "https://old.avwiki.test"
		newMirror = "https://new.avwiki.test"
		name      = "女優名"
	)
	provider := ProviderAVWiki
	oldProvider, err := newProvider(provider, &http.Client{Transport: testRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Scheme != "https" || req.URL.Host != "old.avwiki.test" {
			t.Errorf("lookup sent to %s, want %s", req.URL, oldMirror)
		}
		return mockAVWikiProfile(oldMirror, name), nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	oldProvider.(baseURLProvider).SetBaseURL(oldMirror)
	c := defaultMetadataClient
	c.providersMu.Lock()
	c.providers[provider] = oldProvider
	c.providersMu.Unlock()
	t.Cleanup(func() {
		_ = SetProviderBaseURL(provider, "")
		util.SetJavImageSources(nil)
	})

	cache := newMemoryLookupCache()
	oldCache := c.currentLookupCache()
	t.Cleanup(func() { SetCache(oldCache) })
	SetCache(lookupCacheWithReadHook{memoryLookupCache: cache, onRead: func() {
		// Switch after looking up the key and before the upstream request starts.
		if err := SetProviderBaseURL(provider, newMirror); err != nil {
			t.Fatal(err)
		}
		SetCache(cache)
	}})

	result, err := LookupActressByJapaneseName(context.Background(), name, provider)
	if err != nil || result == nil || result.ProfileURL != oldMirror+"/av-actress/test/" {
		t.Fatalf("lookup=%+v, err=%v", result, err)
	}
	if got := ProviderBaseURL(provider); got != newMirror {
		t.Fatalf("active provider origin = %q, want %q", got, newMirror)
	}
	oldKey := lookupCacheKey(provider, "lookup_actress_name", name, oldMirror)
	if got, ok, err := lookupCacheGet[ActressInfo](c, oldKey); err != nil || !ok || got.ProfileURL != result.ProfileURL {
		t.Fatalf("old cache: profile=%+v, hit=%v, err=%v", got, ok, err)
	}
	newKey := lookupCacheKey(provider, "lookup_actress_name", name, newMirror)
	if _, ok, _ := lookupCacheGet[ActressInfo](c, newKey); ok {
		t.Fatal("old lookup result was cached under the new origin")
	}
}

func TestMirrorSessionFollowsLookupOrigin(t *testing.T) {
	for _, provider := range []Provider{ProviderAvmoo, ProviderAvsox} {
		t.Run(provider.String(), func(t *testing.T) {
			const oldOrigin = "https://old.test"
			const newOrigin = "https://new.test"
			var scraper baseURLProvider
			var requests []string
			httpClient := &http.Client{Transport: testRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				origin := req.URL.Scheme + "://" + req.URL.Host
				requests = append(requests, req.Method+" "+origin)
				token := req.URL.Hostname()
				header := make(http.Header)
				body := `{"code":200,"data":[]}`
				if req.Method == http.MethodGet {
					header.Set("Set-Cookie", "session="+token)
					body = `<meta name="csrf-token" content="` + token + `">`
					if origin == oldOrigin {
						// Emulate a save while the first session request is in flight.
						scraper.SetBaseURL(newOrigin)
					}
				} else if req.Header.Get("Cookie") != "session="+token ||
					req.Header.Get("X-CSRF-Token") != token || req.Header.Get("Origin") != origin {
					t.Errorf("POST to %s used headers from another origin", origin)
				}
				return &http.Response{StatusCode: http.StatusOK, Header: header, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			implementation, err := newProvider(provider, httpClient)
			if err != nil {
				t.Fatal(err)
			}
			scraper = implementation.(baseURLProvider)
			scraper.SetBaseURL(oldOrigin)

			for range 2 {
				_, err := implementation.(MovieLookup).LookupJavByCode(context.Background(), "ABC-001")
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("empty result error = %v, want not found", err)
				}
			}
			want := []string{"GET " + oldOrigin, "POST " + oldOrigin, "GET " + newOrigin, "POST " + newOrigin}
			if !slices.Equal(requests, want) {
				t.Fatalf("requests = %v, want %v", requests, want)
			}
		})
	}
}

func TestChangingOriginInvalidatesOnlyThatProvidersAvailability(t *testing.T) {
	c := defaultMetadataClient
	InvalidateAvailabilityCache()
	t.Cleanup(InvalidateAvailabilityCache)
	t.Cleanup(func() {
		_ = SetProviderBaseURL(ProviderJavBus, "")
		util.SetJavImageSources(nil)
	})

	now := time.Now()
	other := AvailabilityResult{Provider: ProviderAVWiki, Status: "ok", CheckedAt: now}
	c.cacheAvailabilityResult(other, c.beginAvailabilityCheck(other.Provider))
	edited := AvailabilityResult{Provider: ProviderJavBus, Status: "ok", CheckedAt: now}
	c.cacheAvailabilityResult(edited, c.beginAvailabilityCheck(edited.Provider))
	pending := c.beginAvailabilityCheck(edited.Provider)

	if err := SetProviderBaseURL(edited.Provider, "https://mirror.javbus.test"); err != nil {
		t.Fatal(err)
	}
	c.cacheAvailabilityResult(edited, pending)
	if got := c.cachedAvailabilityResult(edited.Provider); got != nil {
		t.Fatalf("edited provider kept a stale result: %+v", got)
	}
	if got := c.cachedAvailabilityResult(other.Provider); got == nil || *got != other {
		t.Fatalf("other provider lost its result: %+v", got)
	}
}

func TestApplyProviderBaseURLConfig(t *testing.T) {
	t.Cleanup(func() {
		_ = SetProviderBaseURL(ProviderJavMenu, "")
		util.SetJavImageSources(nil)
	})
	ApplyProviderBaseURLConfig(map[string]string{
		"javmenu_base_url": "https://mirror.javmenu.test",
		"javbus_base_url":  "http://mirror.javbus.test:8080",
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
	const mirror = "https://mirror.avwiki.test"
	c := defaultMetadataClient
	originalFactory := c.availabilityFactory
	t.Cleanup(func() {
		c.availabilityFactory = originalFactory
		_ = SetProviderBaseURL(ProviderAVWiki, "")
		util.SetJavImageSources(nil)
		InvalidateAvailabilityCache()
	})

	requests := 0
	c.availabilityFactory = func(provider Provider, client *http.Client) (any, error) {
		client.Transport = testRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests++
			if req.URL.Scheme != "https" || req.URL.Host != "mirror.avwiki.test" || req.URL.Path != "/wp-json/wp/v2/tags" {
				t.Errorf("unexpected availability request: %s", req.URL)
			}
			return mockAVWikiProfile(mirror, "三上悠亜"), nil
		})
		return newProvider(provider, client)
	}

	if err := SetProviderBaseURL(ProviderAVWiki, mirror); err != nil {
		t.Fatalf("SetProviderBaseURL: %v", err)
	}
	result, err := CheckAvailability(context.Background(), ProviderAVWiki)
	if err != nil {
		t.Fatalf("CheckAvailability: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %q, want ok (requests=%d)", result.Status, requests)
	}
	if requests != 1 {
		t.Fatal("availability check did not reach the configured origin")
	}
}

func mockAVWikiProfile(origin, name string) *http.Response {
	description := `<dl class="actress-data"><dt>AV女優名</dt><dd>` + name + `</dd><dt>サイズ</dt><dd>T160</dd></dl>`
	body, _ := json.Marshal([]map[string]string{{
		"name": name, "link": origin + "/av-actress/test/", "description": description,
	}})
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(body)))}
}
