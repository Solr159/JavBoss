package jav

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
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
