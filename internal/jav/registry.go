package jav

import (
	"sync"

	"javboss/internal/jav/avmoo"
	"javboss/internal/jav/avsox"
	"javboss/internal/jav/javbus"
	"javboss/internal/jav/javdatabase"
	"javboss/internal/jav/javdb"
	"javboss/internal/jav/javmenu"
	"javboss/internal/jav/javmodel"
	"javboss/internal/jav/minnanoav"
	"javboss/internal/jav/theporndb"
)

// Client owns an immutable provider registry and an independently configurable cache.
// Providers supplied to NewClient must support concurrent queries.
type Client struct {
	providers            map[Provider]any
	cacheMu              sync.RWMutex
	cache                LookupCache
	connectivityMu       sync.Mutex
	connectivitySequence uint64
	connectivityResults  map[Provider]connectivityCacheEntry
}

// NewClient copies providers so callers cannot mutate its registry. Nil selects the built-in providers.
func NewClient(providers map[Provider]any, cache LookupCache) *Client {
	if providers == nil {
		providers = defaultProviders()
	}
	registry := make(map[Provider]any, len(providers))
	for id, provider := range providers {
		registry[id] = provider
	}
	return &Client{providers: registry, cache: cache}
}

var defaultClient = NewClient(nil, nil)

func defaultProviders() map[Provider]any {
	html, api := javdb.NewProviders()
	return map[Provider]any{
		ProviderJavBus: javbus.New(), ProviderJavDatabase: javdatabase.New(),
		ProviderJavDB: html, ProviderJavDBAPI: api,
		ProviderAvmoo: avmoo.New(), ProviderAvsox: avsox.New(),
		ProviderJavMenu: javmenu.New(), ProviderJavModel: javmodel.New(),
		ProviderMinnanoAV: minnanoav.New(), ProviderThePornDB: theporndb.New(),
	}
}

func (c *Client) providerFor(provider Provider) (any, error) {
	provider = ParseProvider(int(provider))
	if provider == ProviderUnknown || provider == ProviderUser || provider == ProviderManualScrape {
		return nil, ErrUnsupportedProvider
	}
	implementation, ok := c.providers[provider]
	if !ok || implementation == nil {
		return nil, ErrUnsupportedProvider
	}
	return implementation, nil
}
