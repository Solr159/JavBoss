package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"javboss/internal/jav"
	"javboss/internal/util"
)

func TestProviderConnectivityRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router)
	list := httptest.NewRecorder()
	router.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/jav/providers", nil))
	var providers []jav.ConnectivityProvider
	if err := json.Unmarshal(list.Body.Bytes(), &providers); err != nil || list.Code != http.StatusOK || len(providers) != 10 {
		t.Fatalf("provider list: status=%d body=%s err=%v", list.Code, list.Body, err)
	}
	for _, provider := range providers {
		if provider.Domain == "" {
			t.Fatalf("provider %s is missing its domain", provider.Name)
		}
		if provider.ID == jav.ProviderJavDBAPI && provider.Domain != "jdforrepam.com" {
			t.Fatalf("JavDB API must show its API domain, got %s", provider.Domain)
		}
	}
	for _, id := range []string{"0", "3", "11", "999", "invalid"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/jav/providers/"+id+"/connectivity", nil))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("invalid provider %s: status=%d", id, recorder.Code)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/jav/providers/1/connectivity", nil).WithContext(ctx))
	var result jav.ConnectivityResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil || recorder.Code != http.StatusOK || result.Status != "canceled" {
		t.Fatalf("canceled probe: status=%d body=%s err=%v", recorder.Code, recorder.Body, err)
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("connectivity responses must not be cached")
	}
}

func TestProviderConnectivityRequiresAuthentication(t *testing.T) {
	router := NewRouter("", testAuthService(t))
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/jav/providers"},
		{http.MethodPost, "/jav/providers/1/connectivity"},
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(route.method, route.path, nil))
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: status=%d", route.method, route.path, recorder.Code)
		}
	}
}

type connectivityTransportFunc func(*http.Request) (*http.Response, error)

func (f connectivityTransportFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestProviderConnectivityCacheSurvivesListingAndClearsOnProxySave(t *testing.T) {
	testAuthService(t)
	jav.InvalidateConnectivityCache()
	t.Cleanup(jav.InvalidateConnectivityCache)
	client := util.DefaultHTTPClient()
	previous := client.Transport
	t.Cleanup(func() { client.Transport = previous; util.SetProxyPort(0) })
	requests := 0
	client.Transport = connectivityTransportFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Request: req}, nil
	})
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
	request(http.MethodPost, "/jav/providers/7/connectivity", "")
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
		var providers []jav.ConnectivityProvider
		if err := json.Unmarshal(response.Body.Bytes(), &providers); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, provider := range providers {
			if provider.ID != jav.ProviderJavModel {
				continue
			}
			found = true
			if (provider.LastResult != nil) != step.cached {
				t.Fatalf("config=%s cached=%+v", step.config, provider.LastResult)
			}
			if provider.LastResult != nil && provider.LastResult.CheckedAt.IsZero() {
				t.Fatal("cached results must include the check time")
			}
		}
		if !found || requests != 1 {
			t.Fatalf("found=%v outbound requests=%d", found, requests)
		}
	}
}
