package jav

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLookupJavByCodeUsesCache(t *testing.T) {
	cache := newMemoryLookupCache()

	provider := &countingLookupProvider{
		javInfo: &JavInfo{Code: "ABC-001", Title: "Cached Title", Provider: ProviderJavBus},
	}
	client := NewMetadataClient(map[Provider]any{ProviderJavBus: provider}, cache)

	first, err := client.LookupJavByCode(context.Background(), "abc-001", ProviderJavBus)
	if err != nil {
		t.Fatalf("first lookup: %v", err)
	}
	second, err := client.LookupJavByCode(context.Background(), "ABC-001", ProviderJavBus)
	if err != nil {
		t.Fatalf("second lookup: %v", err)
	}
	if provider.javCalls != 1 {
		t.Fatalf("unexpected provider calls: got %d want 1", provider.javCalls)
	}
	if first == nil || second == nil || second.Title != first.Title {
		t.Fatalf("unexpected cached result: first=%#v second=%#v", first, second)
	}
}

func TestLookupJavByCodeCachesNotFound(t *testing.T) {
	cache := newMemoryLookupCache()

	provider := &countingLookupProvider{err: ErrNotFound}
	client := NewMetadataClient(map[Provider]any{ProviderJavBus: provider}, cache)

	for i := 0; i < 2; i++ {
		_, err := client.LookupJavByCode(context.Background(), "MISS-001", ProviderJavBus)
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("lookup %d err=%v want ErrNotFound", i, err)
		}
	}
	if provider.javCalls != 1 {
		t.Fatalf("unexpected provider calls: got %d want 1", provider.javCalls)
	}
}

func TestLookupJavByCodeDoesNotCacheTemporaryErrors(t *testing.T) {
	cache := newMemoryLookupCache()

	provider := &countingLookupProvider{err: errors.New("temporary")}
	client := NewMetadataClient(map[Provider]any{ProviderJavBus: provider}, cache)

	for i := 0; i < 2; i++ {
		_, err := client.LookupJavByCode(context.Background(), "TMP-001", ProviderJavBus)
		if err == nil {
			t.Fatalf("lookup %d expected error", i)
		}
	}
	if provider.javCalls != 2 {
		t.Fatalf("unexpected provider calls: got %d want 2", provider.javCalls)
	}
}

type actressNameLookupFunc func(context.Context, string) (*ActressInfo, error)

func (f actressNameLookupFunc) LookupActressByName(ctx context.Context, name string) (*ActressInfo, error) {
	return f(ctx, name)
}

func TestAVWikiActressLookupCache(t *testing.T) {
	for _, tc := range []struct {
		name      string
		err       error
		wantCalls int
	}{
		{"profile", nil, 1},
		{"missing actress", ErrNotFound, 1},
		{"unavailable API", errors.New("avwiki: http 403"), 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			provider := actressNameLookupFunc(func(_ context.Context, name string) (*ActressInfo, error) {
				calls++
				if tc.err != nil {
					return nil, tc.err
				}
				return &ActressInfo{JapaneseName: name, HeightCM: 160, ProfileURL: "https://av-wiki.net/av-actress/test/"}, nil
			})
			client := NewMetadataClient(map[Provider]any{ProviderAVWiki: provider}, newMemoryLookupCache())
			for range 2 {
				info, err := client.LookupActressByJapaneseName(context.Background(), "女優名", ProviderAVWiki)
				if tc.err == nil {
					if err != nil || info == nil || info.HeightCM != 160 || info.JapaneseName != "女優名" {
						t.Fatalf("info=%+v error=%v", info, err)
					}
				} else if !errors.Is(err, tc.err) {
					t.Fatalf("error=%v, want %v", err, tc.err)
				}
			}
			if calls != tc.wantCalls {
				t.Fatalf("calls=%d, want %d", calls, tc.wantCalls)
			}
		})
	}
}

func TestLookupCacheKeyVersionIsProviderSpecific(t *testing.T) {
	cases := []struct {
		name     string
		provider Provider
		method   string
		input    string
		want     string
	}{
		{
			name:     "javbus lookup jav uses provider version",
			provider: ProviderJavBus,
			method:   "lookup_jav",
			input:    "abc-001",
			want:     "v8:jav:javbus:lookup_jav:ABC-001",
		},
		{
			name:     "javdatabase lookup jav uses provider version",
			provider: ProviderJavDatabase,
			method:   "lookup_jav",
			input:    "abc-001",
			want:     "v4:jav:javdatabase:lookup_jav:ABC-001",
		},
		{
			name:     "javdb lookup jav uses provider version",
			provider: ProviderJavDB,
			method:   "lookup_jav",
			input:    "abc-001",
			want:     "v4:jav:javdb:lookup_jav:ABC-001",
		},
		{
			name:     "javdb-api lookup jav uses strict number version",
			provider: ProviderJavDBAPI,
			method:   "lookup_jav",
			input:    "abc-001",
			want:     "v5:jav:javdb-api:lookup_jav:ABC-001",
		},
		{
			name:     "javdb-api studio link uses strict number version",
			provider: ProviderJavDBAPI,
			method:   "lookup_studio_url",
			input:    "053026_001",
			want:     "v2:jav:javdb-api:lookup_studio_url:053026_001",
		},
		{
			name:     "javdb-api series link uses strict number version",
			provider: ProviderJavDBAPI,
			method:   "lookup_series_url",
			input:    "053026_001",
			want:     "v2:jav:javdb-api:lookup_series_url:053026_001",
		},
		{
			name:     "javdb-api actress link uses strict number version",
			provider: ProviderJavDBAPI,
			method:   "lookup_actress_url_code_name",
			input:    "053026_001|Actress",
			want:     "v2:jav:javdb-api:lookup_actress_url_code_name:053026_001|Actress",
		},
		{
			name:     "avmoo lookup jav uses provider version",
			provider: ProviderAvmoo,
			method:   "lookup_jav",
			input:    "abc-001",
			want:     "v6:jav:avmoo:lookup_jav:ABC-001",
		},
		{
			name:     "avsox lookup jav uses provider version",
			provider: ProviderAvsox,
			method:   "lookup_jav",
			input:    "030919_047",
			want:     "v3:jav:avsox:lookup_jav:030919_047",
		},
		{
			name:     "javmenu lookup jav uses provider version",
			provider: ProviderJavMenu,
			method:   "lookup_jav",
			input:    "abc-001",
			want:     "v2:jav:javmenu:lookup_jav:ABC-001",
		},
		{
			name:     "javdb actress url uses fallback version",
			provider: ProviderJavDB,
			method:   "lookup_actress_url_code_name",
			input:    "HEYZO-1867|美月アンジェリア",
			want:     "v3:jav:javdb:lookup_actress_url_code_name:HEYZO-1867|美月アンジェリア",
		},
		{
			name:     "javdatabase actress lookup uses fixed parser version",
			provider: ProviderJavDatabase,
			method:   "lookup_actress_code",
			input:    "ipx-228",
			want:     "v2:jav:javdatabase:lookup_actress_code:IPX-228",
		},
		{
			name:     "minnanoav actress name lookup uses qualified result parser version",
			provider: ProviderMinnanoAV,
			method:   "lookup_actress_name",
			input:    "倉沢裕美",
			want:     "v3:jav:minnanoav:lookup_actress_name:倉沢裕美",
		},
		{
			name:     "avwiki actress lookup uses normalized roman name version",
			provider: ProviderAVWiki,
			method:   "lookup_actress_name",
			input:    "九井スナオ",
			want:     "v2:jav:avwiki:lookup_actress_name:九井スナオ",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := lookupCacheKey(tc.provider, tc.method, tc.input); got != tc.want {
				t.Fatalf("lookupCacheKey() = %q, want %q", got, tc.want)
			}
		})
	}
}

type memoryLookupCache struct {
	items map[string]memoryLookupCacheItem
}

type memoryLookupCacheItem struct {
	value     []byte
	expiresAt time.Time
}

func newMemoryLookupCache() *memoryLookupCache {
	return &memoryLookupCache{items: map[string]memoryLookupCacheItem{}}
}

func (m *memoryLookupCache) Get(key string, now time.Time) ([]byte, bool, error) {
	item, ok := m.items[key]
	if !ok || !item.expiresAt.After(now) {
		return nil, false, nil
	}
	return item.value, true, nil
}

func (m *memoryLookupCache) Set(key string, value []byte, expiresAt time.Time) error {
	m.items[key] = memoryLookupCacheItem{value: append([]byte(nil), value...), expiresAt: expiresAt}
	return nil
}

type countingLookupProvider struct {
	javInfo    *JavInfo
	actress    *ActressInfo
	profileURL string
	seriesURL  string
	studioURL  string
	err        error

	javCalls int
}

func (p *countingLookupProvider) LookupJavByCode(_ context.Context, _ string) (*JavInfo, error) {
	p.javCalls++
	return p.javInfo, p.err
}
