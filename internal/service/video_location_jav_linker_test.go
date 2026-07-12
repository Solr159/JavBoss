package service

import (
	"reflect"
	"testing"

	"javboss/internal/jav"
)

func TestJavScrapeCodesForVideoUsesForcedCodeOnly(t *testing.T) {
	got := javScrapeCodesForVideo("ABC-001 DEF-002.mp4", "XYZ-999")
	want := []string{"XYZ-999"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("javScrapeCodesForVideo() = %#v, want %#v", got, want)
	}
}

func TestJavLinkProvidersUsesJavBusInChineseMode(t *testing.T) {
	prevLang := jav.CurrentMetadataLanguage()
	t.Cleanup(func() { jav.SetMetadataLanguage(string(prevLang)) })
	jav.SetMetadataLanguage("zh")

	got := javLinkProviders()
	want := []jav.Provider{jav.ProviderJavBus}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("javLinkProviders() = %#v, want %#v", got, want)
	}
}

func TestJavLinkProvidersUsesJavDatabaseInEnglishMode(t *testing.T) {
	prevLang := jav.CurrentMetadataLanguage()
	t.Cleanup(func() { jav.SetMetadataLanguage(string(prevLang)) })
	jav.SetMetadataLanguage("en")

	got := javLinkProviders()
	want := []jav.Provider{jav.ProviderJavDatabase}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("javLinkProviders() = %#v, want %#v", got, want)
	}
}

func TestForcedJavScrapeCodeSupportsManualOverride(t *testing.T) {
	got := forcedJavScrapeCode(":manual:abc-001")
	if got != "ABC-001" {
		t.Fatalf("forcedJavScrapeCode() = %q, want ABC-001", got)
	}
}
