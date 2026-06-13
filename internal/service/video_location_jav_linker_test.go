package service

import (
	"reflect"
	"testing"

	"pornboss/internal/jav"
)

func TestJavScrapeCodesForVideoUsesForcedCodeOnly(t *testing.T) {
	got := javScrapeCodesForVideo("ABC-001 DEF-002.mp4", "XYZ-999")
	want := []string{"XYZ-999"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("javScrapeCodesForVideo() = %#v, want %#v", got, want)
	}
}

func TestJavLinkProvidersExcludesJavDB(t *testing.T) {
	got := javLinkProviders()
	want := []jav.Provider{jav.ProviderJavBus, jav.ProviderJavDatabase}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("javLinkProviders() = %#v, want %#v", got, want)
	}
}
