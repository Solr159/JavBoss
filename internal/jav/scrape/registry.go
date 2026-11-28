package scrape

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/jav/scrape/avmoo"
	"javboss/internal/jav/scrape/avsox"
	"javboss/internal/jav/scrape/avwiki"
	"javboss/internal/jav/scrape/internal/baseurl"
	"javboss/internal/jav/scrape/javbus"
	"javboss/internal/jav/scrape/javdatabase"
	"javboss/internal/jav/scrape/javdb"
	"javboss/internal/jav/scrape/javdbapi"
	"javboss/internal/jav/scrape/javmenu"
	"javboss/internal/jav/scrape/javmodel"
	"javboss/internal/jav/scrape/minnanoav"
	"javboss/internal/jav/scrape/theporndb"
	"javboss/internal/util"
)

// MetadataClient owns the provider registry and an independently configurable cache.
// Providers supplied to NewMetadataClient must support concurrent queries.
type MetadataClient struct {
	providersMu          sync.RWMutex
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
		httpClient := newCachedProviderHTTPClient(id)
		result[id], _ = newProvider(id, httpClient)
	}
	return result
}

// Normal lookups cache failed URLs for every provider. Availability checks use
// newProviderHTTPClient directly so they always reach the network.
func newCachedProviderHTTPClient(id Provider) *http.Client {
	return util.WithNegativeCache(newProviderHTTPClient(id))
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
	c.providersMu.RLock()
	implementation, ok := c.providers[provider]
	c.providersMu.RUnlock()
	if !ok || implementation == nil {
		return nil, ErrUnsupportedProvider
	}
	return implementation, nil
}

// baseURLProvider is implemented by scrapers whose site origin users may override.
type baseURLProvider interface {
	BaseURL() string
	SetBaseURL(string)
}

// ConfigurableBaseURLProviders lists the providers whose site origin can be
// overridden. Public API services keep their fixed endpoints.
func ConfigurableBaseURLProviders() []Provider {
	return []Provider{
		ProviderJavBus, ProviderJavDatabase, ProviderJavDB, ProviderAvmoo, ProviderAvsox,
		ProviderJavMenu, ProviderJavModel, ProviderMinnanoAV, ProviderAVWiki,
	}
}

// DefaultProviderBaseURL returns the official origin of a configurable provider,
// or an empty string for providers without a configurable origin.
func DefaultProviderBaseURL(provider Provider) string {
	switch ParseProvider(int(provider)) {
	case ProviderJavBus:
		return javbus.DefaultBaseURL
	case ProviderJavDatabase:
		return javdatabase.DefaultBaseURL
	case ProviderJavDB:
		return javdb.DefaultBaseURL
	case ProviderAvmoo:
		return avmoo.DefaultBaseURL
	case ProviderAvsox:
		return avsox.DefaultBaseURL
	case ProviderJavMenu:
		return javmenu.DefaultBaseURL
	case ProviderJavModel:
		return javmodel.DefaultBaseURL
	case ProviderMinnanoAV:
		return minnanoav.DefaultBaseURL
	case ProviderAVWiki:
		return avwiki.DefaultBaseURL
	default:
		return ""
	}
}

// ProviderBaseURLConfigKey returns the config key that stores a provider's
// configured origin, or an empty string for providers without one.
func ProviderBaseURLConfigKey(provider Provider) string {
	provider = ParseProvider(int(provider))
	if DefaultProviderBaseURL(provider) == "" {
		return ""
	}
	return provider.String() + "_base_url"
}

// NormalizeProviderBaseURL validates a configured site origin. Empty input
// selects the default origin.
func NormalizeProviderBaseURL(provider Provider, raw string) (string, error) {
	provider = ParseProvider(int(provider))
	if DefaultProviderBaseURL(provider) == "" {
		return "", ErrUnsupportedProvider
	}
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	normalized, err := baseurl.Normalize(raw)
	if err != nil {
		return "", fmt.Errorf("%s base url: %w", provider, err)
	}
	return normalized, nil
}

// ProviderBaseURL returns the configured origin of a provider, or an empty
// string when the default origin is in use.
func ProviderBaseURL(provider Provider) string {
	return defaultMetadataClient.ProviderBaseURL(provider)
}

// ProviderBaseURL returns this client's configured origin of a provider, or an
// empty string when the default origin is in use.
func (c *MetadataClient) ProviderBaseURL(provider Provider) string {
	implementation, err := c.providerFor(provider)
	if err != nil {
		return ""
	}
	return configuredProviderBaseURL(provider, implementation)
}

func configuredProviderBaseURL(provider Provider, implementation any) string {
	effective := effectiveProviderBaseURL(implementation)
	if effective == "" || effective == DefaultProviderBaseURL(provider) {
		return ""
	}
	return effective
}

// effectiveProviderBaseURL reports the origin a provider currently requests.
func effectiveProviderBaseURL(implementation any) string {
	scraper, ok := implementation.(baseURLProvider)
	if !ok {
		return ""
	}
	return strings.TrimSpace(scraper.BaseURL())
}

// applyProviderBaseURL aligns a freshly created provider with the configured origin.
func (c *MetadataClient) applyProviderBaseURL(provider Provider, implementation any) {
	setter, ok := implementation.(interface{ SetBaseURL(string) })
	if !ok {
		return
	}
	if configured := c.ProviderBaseURL(provider); configured != "" {
		setter.SetBaseURL(configured)
	}
}

// SetProviderBaseURL changes a provider's origin on the default metadata client.
// An empty value restores the official origin.
func SetProviderBaseURL(provider Provider, raw string) error {
	provider = ParseProvider(int(provider))
	normalized, err := NormalizeProviderBaseURL(provider, raw)
	if err != nil {
		return err
	}
	c := defaultMetadataClient
	c.providersMu.Lock()
	defer c.providersMu.Unlock()

	origin := normalized
	if origin == "" {
		origin = DefaultProviderBaseURL(provider)
	}
	if effectiveProviderBaseURL(c.providers[provider]) == origin {
		return nil
	}

	// In-flight lookups keep their original instance, including its sessions.
	implementation, err := newProvider(provider, newCachedProviderHTTPClient(provider))
	if err != nil {
		return err
	}
	setter, ok := implementation.(baseURLProvider)
	if !ok {
		return ErrUnsupportedProvider
	}
	setter.SetBaseURL(normalized)
	c.providers[provider] = implementation
	c.invalidateProviderAvailability(provider)
	refreshJavImageSources(c.providers)
	return nil
}

// ApplyProviderBaseURLConfig applies every persisted provider origin in cfg so
// startup and config reloads agree with the stored values.
func ApplyProviderBaseURLConfig(cfg map[string]string) {
	for _, provider := range ConfigurableBaseURLProviders() {
		raw := strings.TrimSpace(cfg[ProviderBaseURLConfigKey(provider)])
		if raw == "" {
			continue
		}
		if err := SetProviderBaseURL(provider, raw); err != nil {
			logging.Error("apply %s base url failed: %v", provider, err)
		}
	}
}

// refreshJavImageSources teaches the image proxy which Referer each configured
// origin expects so covers served from a mirror keep loading.
func refreshJavImageSources(providers map[Provider]any) {
	sources := make([]util.JavImageSource, 0, len(ConfigurableBaseURLProviders()))
	for _, provider := range ConfigurableBaseURLProviders() {
		origin := effectiveProviderBaseURL(providers[provider])
		if origin == "" {
			continue
		}
		source := util.JavImageSource{Origin: origin}
		if provider == ProviderJavBus {
			source.Cookie = "age=verified; existmag=mag"
		}
		sources = append(sources, source)
	}
	util.SetJavImageSources(sources)
}
