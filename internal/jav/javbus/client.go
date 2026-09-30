package javbus

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/jav/internal/parseutil"
	"javboss/internal/jav/internal/ratelimit"
	"javboss/internal/jav/metadata"
)

// JavBusClient retrieves metadata from javbus.
type JavBusClient struct {
	httpClient *http.Client
	limiter    *ratelimit.Limiter
}

const javBusRequestInterval = 500 * time.Millisecond

// LookupJavByCode fetches metadata for a given code.
func (p *JavBusClient) LookupJavByCode(ctx context.Context, code string) (*metadata.JavInfo, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, metadata.ErrNotFound
	}
	lookupCode, rewrite := javBusLookupCode(code)
	logging.Info("javbus: code -> %s", lookupCode)

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	info, err := p.fetchInfo(ctx, lookupCode)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}
	if info.Code == "" {
		info.Code = lookupCode
	}
	if rewrite != nil {
		normalizeJavBusRewrittenInfo(info, rewrite)
	}
	return info, nil
}

func (p *JavBusClient) fetchInfo(ctx context.Context, code string) (*metadata.JavInfo, error) {
	doc, url, err := p.fetchJavBusDocument(ctx, code)
	if err != nil {
		return nil, err
	}

	info := parseDocument(doc)
	if info == nil {
		logging.Info("javbus: parseDocument returned nil")
		return nil, metadata.ErrNotFound
	}
	info.CoverURL = parseJavBusCoverURL(doc, url)
	info.SampleImages = parseutil.ParseSampleImages(doc, url)
	if info.Code == "" || info.Title == "" {
		logging.Info("javbus: parsed title/code empty (title=%q code=%q)", info.Title, info.Code)
		return nil, metadata.ErrNotFound
	}
	logging.Info("javbus parsed from %s: title=%q tags=%d actors=%d", url, info.Title, len(info.Tags), len(info.Actors))
	return info, nil
}

var javBusCodeRewrites = []javBusCodeRewrite{
	{inputPrefix: "gana", requestPrefix: "200gana"},
	{inputPrefix: "mium", requestPrefix: "300mium"},
	{inputPrefix: "luxu", requestPrefix: "259luxu"},
}

// FetchGenreCategories loads the censored and uncensored JavBus genre
// indexes and returns the category assigned to each label by JavBus.
func (p *JavBusClient) FetchGenreCategories(ctx context.Context) ([]metadata.GenreCategory, error) {
	pages := []struct {
		url        string
		pathPrefix string
	}{
		{url: "https://www.javbus.com/genre", pathPrefix: "/genre/"},
		{url: "https://www.javbus.com/uncensored/genre", pathPrefix: "/uncensored/genre/"},
	}

	seen := make(map[string]struct{})
	genres := make([]metadata.GenreCategory, 0, 256)
	for _, page := range pages {
		doc, err := p.fetchJavBusGenreDocument(ctx, page.url)
		if err != nil {
			return nil, err
		}
		for _, genre := range parseJavBusGenreCategories(doc, page.pathPrefix) {
			key := genre.Name + "\x00" + genre.Category
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			genres = append(genres, genre)
		}
	}
	if len(genres) == 0 {
		return nil, errors.New("javbus: genre pages did not contain any categories")
	}
	return genres, nil
}

// New creates a provider using the supplied non-nil HTTP client.
func New(httpClient *http.Client) *JavBusClient {
	return &JavBusClient{httpClient: httpClient, limiter: ratelimit.New(javBusRequestInterval)}
}
