package metadata

import "testing"

func TestPersistedProviderIdentities(t *testing.T) {
	for _, tc := range []struct {
		provider Provider
		id       int
		name     string
	}{
		{ProviderUnknown, 0, "unknown"},
		{ProviderJavBus, 1, "javbus"},
		{ProviderJavDatabase, 2, "javdatabase"},
		{ProviderUser, 3, "user"},
		{ProviderJavDB, 4, "javdb"},
		{ProviderAvmoo, 5, "avmoo"},
		{ProviderThePornDB, 6, "theporndb"},
		{ProviderJavModel, 7, "javmodel"},
		{ProviderAvsox, 8, "avsox"},
		{ProviderJavMenu, 9, "javmenu"},
		{ProviderMinnanoAV, 10, "minnanoav"},
		{ProviderManualScrape, 11, "manual_scrape"},
		{ProviderJavDBAPI, 12, "javdb-api"},
		{ProviderAVWiki, 13, "avwiki"},
	} {
		if int(tc.provider) != tc.id || tc.provider.String() != tc.name || ParseProvider(tc.id) != tc.provider {
			t.Errorf("provider %s no longer matches persisted identity %d/%s", tc.provider, tc.id, tc.name)
		}
	}
	for _, id := range []int{-1, 14, 100} {
		if ParseProvider(id) != ProviderUnknown {
			t.Errorf("unknown id %d accepted", id)
		}
	}
}
