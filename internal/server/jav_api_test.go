package server

import (
	"errors"
	"reflect"
	"testing"

	"javboss/internal/jav"
	"javboss/internal/models"
)

func TestLookupJavSampleImagesByProviderFallsBackFromJavMenuToJavDB(t *testing.T) {
	var calls []jav.Provider
	images, err := lookupJavSampleImagesByProvider("IPX-228", func(code string, provider jav.Provider) (*jav.JavInfo, error) {
		if code != "IPX-228" {
			t.Fatalf("unexpected code: %q", code)
		}
		calls = append(calls, provider)
		switch provider {
		case jav.ProviderJavMenu:
			return &jav.JavInfo{Code: code, Provider: provider}, nil
		case jav.ProviderJavDB:
			return &jav.JavInfo{
				Code:     code,
				Provider: provider,
				SampleImages: []jav.SampleImage{
					{
						ThumbnailURL: "https://example.com/thumb.jpg",
						DetailURL:    "https://example.com/detail.jpg",
					},
				},
			}, nil
		default:
			t.Fatalf("unexpected provider: %s", provider.String())
			return nil, nil
		}
	})
	if err != nil {
		t.Fatalf("lookup sample images: %v", err)
	}
	if !reflect.DeepEqual(calls, []jav.Provider{jav.ProviderJavMenu, jav.ProviderJavDB}) {
		t.Fatalf("provider calls = %#v", calls)
	}
	want := models.JavSampleImages{
		{
			ThumbnailURL: "https://example.com/thumb.jpg",
			DetailURL:    "https://example.com/detail.jpg",
		},
	}
	if !reflect.DeepEqual(images, want) {
		t.Fatalf("sample images = %#v, want %#v", images, want)
	}
}

func TestLookupJavSampleImagesByProviderStopsAfterJavMenuSuccess(t *testing.T) {
	var calls []jav.Provider
	images, err := lookupJavSampleImagesByProvider("IPX-228", func(_ string, provider jav.Provider) (*jav.JavInfo, error) {
		calls = append(calls, provider)
		if provider != jav.ProviderJavMenu {
			return nil, errors.New("JavDB must not be called after JavMenu succeeds")
		}
		return &jav.JavInfo{
			SampleImages: []jav.SampleImage{
				{ThumbnailURL: "thumbnail", DetailURL: "detail"},
			},
		}, nil
	})
	if err != nil {
		t.Fatalf("lookup sample images: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("sample image count = %d, want 1", len(images))
	}
	if !reflect.DeepEqual(calls, []jav.Provider{jav.ProviderJavMenu}) {
		t.Fatalf("provider calls = %#v", calls)
	}
}

func TestLookupJavSampleImagesByProviderPreservesTemporaryErrors(t *testing.T) {
	temporaryErr := errors.New("network timeout")
	images, err := lookupJavSampleImagesByProvider("IPX-228", func(_ string, provider jav.Provider) (*jav.JavInfo, error) {
		switch provider {
		case jav.ProviderJavMenu:
			return nil, temporaryErr
		case jav.ProviderJavDB:
			return nil, jav.ResourceNotFonud
		default:
			t.Fatalf("unexpected provider: %s", provider.String())
			return nil, nil
		}
	})
	if len(images) != 0 {
		t.Fatalf("sample image count = %d, want 0", len(images))
	}
	if !errors.Is(err, temporaryErr) {
		t.Fatalf("lookup error = %v, want network timeout", err)
	}
}

func TestLookupJavSampleImagesByProviderTreatsConfirmedMissAsNotFound(t *testing.T) {
	images, err := lookupJavSampleImagesByProvider("IPX-228", func(_ string, _ jav.Provider) (*jav.JavInfo, error) {
		return nil, jav.ResourceNotFonud
	})
	if err != nil {
		t.Fatalf("confirmed miss returned error: %v", err)
	}
	if len(images) != 0 {
		t.Fatalf("sample image count = %d, want 0", len(images))
	}
}
