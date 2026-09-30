package metadata

// Provider identifies where JAV metadata or tags came from.
type Provider int

// Values are persisted in the database; never renumber existing providers.
const (
	ProviderUnknown      Provider = 0
	ProviderJavBus       Provider = 1
	ProviderJavDatabase  Provider = 2
	ProviderUser         Provider = 3
	ProviderJavDB        Provider = 4
	ProviderAvmoo        Provider = 5
	ProviderThePornDB    Provider = 6
	ProviderJavModel     Provider = 7
	ProviderAvsox        Provider = 8
	ProviderJavMenu      Provider = 9
	ProviderMinnanoAV    Provider = 10
	ProviderManualScrape Provider = 11
	ProviderJavDBAPI     Provider = 12
	ProviderAVWiki       Provider = 13
)

func (p Provider) String() string {
	switch p {
	case ProviderJavBus:
		return "javbus"
	case ProviderJavDatabase:
		return "javdatabase"
	case ProviderUser:
		return "user"
	case ProviderJavDBAPI:
		return "javdb-api"
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
	case ProviderAVWiki:
		return "avwiki"
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
	case ProviderJavBus, ProviderJavDatabase, ProviderUser, ProviderJavDB, ProviderAvmoo, ProviderThePornDB, ProviderJavModel, ProviderAvsox, ProviderJavMenu, ProviderMinnanoAV, ProviderManualScrape, ProviderJavDBAPI, ProviderAVWiki:
		return p
	default:
		return ProviderUnknown
	}
}
