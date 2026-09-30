package jav

import (
	"context"

	"javboss/internal/jav/metadata"
)

// MovieLookup is implemented only by providers supporting this operation.
type MovieLookup interface {
	LookupJavByCode(ctx context.Context, code string) (*metadata.JavInfo, error)
}

// ActressCodeLookup is implemented only by providers supporting this operation.
type ActressCodeLookup interface {
	LookupActressByCode(ctx context.Context, code string) (*metadata.ActressInfo, error)
}

// ActressNameLookup is implemented only by providers supporting this operation.
type ActressNameLookup interface {
	LookupActressByName(ctx context.Context, name string) (*metadata.ActressInfo, error)
}

// ActressURLLookup is implemented only by providers supporting this operation.
type ActressURLLookup interface {
	LookupActressURLByCodeAndName(ctx context.Context, code, name string) (string, error)
}

// SeriesURLLookup is implemented only by providers supporting this operation.
type SeriesURLLookup interface {
	LookupSeriesURLByCode(ctx context.Context, code string) (string, error)
}

// StudioURLLookup is implemented only by providers supporting this operation.
type StudioURLLookup interface {
	LookupStudioURLByCode(ctx context.Context, code string) (string, error)
}

type MovieURLLookup interface {
	LookupMovieURLByCode(context.Context, string) (string, error)
}
type GenreCategoryLookup interface {
	FetchGenreCategories(context.Context) ([]metadata.GenreCategory, error)
}

// Capabilities describes the operations implemented by a registered provider.
type Capabilities struct {
	Movie, ActressByCode, ActressByName bool
	ActressURL, SeriesURL, StudioURL    bool
	MovieURL, GenreCategories           bool
}

// CapabilitiesFor reports supported operations without making network requests.
// Unknown providers and non-lookup sources have no capabilities.
func CapabilitiesFor(provider Provider) Capabilities {
	return defaultMetadataClient.CapabilitiesFor(provider)
}

func (c *MetadataClient) CapabilitiesFor(provider Provider) Capabilities {
	implementation, err := c.providerFor(provider)
	if err != nil {
		return Capabilities{}
	}
	var result Capabilities
	_, result.Movie = implementation.(MovieLookup)
	_, result.ActressByCode = implementation.(ActressCodeLookup)
	_, result.ActressByName = implementation.(ActressNameLookup)
	_, result.ActressURL = implementation.(ActressURLLookup)
	_, result.SeriesURL = implementation.(SeriesURLLookup)
	_, result.StudioURL = implementation.(StudioURLLookup)
	_, result.MovieURL = implementation.(MovieURLLookup)
	_, result.GenreCategories = implementation.(GenreCategoryLookup)
	return result
}
