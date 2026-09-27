package jav

import (
	"encoding/json"
	"errors"
	"strings"
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
	ProviderJavBus:      "v6", // Include the studio parsed from the movie page.
	ProviderJavDatabase: "v4",
	ProviderJavDB:       "v4",
	ProviderJavDBAPI:    "v5", // Preserve separators when validating movie numbers.
	ProviderAvmoo:       "v6",
	ProviderAvsox:       "v3",
	ProviderJavMenu:     "v2",
	ProviderThePornDB:   "v2",
}

var lookupActressNameCacheKeyVersionByProvider = map[Provider]string{
	ProviderMinnanoAV: "v3",
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

// SetCache configures the default client's lookup cache. Nil disables caching.
func SetCache(store LookupCache) { defaultClient.SetCache(store) }

// SetCache changes this client's lookup cache safely while lookups are running.
func (c *Client) SetCache(store LookupCache) {
	c.cacheMu.Lock()
	c.cache = store
	c.cacheMu.Unlock()
}
func (c *Client) currentLookupCache() LookupCache {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()
	return c.cache
}

func lookupCacheGet[T any](c *Client, key string) (*T, bool, error) {
	store := c.currentLookupCache()
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
		return nil, true, ErrNotFound
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

func lookupCacheSetHit(c *Client, key string, value any) {
	if value == nil {
		return
	}
	store := c.currentLookupCache()
	if store == nil {
		return
	}
	data, err := json.Marshal(value)
	if err != nil || string(data) == "null" {
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

func lookupCacheSetNotFound(c *Client, key string) {
	store := c.currentLookupCache()
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

func cacheableLookupResult(c *Client, key string, value any, err error) {
	if err == nil {
		lookupCacheSetHit(c, key, value)
		return
	}
	if errors.Is(err, ErrNotFound) {
		lookupCacheSetNotFound(c, key)
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
	if provider == ProviderJavDBAPI {
		switch method {
		case "lookup_actress_url_code_name", "lookup_series_url", "lookup_studio_url":
			return "v2" // These links also depend on strict movie-number matching.
		}
	}
	if provider == ProviderJavDB && method == "lookup_actress_url_code_name" {
		return "v3"
	}
	if provider == ProviderJavDatabase && method == "lookup_actress_code" {
		return "v2"
	}
	if method == "lookup_jav" {
		if version, ok := lookupJavCacheKeyVersionByProvider[provider]; ok {
			return version
		}
	}
	if method == "lookup_actress_name" {
		if version, ok := lookupActressNameCacheKeyVersionByProvider[provider]; ok {
			return version
		}
	}
	return lookupCacheDefaultKeyVersion
}

func normalizeLookupCacheInput(method, input string) string {
	input = strings.TrimSpace(input)
	switch method {
	case "lookup_jav", "lookup_actress_code":
		return strings.ToUpper(input)
	default:
		return strings.Join(strings.Fields(input), " ")
	}
}
