package service

import (
	"reflect"
	"testing"

	"javboss/internal/jav"
)

func TestJavMetadataFastZhProvidersExcludeSlowProviders(t *testing.T) {
	got := javFastZhMetadataProviders()
	want := []jav.Provider{jav.ProviderJavBus}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("javFastZhMetadataProviders() = %#v, want %#v", got, want)
	}
}
