package jav

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"javboss/internal/jav/javdbapi"
	"javboss/internal/util"
)

type availabilityMovie struct {
	lookup func(context.Context, string) (*JavInfo, error)
}

func (*availabilityMovie) OriginURL() string { return "https://provider.invalid" }
func (p *availabilityMovie) LookupJavByCode(ctx context.Context, code string) (*JavInfo, error) {
	return p.lookup(ctx, code)
}
func availabilityTestClient(lookup func(context.Context, string) (*JavInfo, error)) *MetadataClient {
	c := NewMetadataClient(map[Provider]any{ProviderJavBus: &availabilityMovie{}}, newMemoryLookupCache())
	c.availabilityFactory = func(_ Provider, httpClient *http.Client) (any, error) { return &availabilityMovie{lookup: lookup}, nil }
	return c
}
func sampleMovie() *JavInfo { return &JavInfo{Code: "SSIS-001", Title: "A real title"} }

type availabilityActress struct{ info *ActressInfo }

func (p *availabilityActress) LookupActressByName(_ context.Context, name string) (*ActressInfo, error) {
	if name != "三上悠亜" {
		return nil, fmt.Errorf("wrong sample: %s", name)
	}
	return p.info, nil
}

func TestAvailabilityRequiresMatchingActressProfile(t *testing.T) {
	for _, tc := range []struct {
		name string
		info *ActressInfo
		want string
	}{
		{"profile", &ActressInfo{JapaneseName: "三上悠亜", HeightCM: 159}, "ok"},
		{"empty", nil, "invalid_response"},
		{"name only", &ActressInfo{JapaneseName: "三上悠亜"}, "invalid_response"},
		{"wrong actress", &ActressInfo{JapaneseName: "別人", HeightCM: 159}, "invalid_response"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := NewMetadataClient(map[Provider]any{ProviderAVWiki: &availabilityActress{}}, nil)
			client.availabilityFactory = func(_ Provider, httpClient *http.Client) (any, error) {
				return &availabilityActress{info: tc.info}, nil
			}
			result, err := client.CheckAvailability(context.Background(), ProviderAVWiki)
			if err != nil || result.Status != tc.want {
				t.Fatalf("result=%+v err=%v", result, err)
			}
		})
	}
}

func TestAvailabilityRequiresParsedData(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		want       string
	}{
		{"success", `{"Code":"SSIS-001","Title":"A real title"}`, 200, "ok"},
		{"empty HTTP 200", `{}`, 200, "invalid_response"},
		{"challenge HTTP 200", `<html>Just a moment</html>`, 200, "invalid_response"},
		{"wrong movie", `{"Code":"SSIS-002","Title":"Wrong movie"}`, 200, "invalid_response"},
		{"code only", `{"Code":"SSIS-001","Title":"SSIS-001"}`, 200, "invalid_response"},
		{"forbidden", ``, 403, "http_error"},
		{"rate limited", ``, 429, "http_error"},
		{"missing", ``, 404, "http_error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/SSIS-001" {
					t.Errorf("not a real lookup: %s", r.URL)
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			client := availabilityTestClient(nil)
			client.availabilityFactory = func(_ Provider, httpClient *http.Client) (any, error) {
				return &availabilityMovie{lookup: func(ctx context.Context, code string) (*JavInfo, error) {
					if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 30*time.Second {
						t.Error("unbounded lookup")
					}
					req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/"+code, nil)
					resp, err := httpClient.Do(req)
					if err != nil {
						return nil, err
					}
					defer resp.Body.Close()
					if resp.StatusCode != 200 {
						return nil, ErrNotFound
					}
					var info JavInfo
					err = json.NewDecoder(resp.Body).Decode(&info)
					return &info, err
				}}, nil
			}
			got, err := client.CheckAvailability(context.Background(), ProviderJavBus)
			if err != nil || got.Status != tc.want || got.HTTPStatus != tc.status {
				t.Fatalf("result=%+v err=%v", got, err)
			}
		})
	}
}

func TestAvailabilityErrorClassification(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{context.Canceled, "canceled"}, {context.DeadlineExceeded, "timeout"},
		{&net.DNSError{Err: "lookup failed"}, "dns_error"},
		{&tls.CertificateVerificationError{Err: errors.New("bad cert")}, "tls_error"},
		{&url.Error{Op: "Get", URL: "https://private.invalid", Err: errors.New("proxy credentials")}, "network_error"},
		{ErrNotFound, "not_found"}, {errors.New("parse failure with private data"), "invalid_response"},
	} {
		c := availabilityTestClient(func(context.Context, string) (*JavInfo, error) { return nil, tc.err })
		result, err := c.CheckAvailability(context.Background(), ProviderJavBus)
		if err != nil || result.Status != tc.want {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	}
}

func TestAvailabilityUsesFreshClientsAndBypassesLookupCache(t *testing.T) {
	c := availabilityTestClient(nil)
	lookupCacheSetHit(c, lookupCacheKey(ProviderJavBus, "lookup_jav", "SSIS-001"), sampleMovie())
	var instances []*availabilityMovie
	var httpClients []*http.Client
	c.availabilityFactory = func(_ Provider, httpClient *http.Client) (any, error) {
		p := &availabilityMovie{lookup: func(context.Context, string) (*JavInfo, error) { return nil, ErrNotFound }}
		instances = append(instances, p)
		httpClients = append(httpClients, httpClient)
		return p, nil
	}
	for range 2 {
		result, err := c.CheckAvailability(context.Background(), ProviderJavBus)
		if err != nil || result.Status != "not_found" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	}
	if len(instances) != 2 || instances[0] == instances[1] {
		t.Fatal("provider reused between checks")
	}
	if httpClients[0] == httpClients[1] || httpClients[0].Transport == httpClients[1].Transport {
		t.Fatal("HTTP client or transport reused between checks")
	}
	cached, ok, err := lookupCacheGet[JavInfo](c, lookupCacheKey(ProviderJavBus, "lookup_jav", "SSIS-001"))
	if !ok || err != nil || cached.Title != "A real title" {
		t.Fatal("check changed normal lookup cache")
	}
}

func TestAvailabilityProvidersCoverRegistry(t *testing.T) {
	c := NewMetadataClient(nil, nil)
	providers := c.AvailabilityProviders()
	if len(providers) != 11 || len(providers) != len(c.providers) {
		t.Fatalf("providers=%+v", providers)
	}
	for i, p := range providers {
		if p.Domain == "" || p.Sample == "" || (i > 0 && p.ID <= providers[i-1].ID) {
			t.Fatalf("provider=%+v", p)
		}
	}
	for _, id := range []Provider{ProviderUnknown, ProviderUser, ProviderManualScrape, Provider(999)} {
		if _, err := c.CheckAvailability(context.Background(), id); !errors.Is(err, ErrUnsupportedProvider) {
			t.Fatalf("id=%d err=%v", id, err)
		}
	}
}

func TestAvailabilitySignedAPIFollowsSearchAndDetailUsingCurrentProxy(t *testing.T) {
	t.Setenv("JAVBOSS_PROXY_HOST_GATEWAY", "0")
	t.Cleanup(func() { util.SetProxyPort(0) })
	c := NewMetadataClient(map[Provider]any{ProviderJavDBAPI: javdbapi.New(newProviderHTTPClient(ProviderJavDBAPI), "http://provider.invalid")}, nil)
	c.availabilityFactory = func(_ Provider, httpClient *http.Client) (any, error) {
		return javdbapi.New(httpClient, "http://provider.invalid"), nil
	}
	var identities []string
	for _, failDetail := range []bool{true, false} {
		calls := 0
		device := ""
		proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			if r.URL.Host != "provider.invalid" || r.Header.Get("jdsignature") == "" {
				t.Error("missing signed API request via proxy")
			}
			if device == "" {
				device = r.URL.Query().Get("device_uuid")
			}
			if device == "" || device != r.URL.Query().Get("device_uuid") {
				t.Error("unstable identity within lookup")
			}
			switch r.URL.Path {
			case "/api/v2/search":
				fmt.Fprint(w, `{"success":1,"data":{"movies":[{"id":"m1","number":"SSIS-001"}]}}`)
			case "/api/v4/movies/m1":
				if failDetail {
					w.WriteHeader(403)
					return
				}
				fmt.Fprint(w, `{"success":1,"data":{"movie":{"number":"SSIS-001","title":"Title"}}}`)
			default:
				t.Errorf("unexpected path %s", r.URL.Path)
			}
		}))
		defer proxy.Close()
		host, portText, _ := net.SplitHostPort(strings.TrimPrefix(proxy.URL, "http://"))
		port, _ := strconv.Atoi(portText)
		util.SetProxy(host, port)
		result, err := c.CheckAvailability(context.Background(), ProviderJavDBAPI)
		want := "ok"
		if failDetail {
			want = "http_error"
		}
		if err != nil || calls != 2 || result.Status != want {
			t.Fatalf("result=%+v calls=%d err=%v", result, calls, err)
		}
		identities = append(identities, device)
	}
	if identities[0] == identities[1] {
		t.Fatal("checks reused device identity")
	}
}

func TestAvailabilityCacheRefreshCancellationAndInvalidation(t *testing.T) {
	calls := 0
	c := availabilityTestClient(func(context.Context, string) (*JavInfo, error) { calls++; return sampleMovie(), nil })
	if c.AvailabilityProviders()[0].LastResult != nil || calls != 0 {
		t.Fatal("listing performed lookup")
	}
	for range 2 {
		result, err := c.CheckAvailability(context.Background(), ProviderJavBus)
		if err != nil {
			t.Fatal(err)
		}
		cached := c.AvailabilityProviders()[0].LastResult
		if cached == nil || *cached != result {
			t.Fatalf("cache=%+v result=%+v", cached, result)
		}
		cached.Status = "mutated"
	}
	if calls != 2 {
		t.Fatal("manual checks did not refresh")
	}
	previous := *c.AvailabilityProviders()[0].LastResult
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, _ := c.CheckAvailability(ctx, ProviderJavBus)
	if result.Status != "canceled" || *c.AvailabilityProviders()[0].LastResult != previous || calls != 2 {
		t.Fatal("cancellation changed result or made a request")
	}
	c.invalidateAvailabilityCache()
	if c.AvailabilityProviders()[0].LastResult != nil {
		t.Fatal("cache not invalidated")
	}
}

func TestAvailabilityCacheDiscardsStaleInFlightResults(t *testing.T) {
	for _, mode := range []string{"newer check", "invalidate", "invalidate and new check"} {
		t.Run(mode, func(t *testing.T) {
			started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var calls atomic.Int32
			c := availabilityTestClient(func(context.Context, string) (*JavInfo, error) {
				if calls.Add(1) == 1 {
					close(started)
					<-release
					return nil, ErrNotFound
				}
				return sampleMovie(), nil
			})
			go func() { defer close(done); _, _ = c.CheckAvailability(context.Background(), ProviderJavBus) }()
			<-started
			if mode != "newer check" {
				c.invalidateAvailabilityCache()
			}
			if mode != "invalidate" {
				_, _ = c.CheckAvailability(context.Background(), ProviderJavBus)
			}
			close(release)
			<-done
			cached := c.AvailabilityProviders()[0].LastResult
			if mode == "invalidate" {
				if cached != nil {
					t.Fatal("stale result restored cache")
				}
			} else if cached == nil || cached.Status != "ok" {
				t.Fatalf("stale result replaced latest: %+v", cached)
			}
		})
	}
}
