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

func TestJavLinkProvidersForCode(t *testing.T) {
	tests := []struct {
		name string
		code string
		want []jav.Provider
	}{
		{
			name: "gana prefers javmenu",
			code: "gana-1234",
			want: []jav.Provider{jav.ProviderJavMenu, jav.ProviderJavBus},
		},
		{
			name: "stars prefers javbus",
			code: " STARS-001 ",
			want: []jav.Provider{jav.ProviderJavBus, jav.ProviderAvmoo},
		},
		{
			name: "ap uses avmoo only",
			code: "ap-001",
			want: []jav.Provider{jav.ProviderAvmoo},
		},
		{
			name: "other uses javbus only",
			code: "IPX-228",
			want: []jav.Provider{jav.ProviderJavBus},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := javLinkProvidersForCode(tt.code)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("javLinkProvidersForCode(%q) = %#v, want %#v", tt.code, got, tt.want)
			}
		})
	}
}

func TestForcedJavScrapeCodeSupportsManualOverride(t *testing.T) {
	got := forcedJavScrapeCode(":manual:abc-001")
	if got != "ABC-001" {
		t.Fatalf("forcedJavScrapeCode() = %q, want ABC-001", got)
	}
}
