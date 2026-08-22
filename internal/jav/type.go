package jav

import (
	"errors"
	"strings"
)

// ResourceNotFonud indicates the requested resource is not available.
var ResourceNotFonud = errors.New("jav: resource not found")

// Provider identifies where JAV metadata or tags came from.
type Provider int

const (
	ProviderUnknown Provider = iota
	ProviderJavBus
	ProviderJavDatabase
	ProviderUser
	ProviderJavDB
	ProviderAvmoo
	ProviderThePornDB
	ProviderJavModel
	ProviderAvsox
	ProviderJavMenu
	ProviderMinnanoAV
	ProviderManualScrape
)

func (p Provider) String() string {
	switch p {
	case ProviderJavBus:
		return "javbus"
	case ProviderJavDatabase:
		return "javdatabase"
	case ProviderUser:
		return "user"
	case ProviderJavDB:
		return "javdb"
	case ProviderAvmoo:
		return "avmoo"
	case ProviderThePornDB:
		return "theporndb"
	case ProviderJavModel:
		return "javmodel"
	case ProviderAvsox:
		return "avsox"
	case ProviderJavMenu:
		return "javmenu"
	case ProviderMinnanoAV:
		return "minnanoav"
	case ProviderManualScrape:
		return "manual_scrape"
	default:
		return "unknown"
	}
}

// ParseProvider converts a persisted numeric provider to a known enum.
func ParseProvider(value int) Provider {
	p := Provider(value)
	switch p {
	case ProviderJavBus, ProviderJavDatabase, ProviderUser, ProviderJavDB, ProviderAvmoo, ProviderThePornDB, ProviderJavModel, ProviderAvsox, ProviderJavMenu, ProviderMinnanoAV, ProviderManualScrape:
		return p
	default:
		return ProviderUnknown
	}
}

var errUnsupportedProvider = errors.New("jav: unsupported provider")

var lookupProvidersByProvider = map[Provider]lookupProvider{
	ProviderJavBus:      javBusProvider,
	ProviderJavDatabase: javDatabaseProvider,
	ProviderJavDB:       javDBProvider,
	ProviderAvmoo:       avmooProvider,
	ProviderThePornDB:   thePornDBProvider,
	ProviderJavModel:    javModelProvider,
	ProviderAvsox:       avsoxProvider,
	ProviderJavMenu:     javMenuProvider,
	ProviderMinnanoAV:   minnanoAVProvider,
}

// JavInfo holds basic metadata extracted from a JAV metadata provider.
type JavInfo struct {
	Title        string
	Code         string
	Studio       string
	Series       string
	ReleaseUnix  int64
	DurationMin  int
	Tags         []string
	Actors       []string
	CoverURL     string
	SampleImages []SampleImage
	IsUncensored *bool `json:",omitempty"`
	Provider     Provider
}

// ActressInfo describes basic actress profile fields from JavDatabase.
type ActressInfo struct {
	RomanName    string
	JapaneseName string
	ChineseName  string
	HeightCM     int
	Bust         int
	Waist        int
	Hips         int
	BirthDate    int
	Cup          int
	ProfileURL   string
}

// lookupProvider can resolve both JAV metadata and actress profiles by code.
// Return ResourceNotFonud when the code is invalid or not found.
// Return other error for retryable lookup failures.
type lookupProvider interface {
	LookupActressByCode(code string) (*ActressInfo, error)
	LookupActressByName(name string) (*ActressInfo, error)
	LookupActressURLByCodeAndName(code, name string) (string, error)
	LookupJavByCode(code string) (*JavInfo, error)
	LookupSeriesURLByCode(code string) (string, error)
	LookupStudioURLByCode(code string) (string, error)
}

func lookupProviderFor(provider Provider) (lookupProvider, error) {
	provider = ParseProvider(int(provider))
	if provider == ProviderUnknown || provider == ProviderUser || provider == ProviderManualScrape {
		return nil, errUnsupportedProvider
	}
	lookup, ok := lookupProvidersByProvider[provider]
	if !ok || lookup == nil {
		return nil, errUnsupportedProvider
	}
	return lookup, nil
}

// LookupJavByCode fetches JAV metadata from the selected provider.
func LookupJavByCode(code string, provider Provider) (info *JavInfo, err error) {
	lookup, err := lookupProviderFor(provider)
	if err != nil {
		return nil, err
	}
	cacheKey := lookupCacheKey(provider, "lookup_jav", code)
	if cached, ok, err := lookupCacheGet[JavInfo](cacheKey); ok {
		return cached, err
	}
	defer recoverUnsupportedProvider(&err)
	info, err = lookup.LookupJavByCode(code)
	cacheableLookupResult(cacheKey, info, err)
	return info, err
}

// LookupActressByCode fetches actress profile metadata by a movie code from the selected provider.
func LookupActressByCode(code string, provider Provider) (info *ActressInfo, err error) {
	lookup, err := lookupProviderFor(provider)
	if err != nil {
		return nil, err
	}
	cacheKey := lookupCacheKey(provider, "lookup_actress_code", code)
	if cached, ok, err := lookupCacheGet[ActressInfo](cacheKey); ok {
		return cached, err
	}
	defer recoverUnsupportedProvider(&err)
	info, err = lookup.LookupActressByCode(code)
	cacheableLookupResult(cacheKey, info, err)
	return info, err
}

// LookupActressByJapaneseName fetches actress profile metadata by Japanese/name text from the selected provider.
func LookupActressByJapaneseName(name string, provider Provider) (info *ActressInfo, err error) {
	lookup, err := lookupProviderFor(provider)
	if err != nil {
		return nil, err
	}
	cacheKey := lookupCacheKey(provider, "lookup_actress_name", name)
	if cached, ok, err := lookupCacheGet[ActressInfo](cacheKey); ok {
		return cached, err
	}
	defer recoverUnsupportedProvider(&err)
	info, err = lookup.LookupActressByName(name)
	cacheableLookupResult(cacheKey, info, err)
	return info, err
}

// LookupActressURLByCodeAndName fetches an actress profile URL using a movie code and actress name.
func LookupActressURLByCodeAndName(code, name string, provider Provider) (actressURL string, err error) {
	lookup, err := lookupProviderFor(provider)
	if err != nil {
		return "", err
	}
	cacheInput := strings.ToUpper(strings.TrimSpace(code)) + "|" + strings.Join(strings.Fields(name), " ")
	cacheKey := lookupCacheKey(provider, "lookup_actress_url_code_name", cacheInput)
	if cached, ok, err := lookupCacheGet[string](cacheKey); ok {
		if cached == nil {
			return "", err
		}
		return *cached, err
	}
	defer recoverUnsupportedProvider(&err)
	actressURL, err = lookup.LookupActressURLByCodeAndName(code, name)
	cacheableLookupResult(cacheKey, actressURL, err)
	return actressURL, err
}

// LookupSeriesURLByCode fetches a series detail URL using a movie code.
func LookupSeriesURLByCode(code string, provider Provider) (seriesURL string, err error) {
	lookup, err := lookupProviderFor(provider)
	if err != nil {
		return "", err
	}
	cacheKey := lookupCacheKey(provider, "lookup_series_url", code)
	if cached, ok, err := lookupCacheGet[string](cacheKey); ok {
		if cached == nil {
			return "", err
		}
		return *cached, err
	}
	defer recoverUnsupportedProvider(&err)
	seriesURL, err = lookup.LookupSeriesURLByCode(code)
	cacheableLookupResult(cacheKey, seriesURL, err)
	return seriesURL, err
}

// LookupStudioURLByCode fetches a studio detail URL using a movie code.
func LookupStudioURLByCode(code string, provider Provider) (studioURL string, err error) {
	lookup, err := lookupProviderFor(provider)
	if err != nil {
		return "", err
	}
	cacheKey := lookupCacheKey(provider, "lookup_studio_url", code)
	if cached, ok, err := lookupCacheGet[string](cacheKey); ok {
		if cached == nil {
			return "", err
		}
		return *cached, err
	}
	defer recoverUnsupportedProvider(&err)
	studioURL, err = lookup.LookupStudioURLByCode(code)
	cacheableLookupResult(cacheKey, studioURL, err)
	return studioURL, err
}

func recoverUnsupportedProvider(err *error) {
	if r := recover(); r != nil {
		*err = errUnsupportedProvider
	}
}
