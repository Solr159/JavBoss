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
	default:
		return "unknown"
	}
}

// ParseProvider converts a persisted numeric provider to a known enum.
func ParseProvider(value int) Provider {
	p := Provider(value)
	switch p {
	case ProviderJavBus, ProviderJavDatabase, ProviderUser, ProviderJavDB, ProviderAvmoo:
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

var lookupProvidersByProvider = map[Provider]JavLookupProvider{
	ProviderJavBus:      JavBusProvider,
	ProviderJavDatabase: JavDatabaseProvider,
	ProviderJavDB:       JavDBProvider,
	ProviderAvmoo:       AvmooProvider,
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

// PreferredProvider chooses the metadata source based on the configured language.
func PreferredProvider() Provider {
	if CurrentMetadataLanguage() == MetadataLanguageEnglish {
		return ProviderJavDatabase
	}
	return ProviderJavBus
}

// PreferredLookupProvider returns the scraper that matches the configured metadata language.
func PreferredLookupProvider() JavLookupProvider {
	if provider, ok := lookupProvidersByProvider[PreferredProvider()]; ok {
		return provider
	}
	return JavBusProvider
}

// Info holds basic metadata extracted from a JAV metadata provider.
type Info struct {
	Title       string
	Code        string
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

// JavLookupProvider can resolve both JAV metadata and actress profiles by code.
// Return ResourceNotFonud when the code is invalid or not found.
// Return other error for retryable lookup failures.
type JavLookupProvider interface {
	LookupActressByCode(code string) (*ActressInfo, error)
	LookupActressByJapaneseName(name string) (*ActressInfo, error)
	LookupCoverURLByCode(code string) (string, error)
	LookupJavByCode(code string) (*Info, error)
}
