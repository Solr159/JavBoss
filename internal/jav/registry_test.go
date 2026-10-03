package jav

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"javboss/internal/util"
)

func TestProvidersUseInjectedHTTPClient(t *testing.T) {
	for _, provider := range NewMetadataClient(nil, nil).AvailabilityProviders() {
		t.Run(provider.Name, func(t *testing.T) {
			calls := 0
			injectedErr := errors.New("injected transport")
			httpClient := &http.Client{Transport: testRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				return nil, injectedErr
			})}
			implementation, err := newProvider(provider.ID, httpClient)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			switch p := implementation.(type) {
			case MovieLookup:
				_, err = p.LookupJavByCode(ctx, provider.Sample)
			case ActressNameLookup:
				_, err = p.LookupActressByName(ctx, provider.Sample)
			default:
				t.Fatal("provider has no supported lookup")
			}
			if calls == 0 || err == nil {
				t.Fatalf("lookup did not use injected failing transport: calls=%d err=%v", calls, err)
			}
		})
	}
}

func TestAllProviderClientsCacheFailedURLsButProbesBypass(t *testing.T) {
	for _, provider := range NewMetadataClient(nil, nil).AvailabilityProviders() {
		for _, status := range []int{http.StatusForbidden, http.StatusNotFound} {
			t.Run(fmt.Sprintf("%s/%d", provider.Name, status), func(t *testing.T) {
				var calls atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls.Add(1)
					w.WriteHeader(status)
				}))
				defer server.Close()
				// A unique path also isolates repeated test runs from the process-wide cache.
				target := fmt.Sprintf("%s/%s/%d", server.URL, t.Name(), time.Now().UnixNano())
				client := newCachedProviderHTTPClient(provider.ID)
				defer client.CloseIdleConnections()
				resp, err := client.Get(target)
				if err != nil {
					t.Fatal(err)
				}
				resp.Body.Close()
				if resp.StatusCode != status {
					t.Fatalf("status=%d want=%d", resp.StatusCode, status)
				}
				resp, err = client.Get(target)
				if err != nil {
					t.Fatal(err)
				}
				resp.Body.Close()
				if resp.StatusCode != status {
					t.Fatalf("cached status=%d want=%d", resp.StatusCode, status)
				}
				if calls.Load() != 1 {
					t.Fatalf("cached lookup made %d requests", calls.Load())
				}
				probe := newProviderHTTPClient(provider.ID)
				defer probe.CloseIdleConnections()
				resp, err = probe.Get(target)
				if err != nil {
					t.Fatal(err)
				}
				resp.Body.Close()
				if resp.StatusCode != status || calls.Load() != 2 {
					t.Fatalf("probe did not reach network: status=%d calls=%d", resp.StatusCode, calls.Load())
				}
			})
		}
	}
}

func TestProviderHTTPClientsAreFresh(t *testing.T) {
	for _, provider := range NewMetadataClient(nil, nil).AvailabilityProviders() {
		t.Run(provider.Name, func(t *testing.T) {
			first, second := newProviderHTTPClient(provider.ID), newProviderHTTPClient(provider.ID)
			defer first.CloseIdleConnections()
			defer second.CloseIdleConnections()
			if first == second || first.Transport == nil || first.Transport == second.Transport {
				t.Fatal("provider client factory reused a client or transport")
			}
			wantTimeout := 10 * time.Second
			switch provider.ID {
			case ProviderJavDB:
				wantTimeout = 15 * time.Second
			case ProviderJavDBAPI:
				wantTimeout = 20 * time.Second
			case ProviderAvmoo, ProviderAvsox:
				wantTimeout = 30 * time.Second
			}
			if first.Timeout != wantTimeout {
				t.Fatalf("timeout=%s want=%s", first.Timeout, wantTimeout)
			}
		})
	}
}

func TestNewlyCachedProvidersPreserveLookupErrors(t *testing.T) {
	for _, provider := range []Provider{ProviderAVWiki, ProviderJavDB, ProviderJavDBAPI, ProviderAvmoo, ProviderAvsox} {
		for _, status := range []int{http.StatusForbidden, http.StatusNotFound} {
			t.Run(fmt.Sprintf("%s/%d", provider, status), func(t *testing.T) {
				calls := 0
				httpClient := util.WithNegativeCache(&http.Client{Transport: testRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					calls++
					return &http.Response{
						StatusCode: status, Status: fmt.Sprintf("%d %s", status, http.StatusText(status)),
						Header: make(http.Header), Body: io.NopCloser(strings.NewReader("")), Request: req,
					}, nil
				})})
				implementation, err := newProvider(provider, httpClient)
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				input := fmt.Sprintf("CACHE-%d-%d-%d", provider, status, time.Now().UnixNano())
				var firstError string
				for attempt := 0; attempt < 2; attempt++ {
					switch p := implementation.(type) {
					case MovieLookup:
						_, err = p.LookupJavByCode(ctx, input)
					case ActressNameLookup:
						_, err = p.LookupActressByName(ctx, input)
					default:
						t.Fatal("provider has no supported lookup")
					}
					// AVWiki treats an unavailable tags API as an error, not a missing actress.
					wantNotFound := status == http.StatusNotFound && provider != ProviderAVWiki
					if err == nil || errors.Is(err, ErrNotFound) != wantNotFound {
						t.Fatalf("attempt=%d err=%v wantNotFound=%v", attempt, err, wantNotFound)
					}
					if attempt == 0 {
						firstError = err.Error()
					} else if err.Error() != firstError {
						t.Fatalf("cached error=%v differs from network error=%s", err, firstError)
					}
				}
				if calls != 1 {
					t.Fatalf("lookup made %d requests, want 1", calls)
				}
			})
		}
	}
}
