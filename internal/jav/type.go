package jav

import (
	"errors"
	"strings"
	"sync"
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
	default:
		return "unknown"
	}
}

// ParseProvider converts a persisted numeric provider to a known enum.
func ParseProvider(value int) Provider {
	p := Provider(value)
	switch p {
	case ProviderJavBus, ProviderJavDatabase, ProviderUser, ProviderJavDB, ProviderAvmoo, ProviderThePornDB, ProviderJavModel:
		return p
	default:
		return ProviderUnknown
	}
}

// MetadataLanguage identifies the preferred language for fetched JAV metadata.
type MetadataLanguage string

const (
	MetadataLanguageChinese MetadataLanguage = "zh"
	MetadataLanguageEnglish MetadataLanguage = "en"
)

var metadataLanguageState = struct {
	sync.RWMutex
	value MetadataLanguage
}{value: MetadataLanguageChinese}

var errUnsupportedProvider = errors.New("jav: unsupported provider")

var lookupProvidersByProvider = map[Provider]lookupProvider{
	ProviderJavBus:      javBusProvider,
	ProviderJavDatabase: javDatabaseProvider,
	ProviderJavDB:       javDBProvider,
	ProviderAvmoo:       avmooProvider,
	ProviderThePornDB:   thePornDBProvider,
	ProviderJavModel:    javModelProvider,
}

var metadataLanguageByProvider = map[Provider]MetadataLanguage{
	ProviderJavBus:      MetadataLanguageChinese,
	ProviderJavDB:       MetadataLanguageChinese,
	ProviderAvmoo:       MetadataLanguageChinese,
	ProviderJavDatabase: MetadataLanguageEnglish,
	ProviderThePornDB:   MetadataLanguageEnglish,
}

// ParseMetadataLanguage converts user config to a known metadata language.
func ParseMetadataLanguage(value string) (MetadataLanguage, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "en", "eng", "english", "en-us", "en-gb":
		return MetadataLanguageEnglish, true
	case "zh", "cn", "chi", "chinese", "zh-cn", "zh-hans", "zh-tw", "zh-hant":
		return MetadataLanguageChinese, true
	default:
		return MetadataLanguageChinese, false
	}
}

// NormalizeMetadataLanguage converts user config to a known metadata language.
func NormalizeMetadataLanguage(value string) MetadataLanguage {
	lang, ok := ParseMetadataLanguage(value)
	if !ok {
		return MetadataLanguageChinese
	}
	return lang
}

// SetMetadataLanguage updates the process-wide preferred JAV metadata language.
func SetMetadataLanguage(value string) MetadataLanguage {
	lang := NormalizeMetadataLanguage(value)
	metadataLanguageState.Lock()
	metadataLanguageState.value = lang
	metadataLanguageState.Unlock()
	return lang
}

// CurrentMetadataLanguage returns the process-wide preferred JAV metadata language.
func CurrentMetadataLanguage() MetadataLanguage {
	metadataLanguageState.RLock()
	defer metadataLanguageState.RUnlock()
	return metadataLanguageState.value
}

// CurrentMetadataLanguageIsEnglish reports whether the process-wide metadata language is English.
func CurrentMetadataLanguageIsEnglish() bool {
	return CurrentMetadataLanguage() == MetadataLanguageEnglish
}

// ProviderMetadataLanguage returns the metadata language produced by a provider.
func ProviderMetadataLanguage(provider Provider) MetadataLanguage {
	if language, ok := metadataLanguageByProvider[ParseProvider(int(provider))]; ok {
		return language
	}
	return MetadataLanguageChinese
}

// ProviderIsEnglish reports whether a provider stores English metadata.
func ProviderIsEnglish(provider Provider) bool {
	return ProviderMetadataLanguage(provider) == MetadataLanguageEnglish
}

// PreferredProvider chooses the metadata source based on the configured language.
func PreferredProvider() Provider {
	if CurrentMetadataLanguageIsEnglish() {
		return ProviderJavDatabase
	}
	return ProviderJavBus
}

// JavInfo holds basic metadata extracted from a JAV metadata provider.
type JavInfo struct {
	Title       string
	Code        string
	Studio      string
	Series      string
	ReleaseUnix int64
	DurationMin int
	Tags        []string
	Actors      []string
	Provider    Provider
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
	LookupActressByJapaneseName(name string) (*ActressInfo, error)
	LookupCoverURLByCode(code string) (string, error)
	LookupJavByCode(code string) (*JavInfo, error)
}

func lookupProviderFor(provider Provider) (lookupProvider, error) {
	provider = ParseProvider(int(provider))
	if provider == ProviderUnknown || provider == ProviderUser {
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
	info, err = lookup.LookupActressByJapaneseName(name)
	cacheableLookupResult(cacheKey, info, err)
	return info, err
}

// LookupCoverURLByCode fetches a cover image URL from the selected provider.
func LookupCoverURLByCode(code string, provider Provider) (coverURL string, err error) {
	lookup, err := lookupProviderFor(provider)
	if err != nil {
		return "", err
	}
	cacheKey := lookupCacheKey(provider, "lookup_cover", code)
	if cached, ok, err := lookupCacheGet[string](cacheKey); ok {
		if cached == nil {
			return "", err
		}
		return *cached, err
	}
	defer recoverUnsupportedProvider(&err)
	coverURL, err = lookup.LookupCoverURLByCode(code)
	cacheableLookupResult(cacheKey, coverURL, err)
	return coverURL, err
}

func recoverUnsupportedProvider(err *error) {
	if r := recover(); r != nil {
		*err = errUnsupportedProvider
	}
}
