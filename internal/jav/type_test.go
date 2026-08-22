package jav

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestLookupProvidersByProviderIncludesMetadataProviders(t *testing.T) {
	tests := []Provider{
		ProviderJavBus,
		ProviderJavDatabase,
		ProviderJavDB,
		ProviderAvmoo,
		ProviderAvsox,
		ProviderJavMenu,
		ProviderThePornDB,
		ProviderJavModel,
		ProviderMinnanoAV,
	}

	for _, provider := range tests {
		got, ok := lookupProvidersByProvider[provider]
		if !ok {
			t.Fatalf("lookup provider missing for %s", provider.String())
		}
		if got == nil {
			t.Fatalf("lookup provider for %s is nil", provider.String())
		}
	}
}

func TestManualScrapeProviderIsStableAndNotLookupCapable(t *testing.T) {
	if ProviderManualScrape != Provider(11) {
		t.Fatalf("ProviderManualScrape = %d, want 11", ProviderManualScrape)
	}
	if got := ProviderManualScrape.String(); got != "manual_scrape" {
		t.Fatalf("ProviderManualScrape.String() = %q, want manual_scrape", got)
	}
	if got := ParseProvider(11); got != ProviderManualScrape {
		t.Fatalf("ParseProvider(11) = %s, want manual_scrape", got.String())
	}
	if _, err := lookupProviderFor(ProviderManualScrape); !errors.Is(err, errUnsupportedProvider) {
		t.Fatalf("lookupProviderFor(ProviderManualScrape) error = %v, want unsupported provider", err)
	}
}

func TestJavInfoOmitsUncensoredWhenUnset(t *testing.T) {
	data, err := json.Marshal(JavInfo{Code: "ABC-001", Provider: ProviderAvmoo})
	if err != nil {
		t.Fatalf("marshal jav info: %v", err)
	}
	if strings.Contains(string(data), "IsUncensored") {
		t.Fatalf("unexpected IsUncensored field: %s", data)
	}
}
