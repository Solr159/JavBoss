package javdatabase

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/jav/internal/parseutil"
	"javboss/internal/jav/internal/ratelimit"
	"javboss/internal/jav/metadata"
)

// Client retrieves metadata from javdatabase.
type Client struct {
	limiter *ratelimit.Limiter
}

var errNoActressLink = errors.New("javdatabase: actress link not found")

const javDatabaseRequestInterval = 500 * time.Millisecond

// LookupActressByCode resolves a solo movie code to its actress profile.
func (p *Client) LookupActressByCode(ctx context.Context, code string) (*metadata.ActressInfo, error) {
	return p.lookupActressByCode(ctx, code)
}

// LookupJavByCode fetches metadata for a given code.
func (p *Client) LookupJavByCode(ctx context.Context, code string) (*metadata.JavInfo, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, metadata.ErrNotFound
	}

	base := "https://www.javdatabase.com"
	movieURL := fmt.Sprintf("%s/movies/%s/", base, code)

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	doc, status, err := p.fetchJavDatabaseHTML(ctx, movieURL, base)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound || doc == nil {
		return nil, metadata.ErrNotFound
	}

	info := parseJavDatabaseMovieInfo(doc)
	if info == nil {
		return nil, metadata.ErrNotFound
	}
	if info.Code != "" && normalizeJavDatabaseCode(info.Code) != normalizeJavDatabaseCode(code) {
		logging.Info("javdatabase: requested code %s resolved to %s", code, info.Code)
		return nil, metadata.ErrNotFound
	}
	if info.Code == "" {
		info.Code = code
	}
	info.CoverURL = parseJavDatabaseCoverURL(doc, movieURL)
	info.SampleImages = parseutil.ParseSampleImages(doc, movieURL)
	return info, nil
}

func (p *Client) lookupActressByCode(ctx context.Context, code string) (*metadata.ActressInfo, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, metadata.ErrNotFound
	}

	base := "https://www.javdatabase.com"

	movieURL := fmt.Sprintf("%s/movies/%s", base, code)

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	doc, status, err := p.fetchJavDatabaseHTML(ctx, movieURL, base)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound || doc == nil {
		return nil, metadata.ErrNotFound
	}

	actressLink, err := findJavDatabaseActressLink(doc)
	if err != nil {
		if errors.Is(err, errNoActressLink) {
			return nil, metadata.ErrNotFound
		}
		return nil, err
	}
	if actressLink == "" {
		return nil, metadata.ErrNotFound
	}
	actressURL := parseutil.ResolveURL(movieURL, actressLink)
	if actressURL == "" {
		return nil, metadata.ErrNotFound
	}

	actressDoc, status, err := p.fetchJavDatabaseHTML(ctx, actressURL, movieURL)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound || actressDoc == nil {
		return nil, metadata.ErrNotFound
	}

	info := parseJavDatabaseActressInfo(actressDoc)
	if info == nil {
		return nil, metadata.ErrNotFound
	}
	info.ProfileURL = actressURL
	return info, nil
}

// New creates an independent provider client.
func New() *Client { return &Client{limiter: ratelimit.New(javDatabaseRequestInterval)} }
