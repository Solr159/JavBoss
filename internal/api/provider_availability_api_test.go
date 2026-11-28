package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"javboss/internal/jav/scrape"
	"javboss/internal/util"

	"github.com/gin-gonic/gin"
)

func TestProviderAvailabilityRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router)
	list := httptest.NewRecorder()
	router.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/jav/providers", nil))
	var providers []scrape.AvailabilityProvider
	if err := json.Unmarshal(list.Body.Bytes(), &providers); err != nil || list.Code != http.StatusOK || len(providers) != 11 {
		t.Fatalf("provider list: status=%d body=%s err=%v", list.Code, list.Body, err)
	}
	for _, provider := range providers {
		if provider.Domain == "" || provider.Sample == "" {
			t.Fatalf("provider %s is missing its domain", provider.Name)
		}
		if provider.ID == scrape.ProviderJavDBAPI && provider.Domain != "jdforrepam.com" {
			t.Fatalf("JavDB API must show its API domain, got %s", provider.Domain)
		}
		configurable := provider.ID != scrape.ProviderJavDBAPI && provider.ID != scrape.ProviderThePornDB
		if provider.Configurable != configurable {
			t.Fatalf("provider %s configurable = %v, want %v", provider.Name, provider.Configurable, configurable)
		}
		if configurable != (provider.DefaultBaseURL != "") {
			t.Fatalf("provider %s default base url = %q", provider.Name, provider.DefaultBaseURL)
		}
	}
	for _, id := range []string{"0", "3", "11", "999", "invalid"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/jav/providers/"+id+"/availability", nil))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("invalid provider %s: status=%d", id, recorder.Code)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/jav/providers/1/availability", nil).WithContext(ctx))
	var result scrape.AvailabilityResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil || recorder.Code != http.StatusOK || result.Status != "canceled" {
		t.Fatalf("canceled probe: status=%d body=%s err=%v", recorder.Code, recorder.Body, err)
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("availability responses must not be cached")
	}
}

func TestProviderAvailabilityRequiresAuthentication(t *testing.T) {
	deps := newTestDependencies()
	router := NewRouter("", testAuthService(deps, t))
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/jav/providers"},
		{http.MethodPut, "/jav/providers/1/base_url"},
		{http.MethodPost, "/jav/providers/1/availability"},
		{http.MethodPost, "/jav/providers/1/connectivity"},
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(route.method, route.path, nil))
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: status=%d", route.method, route.path, recorder.Code)
		}
	}
}

func TestProviderAvailabilityCacheSurvivesListingAndClearsOnProxySave(t *testing.T) {
	deps := newTestDependencies()
	testAuthService(deps, t)
	scrape.InvalidateAvailabilityCache()
	t.Cleanup(scrape.InvalidateAvailabilityCache)
	t.Setenv("JAVBOSS_PROXY_HOST_GATEWAY", "0")
	t.Cleanup(func() { util.SetProxyPort(0) })
	var requests atomic.Int32
	// A local proxy failure gives a deterministic completed check without
	// replacing a global HTTP client or contacting the external provider.
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests.Add(1)
		if req.Method != http.MethodConnect || req.Host != "av-wiki.net:443" {
			t.Errorf("unexpected proxy request: %s %s", req.Method, req.Host)
		}
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer proxy.Close()
	host, portText, _ := net.SplitHostPort(strings.TrimPrefix(proxy.URL, "http://"))
	port, _ := strconv.Atoi(portText)
	util.SetProxy(host, port)

	router := gin.New()
	RegisterRoutes(router)
	request := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s %s: status=%d body=%s", method, path, recorder.Code, recorder.Body)
		}
		return recorder
	}
	request(http.MethodPost, "/jav/providers/13/availability", "")
	for _, step := range []struct {
		config string
		cached bool
	}{
		{"", true},
		{`{"video_waterfall_default":true}`, true},
		{`{"proxy_mode":"direct"}`, false},
	} {
		if step.config != "" {
			request(http.MethodPatch, "/config", step.config)
		}
		response := request(http.MethodGet, "/jav/providers", "")
		var providers []scrape.AvailabilityProvider
		if err := json.Unmarshal(response.Body.Bytes(), &providers); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, provider := range providers {
			if provider.ID != scrape.ProviderAVWiki {
				continue
			}
			found = true
			if (provider.LastResult != nil) != step.cached {
				t.Fatalf("config=%s cached=%+v", step.config, provider.LastResult)
			}
			if provider.LastResult != nil && (provider.LastResult.CheckedAt.IsZero() || provider.LastResult.Status != "network_error") {
				t.Fatal("cached results must include the check time")
			}
		}
		if !found || requests.Load() != 1 {
			t.Fatalf("found=%v outbound requests=%d", found, requests.Load())
		}
	}
}

func TestUpdateProviderBaseURL(t *testing.T) {
	deps := newTestDependencies()
	auth := testAuthService(deps, t)
	router := NewRouter("", auth)
	t.Cleanup(func() { _ = scrape.SetProviderBaseURL(scrape.ProviderJavBus, "") })

	login := performRequest(router, http.MethodPost, "/auth/login", []byte(`{"password":"admin"}`), "http://example.com", nil)
	if login.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", login.Code, login.Body)
	}
	cookie := responseCookie(t, login, authSessionCookie)

	update := func(provider, body string) *httptest.ResponseRecorder {
		t.Helper()
		return performRequest(router, http.MethodPut, "/jav/providers/"+provider+"/base_url", []byte(body), "http://example.com", cookie)
	}
	listProviders := func() []scrape.AvailabilityProvider {
		t.Helper()
		response := performRequest(router, http.MethodGet, "/jav/providers", nil, "http://example.com", cookie)
		var providers []scrape.AvailabilityProvider
		if err := json.Unmarshal(response.Body.Bytes(), &providers); err != nil || response.Code != http.StatusOK {
			t.Fatalf("list providers: status=%d body=%s err=%v", response.Code, response.Body, err)
		}
		return providers
	}
	javbus := func(providers []scrape.AvailabilityProvider) scrape.AvailabilityProvider {
		for _, provider := range providers {
			if provider.ID == scrape.ProviderJavBus {
				return provider
			}
		}
		t.Fatal("javbus missing from provider list")
		return scrape.AvailabilityProvider{}
	}
	readConfig := func() map[string]string {
		t.Helper()
		response := performRequest(router, http.MethodGet, "/config", nil, "http://example.com", cookie)
		var cfg map[string]string
		if err := json.Unmarshal(response.Body.Bytes(), &cfg); err != nil || response.Code != http.StatusOK {
			t.Fatalf("read config: status=%d body=%s err=%v", response.Code, response.Body, err)
		}
		return cfg
	}

	if got := javbus(listProviders()); got.BaseURL != "" || got.DefaultBaseURL != "https://www.javbus.com" || got.Domain != "www.javbus.com" {
		t.Fatalf("default provider entry = %+v", got)
	}

	saved := update("1", `{"base_url":"https://Mirror.JavBus.Test/"}`)
	if saved.Code != http.StatusOK {
		t.Fatalf("save status=%d body=%s", saved.Code, saved.Body)
	}
	var payload providerBaseURLResponse
	if err := json.Unmarshal(saved.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.BaseURL != "https://mirror.javbus.test" || payload.DefaultBaseURL != "https://www.javbus.com" || payload.Domain != "mirror.javbus.test" {
		t.Fatalf("response = %+v", payload)
	}
	if got := javbus(listProviders()); got.BaseURL != "https://mirror.javbus.test" || got.Domain != "mirror.javbus.test" {
		t.Fatalf("configured provider entry = %+v", got)
	}
	if got := readConfig()["javbus_base_url"]; got != "https://mirror.javbus.test" {
		t.Fatalf("stored javbus_base_url = %q", got)
	}

	for _, tc := range []struct{ name, provider, body string }{
		{name: "path", provider: "1", body: `{"base_url":"https://mirror.javbus.test/genre"}`},
		{name: "credentials", provider: "1", body: `{"base_url":"https://user:pass@mirror.javbus.test"}`},
		{name: "scheme", provider: "1", body: `{"base_url":"ftp://mirror.javbus.test"}`},
		{name: "port without hostname", provider: "1", body: `{"base_url":"https://:8443"}`},
		{name: "missing field", provider: "1", body: `{}`},
		{name: "api provider", provider: "12", body: `{"base_url":"https://mirror.test"}`},
		{name: "unknown provider", provider: "99", body: `{"base_url":"https://mirror.test"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := update(tc.provider, tc.body); got.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", got.Code, got.Body)
			}
		})
	}

	if got := javbus(listProviders()); got.BaseURL != "https://mirror.javbus.test" {
		t.Fatalf("invalid save changed provider: %+v", got)
	}

	scoped := update("1", `{"base_url":"http://[FE80::1%25Eth0]:8080/"}`)
	if scoped.Code != http.StatusOK {
		t.Fatalf("scoped IPv6 save: status=%d body=%s", scoped.Code, scoped.Body)
	}
	const scopedOrigin = "http://[fe80::1%25Eth0]:8080"
	if got := javbus(listProviders()); got.BaseURL != scopedOrigin || got.Domain != "fe80::1%Eth0" {
		t.Fatalf("scoped IPv6 provider entry = %+v", got)
	}
	if got := readConfig()["javbus_base_url"]; got != scopedOrigin {
		t.Fatalf("stored scoped IPv6 origin = %q", got)
	}

	start := make(chan struct{})
	responses := make(chan *httptest.ResponseRecorder, 2)
	for _, body := range []string{`{"base_url":"https://first.test"}`, `{"base_url":"https://second.test"}`} {
		go func() {
			<-start
			responses <- update("1", body)
		}()
	}
	close(start)
	for range 2 {
		if response := <-responses; response.Code != http.StatusOK {
			t.Errorf("concurrent save status=%d body=%s", response.Code, response.Body)
		}
	}
	if stored, runtime := readConfig()["javbus_base_url"], javbus(listProviders()).BaseURL; runtime != stored {
		t.Fatalf("runtime base url=%q stored=%q", runtime, stored)
	}

	reset := update("1", `{"base_url":"https://WWW.JAVBUS.COM/"}`)
	if reset.Code != http.StatusOK {
		t.Fatalf("reset status=%d body=%s", reset.Code, reset.Body)
	}
	var resetPayload providerBaseURLResponse
	if err := json.Unmarshal(reset.Body.Bytes(), &resetPayload); err != nil {
		t.Fatalf("decode reset response: %v", err)
	}
	if resetPayload.BaseURL != "" || resetPayload.Domain != "www.javbus.com" {
		t.Fatalf("reset response = %+v", resetPayload)
	}
	if got := javbus(listProviders()); got.BaseURL != "" || got.Domain != "www.javbus.com" {
		t.Fatalf("provider entry after reset = %+v", got)
	}
	if got, ok := readConfig()["javbus_base_url"]; !ok || got != "" {
		t.Fatalf("stored default override = %q, present=%v", got, ok)
	}
}
