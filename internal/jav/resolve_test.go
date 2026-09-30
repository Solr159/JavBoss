package jav

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestResolveJavByCodes(t *testing.T) {
	requestErr := errors.New("request failed")
	secondErr := errors.New("second request failed")
	type attempt struct {
		provider Provider
		code     string
	}
	first := attempt{ProviderJavMenu, "GANA-001"}
	second := attempt{ProviderJavBus, "GANA-001"}
	third := attempt{ProviderJavBus, "STARS-001"}
	fourth := attempt{ProviderAvmoo, "STARS-001"}
	fallback := attempt{ProviderAvsox, "123456_001"}
	last := attempt{ProviderAvsox, "123456_002"}
	all := []attempt{first, second, third, fourth, fallback, last}
	for _, tc := range []struct {
		name       string
		hit        attempt
		errors     map[attempt]error
		empty      attempt
		wantCalls  []attempt
		wantErrors []error
	}{
		{name: "first hit stops all lookups", hit: first, wantCalls: all[:1]},
		{name: "provider fallback", hit: second, wantCalls: all[:2]},
		{name: "candidate order before avsox", hit: fourth, wantCalls: all[:4]},
		{name: "avsox uses separate candidates", hit: last, wantCalls: all},
		{name: "request failure falls back", hit: second, errors: map[attempt]error{first: requestErr}, wantCalls: all[:2]},
		{name: "empty result falls back", empty: first, hit: second, wantCalls: all[:2]},
		{name: "all not found", wantCalls: all, wantErrors: []error{ErrNotFound}},
		{name: "request failures retained", errors: map[attempt]error{first: requestErr, fallback: secondErr}, wantCalls: all, wantErrors: []error{requestErr, secondErr}},
		{name: "provider timeout falls back", errors: map[attempt]error{first: context.DeadlineExceeded}, hit: second, wantCalls: all[:2]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls []attempt
			providers := make(map[Provider]any)
			want := &JavInfo{Code: tc.hit.code, Provider: tc.hit.provider, Title: "matched"}
			for _, provider := range []Provider{ProviderJavMenu, ProviderJavBus, ProviderAvmoo, ProviderAvsox} {
				providers[provider] = movieLookupFunc(func(ctx context.Context, code string) (*JavInfo, error) {
					call := attempt{provider, code}
					calls = append(calls, call)
					if err := tc.errors[call]; err != nil {
						return nil, err
					}
					if call == tc.hit {
						return want, nil
					}
					if call == tc.empty {
						return nil, nil
					}
					return nil, fmt.Errorf("lookup: %w", ErrNotFound)
				})
			}
			client := NewMetadataClient(providers, nil)
			info, err := client.ResolveJavByCodes(context.Background(), []string{"GANA-001", "STARS-001"}, []string{"123456_001", "123456_002"})
			if !reflect.DeepEqual(calls, tc.wantCalls) {
				t.Fatalf("calls = %v, want %v", calls, tc.wantCalls)
			}
			if len(tc.wantErrors) == 0 {
				if err != nil || info != want {
					t.Fatalf("result = %+v, %v; want first matching metadata", info, err)
				}
				return
			}
			if info != nil {
				t.Fatalf("unexpected result: %+v", info)
			}
			for _, wantErr := range tc.wantErrors {
				if !errors.Is(err, wantErr) {
					t.Fatalf("error = %v, want %v", err, wantErr)
				}
			}
			if tc.errors != nil && (errors.Is(err, ErrNotFound) || !strings.Contains(err.Error(), "provider=javmenu code=GANA-001")) {
				t.Fatalf("lookup failures must retain context and be distinct from not found: %v", err)
			}
		})
	}
}

func TestResolveJavByCodesEmptyCandidates(t *testing.T) {
	client := NewMetadataClient(map[Provider]any{}, nil)
	info, err := client.ResolveJavByCodes(context.Background(), nil, nil)
	if info != nil || !errors.Is(err, ErrNotFound) {
		t.Fatalf("result = %+v, %v; want not found", info, err)
	}
}

func TestResolveJavByCodesUncensoredCandidatesOnly(t *testing.T) {
	want := &JavInfo{Code: "123456_001", Provider: ProviderAvsox}
	client := NewMetadataClient(map[Provider]any{
		ProviderAvsox: movieLookupFunc(func(_ context.Context, code string) (*JavInfo, error) {
			if code != want.Code {
				t.Fatalf("code = %q, want %q", code, want.Code)
			}
			return want, nil
		}),
	}, nil)
	info, err := client.ResolveJavByCodes(context.Background(), nil, []string{want.Code})
	if err != nil || info != want {
		t.Fatalf("result = %+v, %v; want uncensored metadata", info, err)
	}
}

func TestResolveJavByCodesCancellation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		before    bool
		deadline  bool
		returnHit bool
	}{
		{name: "already cancelled", before: true},
		{name: "expired deadline", before: true, deadline: true},
		{name: "cancelled during lookup"},
		{name: "cancelled despite returned hit", returnHit: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			wantErr := context.Canceled
			if tc.deadline {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 0)
				wantErr = context.DeadlineExceeded
			}
			defer cancel()
			if tc.before {
				cancel()
			}
			calls := 0
			client := NewMetadataClient(map[Provider]any{
				ProviderJavMenu: movieLookupFunc(func(got context.Context, _ string) (*JavInfo, error) {
					calls++
					if got != ctx {
						t.Fatal("context was not preserved")
					}
					cancel()
					if tc.returnHit {
						return &JavInfo{Code: "GANA-001"}, nil
					}
					return nil, got.Err()
				}),
				ProviderJavBus: movieLookupFunc(func(context.Context, string) (*JavInfo, error) {
					t.Fatal("fallback called after cancellation")
					return nil, nil
				}),
			}, nil)
			info, err := client.ResolveJavByCodes(ctx, []string{"GANA-001"}, nil)
			if info != nil || !errors.Is(err, wantErr) {
				t.Fatalf("result = %+v, %v; want %v", info, err, wantErr)
			}
			wantCalls := 1
			if tc.before {
				wantCalls = 0
			}
			if calls != wantCalls {
				t.Fatalf("lookup calls = %d, want %d", calls, wantCalls)
			}
		})
	}
}

func TestProvidersForCode(t *testing.T) {
	tests := []struct {
		name string
		code string
		want []Provider
	}{
		{
			name: "gana uses javmenu then javbus",
			code: "gana-1234",
			want: []Provider{ProviderJavMenu, ProviderJavBus},
		},
		{
			name: "stars uses javbus then avmoo",
			code: " STARS-001 ",
			want: []Provider{ProviderJavBus, ProviderAvmoo},
		},
		{
			name: "ap uses avmoo",
			code: "ap-001",
			want: []Provider{ProviderAvmoo},
		},
		{
			name: "other uses javbus",
			code: "IPX-228",
			want: []Provider{ProviderJavBus},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := providersForCode(tt.code)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("providersForCode(%q) = %#v, want %#v", tt.code, got, tt.want)
			}
		})
	}
}
