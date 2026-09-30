package javdb

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"javboss/internal/jav/internal/parseutil"
	"javboss/internal/jav/internal/ratelimit"
	"javboss/internal/jav/metadata"
)

// JavDBClient retrieves metadata from javdb.
type JavDBClient struct {
	httpClient *http.Client
	limiter    *ratelimit.Limiter
}

const (
	javDBBaseURL         = "https://javdb.com"
	javDBUserAgent       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	javDBRequestInterval = 500 * time.Millisecond
)

// LookupActressURLByCodeAndName resolves an actress profile URL from a movie detail page.
func (p *JavDBClient) LookupActressURLByCodeAndName(ctx context.Context, code, name string) (string, error) {
	code = strings.TrimSpace(code)
	name = strings.TrimSpace(name)
	if code == "" || name == "" {
		return "", metadata.ErrNotFound
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	doc, detailURL, err := p.fetchJavDBDetailByCode(ctx, code)
	if err != nil && !errors.Is(err, metadata.ErrNotFound) {
		return "", err
	}
	if err == nil {
		if actressURL := parseJavDBActressURLByName(doc, name, detailURL); actressURL != "" {
			return actressURL, nil
		}
	}
	return p.lookupJavDBActressURLByName(ctx, name)
}

// LookupSeriesURLByCode resolves a series detail URL from a movie detail page.
func (p *JavDBClient) LookupSeriesURLByCode(ctx context.Context, code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", metadata.ErrNotFound
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	doc, detailURL, err := p.fetchJavDBDetailByCode(ctx, code)
	if err != nil {
		return "", err
	}
	seriesURL := parseJavDBSeriesURL(doc, detailURL)
	if seriesURL == "" {
		return "", metadata.ErrNotFound
	}
	return seriesURL, nil
}

// LookupStudioURLByCode resolves a studio detail URL from a movie detail page.
func (p *JavDBClient) LookupStudioURLByCode(ctx context.Context, code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", metadata.ErrNotFound
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	doc, detailURL, err := p.fetchJavDBDetailByCode(ctx, code)
	if err != nil {
		return "", err
	}
	studioURL := parseJavDBStudioURL(doc, detailURL)
	if studioURL == "" {
		return "", metadata.ErrNotFound
	}
	return studioURL, nil
}

// LookupJavByCode fetches metadata for a given code.
func (p *JavDBClient) LookupJavByCode(ctx context.Context, code string) (*metadata.JavInfo, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, metadata.ErrNotFound
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	doc, detailURL, err := p.fetchJavDBDetailByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	info := parseJavDBMovieInfo(doc)
	if info == nil {
		return nil, metadata.ErrNotFound
	}
	if info.Code == "" {
		info.Code = code
	}
	info.CoverURL = parseJavDBCoverURL(doc, detailURL)
	info.SampleImages = parseutil.ParseSampleImages(doc, detailURL)
	return info, nil
}

// LookupMovieURLByCode resolves a movie code to a JavDB detail URL when the
// search results contain exactly one precise code match. Ambiguous or missing
// precise matches return the search URL so the user can choose manually.
func (p *JavDBClient) LookupMovieURLByCode(ctx context.Context, code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", metadata.ErrNotFound
	}

	searchURL := javDBSearchURL(code)
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	searchDoc, status, err := p.fetchJavDBHTML(ctx, searchURL, javDBBaseURL)
	if err != nil {
		return "", err
	}
	if status == http.StatusNotFound || searchDoc == nil {
		return searchURL, nil
	}

	detailURL := findSingleJavDBSearchResultURL(searchDoc, code, searchURL)
	if detailURL == "" {
		return searchURL, nil
	}
	return detailURL, nil
}

func (p *JavDBClient) lookupJavDBActressURLByName(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", metadata.ErrNotFound
	}

	searchURL := javDBActorSearchURL(name)
	doc, status, err := p.fetchJavDBHTML(ctx, searchURL, javDBBaseURL)
	if err != nil {
		return "", err
	}
	if status == http.StatusNotFound || doc == nil {
		return "", metadata.ErrNotFound
	}

	urls := findJavDBActorSearchResultURLs(doc, name, searchURL)
	switch len(urls) {
	case 0:
		return "", metadata.ErrNotFound
	case 1:
		return urls[0], nil
	default:
		return searchURL, nil
	}
}

// New creates a provider using the supplied non-nil HTTP client.
func New(httpClient *http.Client) *JavDBClient {
	return &JavDBClient{httpClient: httpClient, limiter: ratelimit.New(javDBRequestInterval)}
}
