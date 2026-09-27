package jav

import (
	"context"
	"fmt"
	"strings"
)

// LookupJavByCode queries the default client.
func LookupJavByCode(ctx context.Context, code string, provider Provider) (*JavInfo, error) {
	return defaultClient.LookupJavByCode(ctx, code, provider)
}

func (c *Client) LookupJavByCode(ctx context.Context, code string, provider Provider) (*JavInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	implementation, err := c.providerFor(provider)
	if err != nil {
		return nil, err
	}
	lookup, ok := implementation.(MovieLookup)
	if !ok {
		return nil, fmt.Errorf("%s: lookup_jav: %w", provider, ErrUnsupportedOperation)
	}
	input := code
	key := lookupCacheKey(provider, "lookup_jav", input)
	if cached, ok, err := lookupCacheGet[JavInfo](c, key); ok {
		return cached, err
	}
	result, err := lookup.LookupJavByCode(ctx, code)
	cacheableLookupResult(c, key, result, err)
	return result, err
}

// LookupActressByCode queries the default client.
func LookupActressByCode(ctx context.Context, code string, provider Provider) (*ActressInfo, error) {
	return defaultClient.LookupActressByCode(ctx, code, provider)
}

func (c *Client) LookupActressByCode(ctx context.Context, code string, provider Provider) (*ActressInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	implementation, err := c.providerFor(provider)
	if err != nil {
		return nil, err
	}
	lookup, ok := implementation.(ActressCodeLookup)
	if !ok {
		return nil, fmt.Errorf("%s: lookup_actress_code: %w", provider, ErrUnsupportedOperation)
	}
	input := code
	key := lookupCacheKey(provider, "lookup_actress_code", input)
	if cached, ok, err := lookupCacheGet[ActressInfo](c, key); ok {
		return cached, err
	}
	result, err := lookup.LookupActressByCode(ctx, code)
	cacheableLookupResult(c, key, result, err)
	return result, err
}

// LookupActressByJapaneseName queries the default client.
func LookupActressByJapaneseName(ctx context.Context, name string, provider Provider) (*ActressInfo, error) {
	return defaultClient.LookupActressByJapaneseName(ctx, name, provider)
}

func (c *Client) LookupActressByJapaneseName(ctx context.Context, name string, provider Provider) (*ActressInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	implementation, err := c.providerFor(provider)
	if err != nil {
		return nil, err
	}
	lookup, ok := implementation.(ActressNameLookup)
	if !ok {
		return nil, fmt.Errorf("%s: lookup_actress_name: %w", provider, ErrUnsupportedOperation)
	}
	input := name
	key := lookupCacheKey(provider, "lookup_actress_name", input)
	if cached, ok, err := lookupCacheGet[ActressInfo](c, key); ok {
		return cached, err
	}
	result, err := lookup.LookupActressByName(ctx, name)
	cacheableLookupResult(c, key, result, err)
	return result, err
}

// LookupActressURLByCodeAndName queries the default client.
func LookupActressURLByCodeAndName(ctx context.Context, code, name string, provider Provider) (string, error) {
	return defaultClient.LookupActressURLByCodeAndName(ctx, code, name, provider)
}

func (c *Client) LookupActressURLByCodeAndName(ctx context.Context, code, name string, provider Provider) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	implementation, err := c.providerFor(provider)
	if err != nil {
		return "", err
	}
	lookup, ok := implementation.(ActressURLLookup)
	if !ok {
		return "", fmt.Errorf("%s: lookup_actress_url_code_name: %w", provider, ErrUnsupportedOperation)
	}
	input := strings.ToUpper(strings.TrimSpace(code)) + "|" + strings.Join(strings.Fields(name), " ")
	key := lookupCacheKey(provider, "lookup_actress_url_code_name", input)
	if cached, ok, err := lookupCacheGet[string](c, key); ok {
		if cached == nil {
			return "", err
		}
		return *cached, err
	}
	result, err := lookup.LookupActressURLByCodeAndName(ctx, code, name)
	cacheableLookupResult(c, key, result, err)
	return result, err
}

// LookupSeriesURLByCode queries the default client.
func LookupSeriesURLByCode(ctx context.Context, code string, provider Provider) (string, error) {
	return defaultClient.LookupSeriesURLByCode(ctx, code, provider)
}

func (c *Client) LookupSeriesURLByCode(ctx context.Context, code string, provider Provider) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	implementation, err := c.providerFor(provider)
	if err != nil {
		return "", err
	}
	lookup, ok := implementation.(SeriesURLLookup)
	if !ok {
		return "", fmt.Errorf("%s: lookup_series_url: %w", provider, ErrUnsupportedOperation)
	}
	input := code
	key := lookupCacheKey(provider, "lookup_series_url", input)
	if cached, ok, err := lookupCacheGet[string](c, key); ok {
		if cached == nil {
			return "", err
		}
		return *cached, err
	}
	result, err := lookup.LookupSeriesURLByCode(ctx, code)
	cacheableLookupResult(c, key, result, err)
	return result, err
}

// LookupStudioURLByCode queries the default client.
func LookupStudioURLByCode(ctx context.Context, code string, provider Provider) (string, error) {
	return defaultClient.LookupStudioURLByCode(ctx, code, provider)
}

func (c *Client) LookupStudioURLByCode(ctx context.Context, code string, provider Provider) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	implementation, err := c.providerFor(provider)
	if err != nil {
		return "", err
	}
	lookup, ok := implementation.(StudioURLLookup)
	if !ok {
		return "", fmt.Errorf("%s: lookup_studio_url: %w", provider, ErrUnsupportedOperation)
	}
	input := code
	key := lookupCacheKey(provider, "lookup_studio_url", input)
	if cached, ok, err := lookupCacheGet[string](c, key); ok {
		if cached == nil {
			return "", err
		}
		return *cached, err
	}
	result, err := lookup.LookupStudioURLByCode(ctx, code)
	cacheableLookupResult(c, key, result, err)
	return result, err
}

// LookupMovieURLByCode resolves a provider's movie page. JavDB may return its search page for ambiguous results.
func (c *Client) LookupMovieURLByCode(ctx context.Context, code string, provider Provider) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	implementation, err := c.providerFor(provider)
	if err != nil {
		return "", err
	}
	lookup, ok := implementation.(MovieURLLookup)
	if !ok {
		return "", fmt.Errorf("%s: movie url: %w", provider, ErrUnsupportedOperation)
	}
	return lookup.LookupMovieURLByCode(ctx, code)
}
func LookupJavDBURLByCode(ctx context.Context, code string) (string, error) {
	return defaultClient.LookupMovieURLByCode(ctx, code, ProviderJavDB)
}
func LookupAvsoxURLByCode(ctx context.Context, code string) (string, error) {
	return defaultClient.LookupMovieURLByCode(ctx, code, ProviderAvsox)
}
func FetchJavBusGenreCategories(ctx context.Context) ([]JavBusGenreCategory, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	implementation, err := defaultClient.providerFor(ProviderJavBus)
	if err != nil {
		return nil, err
	}
	lookup, ok := implementation.(GenreCategoryLookup)
	if !ok {
		return nil, ErrUnsupportedOperation
	}
	return lookup.FetchGenreCategories(ctx)
}
