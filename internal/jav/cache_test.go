package jav

import (
	"errors"
	"testing"
	"time"
)

func TestLookupJavByCodeUsesCache(t *testing.T) {
	cache := newMemoryLookupCache()
	SetCache(cache)
	t.Cleanup(func() { SetCache(nil) })

	original := lookupProvidersByProvider[ProviderJavBus]
	provider := &countingLookupProvider{
		javInfo: &JavInfo{Code: "ABC-001", Title: "Cached Title", Provider: ProviderJavBus},
	}
	lookupProvidersByProvider[ProviderJavBus] = provider
	t.Cleanup(func() { lookupProvidersByProvider[ProviderJavBus] = original })

	first, err := LookupJavByCode("abc-001", ProviderJavBus)
	if err != nil {
		t.Fatalf("first lookup: %v", err)
	}
	second, err := LookupJavByCode("ABC-001", ProviderJavBus)
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
	SetCache(cache)
	t.Cleanup(func() { SetCache(nil) })

	original := lookupProvidersByProvider[ProviderJavBus]
	provider := &countingLookupProvider{err: ResourceNotFonud}
	lookupProvidersByProvider[ProviderJavBus] = provider
	t.Cleanup(func() { lookupProvidersByProvider[ProviderJavBus] = original })

	for i := 0; i < 2; i++ {
		_, err := LookupJavByCode("MISS-001", ProviderJavBus)
		if !errors.Is(err, ResourceNotFonud) {
			t.Fatalf("lookup %d err=%v want ResourceNotFonud", i, err)
		}
	}
	if provider.javCalls != 1 {
		t.Fatalf("unexpected provider calls: got %d want 1", provider.javCalls)
	}
}

func TestLookupJavByCodeDoesNotCacheTemporaryErrors(t *testing.T) {
	cache := newMemoryLookupCache()
	SetCache(cache)
	t.Cleanup(func() { SetCache(nil) })

	original := lookupProvidersByProvider[ProviderJavBus]
	provider := &countingLookupProvider{err: errors.New("temporary")}
	lookupProvidersByProvider[ProviderJavBus] = provider
	t.Cleanup(func() { lookupProvidersByProvider[ProviderJavBus] = original })

	for i := 0; i < 2; i++ {
		_, err := LookupJavByCode("TMP-001", ProviderJavBus)
		if err == nil {
			t.Fatalf("lookup %d expected error", i)
		}
	}
	if provider.javCalls != 2 {
		t.Fatalf("unexpected provider calls: got %d want 2", provider.javCalls)
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
	javInfo  *JavInfo
	actress  *ActressInfo
	coverURL string
	err      error

	javCalls int
}

func (p *countingLookupProvider) LookupActressByCode(string) (*ActressInfo, error) {
	return p.actress, p.err
}

func (p *countingLookupProvider) LookupActressByJapaneseName(string) (*ActressInfo, error) {
	return p.actress, p.err
}

func (p *countingLookupProvider) LookupCoverURLByCode(string) (string, error) {
	return p.coverURL, p.err
}

func (p *countingLookupProvider) LookupJavByCode(string) (*JavInfo, error) {
	p.javCalls++
	return p.javInfo, p.err
}
