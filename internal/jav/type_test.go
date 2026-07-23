package jav

import (
	"encoding/json"
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

func TestJavInfoOmitsUncensoredWhenUnset(t *testing.T) {
	data, err := json.Marshal(JavInfo{Code: "ABC-001", Provider: ProviderAvmoo})
	if err != nil {
		t.Fatalf("marshal jav info: %v", err)
	}
	if strings.Contains(string(data), "IsUncensored") {
		t.Fatalf("unexpected IsUncensored field: %s", data)
	}
}
