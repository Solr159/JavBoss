package jav

import (
	"net/http"
	"sync"
	"time"

	"javboss/internal/jav/avmoo"
	"javboss/internal/jav/avsox"
	"javboss/internal/jav/avwiki"
	"javboss/internal/jav/javbus"
	"javboss/internal/jav/javdatabase"
	"javboss/internal/jav/javdb"
	"javboss/internal/jav/javdbapi"
	"javboss/internal/jav/javmenu"
	"javboss/internal/jav/javmodel"
	"javboss/internal/jav/minnanoav"
	"javboss/internal/jav/theporndb"
	"javboss/internal/util"
)

// MetadataClient owns an immutable provider registry and an independently configurable cache.
// Providers supplied to NewMetadataClient must support concurrent queries.
type MetadataClient struct {
	providers            map[Provider]any
	availabilityFactory  func(Provider, *http.Client) (any, error)
	cacheMu              sync.RWMutex
	cache                LookupCache
	availabilityMu       sync.Mutex
	availabilitySequence uint64
	availabilityResults  map[Provider]availabilityCacheEntry
}

// NewMetadataClient copies providers so callers cannot mutate its registry. Nil selects the built-in providers.
func NewMetadataClient(providers map[Provider]any, cache LookupCache) *MetadataClient {
	if providers == nil {
		providers = defaultProviders()
	}
	registry := make(map[Provider]any, len(providers))
	for id, provider := range providers {
		registry[id] = provider
	}
	return &MetadataClient{providers: registry, cache: cache, availabilityFactory: newProvider}
}

var defaultMetadataClient = NewMetadataClient(nil, nil)

func defaultProviders() map[Provider]any {
	result := make(map[Provider]any)
	for _, id := range []Provider{ProviderJavBus, ProviderJavDatabase, ProviderJavDB, ProviderJavDBAPI, ProviderAvmoo, ProviderAvsox, ProviderJavMenu, ProviderJavModel, ProviderMinnanoAV, ProviderThePornDB, ProviderAVWiki} {
		httpClient := newProviderHTTPClient(id)
		// URL caching is configured here, independently of provider parsing.
		switch id {
		case ProviderJavBus, ProviderJavDatabase, ProviderJavMenu, ProviderJavModel, ProviderMinnanoAV, ProviderThePornDB:
			httpClient = util.WithNegativeCache(httpClient)
		}
		result[id], _ = newProvider(id, httpClient)
	}
	return result
}

// newProviderHTTPClient creates a new connection pool with the site's settings.
// Cache and status-recording policies are added by the caller before injection.
func newProviderHTTPClient(id Provider) *http.Client {
	switch id {
	case ProviderJavDB:
		return javdb.NewHTTPClient()
	case ProviderJavDBAPI:
		return util.NewHTTPClient(20 * time.Second)
	case ProviderAvmoo:
		return avmoo.NewHTTPClient()
	case ProviderAvsox:
		return avsox.NewHTTPClient()
	default:
		return util.NewDefaultHTTPClient()
	}
}

func newProvider(id Provider, httpClient *http.Client) (any, error) {
	switch id {
	case ProviderJavBus:
		return javbus.New(httpClient), nil
	case ProviderJavDatabase:
		return javdatabase.New(httpClient), nil
	case ProviderJavDB:
		return javdb.New(httpClient), nil
	case ProviderJavDBAPI:
		return javdbapi.New(httpClient, ""), nil
	case ProviderAvmoo:
		return avmoo.New(httpClient), nil
	case ProviderAvsox:
		return avsox.New(httpClient), nil
	case ProviderJavMenu:
		return javmenu.New(httpClient), nil
	case ProviderJavModel:
		return javmodel.New(httpClient), nil
	case ProviderMinnanoAV:
		return minnanoav.New(httpClient), nil
	case ProviderThePornDB:
		return theporndb.New(httpClient), nil
	case ProviderAVWiki:
		return avwiki.New(httpClient), nil
	default:
		return nil, ErrUnsupportedProvider
	}
}

func (c *MetadataClient) providerFor(provider Provider) (any, error) {
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
