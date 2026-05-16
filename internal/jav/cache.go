package jav

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"
)

const (
	lookupCacheDefaultKeyVersion = "v1"
	lookupCacheStatusHit         = "hit"
	lookupCacheStatusMiss        = "not_found"

	lookupCacheSuccessTTL  = 90 * 24 * time.Hour
	lookupCacheNotFoundTTL = 7 * 24 * time.Hour
)

var lookupJavCacheKeyVersionByProvider = map[Provider]string{
	ProviderJavDatabase: "v2",
	ProviderAvmoo:       "v2",
}

// LookupCache is a persistent key-value store for provider lookup results.
type LookupCache interface {
	Get(key string, now time.Time) ([]byte, bool, error)
	Set(key string, value []byte, expiresAt time.Time) error
}

type lookupCacheEnvelope struct {
	Status string          `json:"status"`
	Data   json.RawMessage `json:"data,omitempty"`
}

var lookupCacheState = struct {
	sync.RWMutex
	store LookupCache
}{}

// SetCache configures the process-wide JAV lookup cache. Passing nil disables caching.
func SetCache(store LookupCache) {
	lookupCacheState.Lock()
	lookupCacheState.store = store
	lookupCacheState.Unlock()
}

func currentLookupCache() LookupCache {
	lookupCacheState.RLock()
	defer lookupCacheState.RUnlock()
	return lookupCacheState.store
}

func lookupCacheGet[T any](key string) (*T, bool, error) {
	store := currentLookupCache()
	if store == nil {
		return nil, false, nil
	}
	raw, ok, err := store.Get(key, time.Now())
	if err != nil || !ok {
		return nil, false, nil
	}
	var envelope lookupCacheEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, false, nil
	}
	switch envelope.Status {
	case lookupCacheStatusMiss:
		return nil, true, ResourceNotFonud
	case lookupCacheStatusHit:
		if len(envelope.Data) == 0 {
			return nil, false, nil
		}
		var value T
		if err := json.Unmarshal(envelope.Data, &value); err != nil {
			return nil, false, nil
		}
		return &value, true, nil
	default:
		return nil, false, nil
	}
}

func lookupCacheSetHit(key string, value any) {
	if value == nil {
		return
	}
	store := currentLookupCache()
	if store == nil {
		return
	}
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	raw, err := json.Marshal(lookupCacheEnvelope{
		Status: lookupCacheStatusHit,
		Data:   data,
	})
	if err != nil {
		return
	}
	_ = store.Set(key, raw, time.Now().Add(lookupCacheSuccessTTL))
}

func lookupCacheSetNotFound(key string) {
	store := currentLookupCache()
	if store == nil {
		return
	}
	raw, err := json.Marshal(lookupCacheEnvelope{
		Status: lookupCacheStatusMiss,
	})
	if err != nil {
		return
	}
	_ = store.Set(key, raw, time.Now().Add(lookupCacheNotFoundTTL))
}

func cacheableLookupResult(key string, value any, err error) {
	if err == nil {
		lookupCacheSetHit(key, value)
		return
	}
	if errors.Is(err, ResourceNotFonud) {
		lookupCacheSetNotFound(key)
	}
}

func lookupCacheKey(provider Provider, method, input string) string {
	return strings.Join([]string{
		lookupCacheKeyVersion(provider, method),
		"jav",
		ParseProvider(int(provider)).String(),
		method,
		normalizeLookupCacheInput(method, input),
	}, ":")
}

func lookupCacheKeyVersion(provider Provider, method string) string {
	provider = ParseProvider(int(provider))
	if method == "lookup_jav" {
		if version, ok := lookupJavCacheKeyVersionByProvider[provider]; ok {
			return version
		}
	}
	return lookupCacheDefaultKeyVersion
}

func normalizeLookupCacheInput(method, input string) string {
	input = strings.TrimSpace(input)
	switch method {
	case "lookup_jav", "lookup_cover", "lookup_actress_code":
		return strings.ToUpper(input)
	default:
		return strings.Join(strings.Fields(input), " ")
	}
}
