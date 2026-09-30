package jav

import (
	"context"
	"errors"
	"testing"
)

type movieLookupFunc func(context.Context, string) (*JavInfo, error)

func (f movieLookupFunc) LookupJavByCode(ctx context.Context, code string) (*JavInfo, error) {
	return f(ctx, code)
}

func TestProviderCapabilities(t *testing.T) {
	client := NewMetadataClient(nil, nil)
	for _, tc := range []struct {
		provider                                                          Provider
		movie, actressCode, actressName, actressURL, seriesURL, studioURL bool
	}{
		{ProviderJavBus, true, false, false, false, false, false},
		{ProviderJavDatabase, true, true, false, false, false, false},
		{ProviderJavDB, true, false, false, true, true, true},
		{ProviderJavDBAPI, true, false, false, true, true, true},
		{ProviderAvmoo, true, false, false, false, false, false},
		{ProviderAvsox, true, false, false, false, false, false},
		{ProviderJavMenu, true, false, false, false, false, false},
		{ProviderThePornDB, true, false, false, false, false, false},
		{ProviderJavModel, false, false, true, false, false, false},
		{ProviderMinnanoAV, false, false, true, false, false, false},
		{ProviderAVWiki, false, false, true, false, false, false},
	} {
		t.Run(tc.provider.String(), func(t *testing.T) {
			capabilities := client.CapabilitiesFor(tc.provider)
			got := [6]bool{capabilities.Movie, capabilities.ActressByCode, capabilities.ActressByName, capabilities.ActressURL, capabilities.SeriesURL, capabilities.StudioURL}
			want := [6]bool{tc.movie, tc.actressCode, tc.actressName, tc.actressURL, tc.seriesURL, tc.studioURL}
			if got != want {
				t.Fatalf("capabilities = %v, want %v", got, want)
			}
		})
	}
}

func TestUnsupportedLookupErrors(t *testing.T) {
	client := NewMetadataClient(nil, nil)
	ctx := context.Background()
	for _, tc := range []struct {
		name   string
		lookup func() error
		want   error
	}{
		{"unknown provider", func() error { _, err := client.LookupJavByCode(ctx, "ABC-001", Provider(100)); return err }, ErrUnsupportedProvider},
		{"manual source", func() error { _, err := client.LookupJavByCode(ctx, "ABC-001", ProviderManualScrape); return err }, ErrUnsupportedProvider},
		{"movie", func() error { _, err := client.LookupJavByCode(ctx, "ABC-001", ProviderJavModel); return err }, ErrUnsupportedOperation},
		{"actress code", func() error { _, err := client.LookupActressByCode(ctx, "ABC-001", ProviderAvmoo); return err }, ErrUnsupportedOperation},
		{"actress name", func() error { _, err := client.LookupActressByJapaneseName(ctx, "name", ProviderJavBus); return err }, ErrUnsupportedOperation},
		{"actress url", func() error {
			_, err := client.LookupActressURLByCodeAndName(ctx, "ABC-001", "name", ProviderJavDatabase)
			return err
		}, ErrUnsupportedOperation},
		{"series url", func() error { _, err := client.LookupSeriesURLByCode(ctx, "ABC-001", ProviderAvsox); return err }, ErrUnsupportedOperation},
		{"studio url", func() error { _, err := client.LookupStudioURLByCode(ctx, "ABC-001", ProviderJavMenu); return err }, ErrUnsupportedOperation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.lookup(); !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestLookupPreservesContext(t *testing.T) {
	type key struct{}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), key{}, "request"))
	defer cancel()
	called := make(chan struct{})
	provider := movieLookupFunc(func(got context.Context, code string) (*JavInfo, error) {
		if got.Value(key{}) != "request" || code != "ABC-001" {
			t.Error("request context or code lost")
		}
		close(called)
		<-got.Done()
		return nil, got.Err()
	})
	client := NewMetadataClient(map[Provider]any{ProviderJavBus: provider}, nil)
	done := make(chan error, 1)
	go func() { _, err := client.LookupJavByCode(ctx, "ABC-001", ProviderJavBus); done <- err }()
	<-called
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}

func TestCancelledLookupDoesNotReadCacheOrCallProvider(t *testing.T) {
	provider := &countingLookupProvider{javInfo: &JavInfo{Code: "ABC-001"}}
	client := NewMetadataClient(map[Provider]any{ProviderJavBus: provider}, newMemoryLookupCache())
	lookupCacheSetHit(client, lookupCacheKey(ProviderJavBus, "lookup_jav", "ABC-001"), provider.javInfo)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.LookupJavByCode(ctx, "ABC-001", ProviderJavBus); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	if provider.javCalls != 0 {
		t.Fatal("called provider after cancellation")
	}
}

func TestClientRegistryAndCacheAreIndependent(t *testing.T) {
	first := &countingLookupProvider{javInfo: &JavInfo{Title: "first"}}
	second := &countingLookupProvider{javInfo: &JavInfo{Title: "second"}}
	registry := map[Provider]any{ProviderJavBus: first}
	one := NewMetadataClient(registry, newMemoryLookupCache())
	registry[ProviderJavBus] = second
	two := NewMetadataClient(registry, newMemoryLookupCache())
	for _, tc := range []struct {
		client *MetadataClient
		title  string
	}{{one, "first"}, {two, "second"}, {one, "first"}} {
		info, err := tc.client.LookupJavByCode(context.Background(), "ABC-001", ProviderJavBus)
		if err != nil || info.Title != tc.title {
			t.Fatalf("info = %+v, error = %v", info, err)
		}
	}
	if first.javCalls != 1 || second.javCalls != 1 {
		t.Fatal("independent client caches did not retain their own results")
	}
}

func TestProviderPanicIsNotMisreportedAsUnsupported(t *testing.T) {
	provider := movieLookupFunc(func(context.Context, string) (*JavInfo, error) { panic("parser bug") })
	client := NewMetadataClient(map[Provider]any{ProviderJavBus: provider}, nil)
	defer func() {
		if got := recover(); got != "parser bug" {
			t.Fatalf("panic = %v", got)
		}
	}()
	_, _ = client.LookupJavByCode(context.Background(), "ABC-001", ProviderJavBus)
}
