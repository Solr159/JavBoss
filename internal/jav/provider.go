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
