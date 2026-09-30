// Package avwiki retrieves actress profiles from AV Wiki's public WordPress tags API.
package avwiki

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/jav/internal/ratelimit"
	"javboss/internal/jav/metadata"
)

const baseURL = "https://av-wiki.net"
const maxResponseBytes = 2 << 20

type AVWikiClient struct {
	httpClient *http.Client
	baseURL    string
	limiter    *ratelimit.Limiter
}

// New creates a provider using the supplied non-nil HTTP client.
func New(httpClient *http.Client) *AVWikiClient {
	return &AVWikiClient{httpClient: httpClient, baseURL: baseURL, limiter: ratelimit.New(time.Second)}
}

type actressTag struct {
	Name        string `json:"name"`
	Link        string `json:"link"`
	Description string `json:"description"`
}

// LookupActressByName accepts only exact tag names. WordPress's tag search does
// not search profile descriptions, so an alias in a description may not be found.
func (p *AVWikiClient) LookupActressByName(ctx context.Context, name string) (*metadata.ActressInfo, error) {
	name = normalizeName(name)
	if name == "" {
		return nil, metadata.ErrNotFound
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var result *metadata.ActressInfo
	for page := 1; ; page++ {
		query := url.Values{
			"search": {name}, "per_page": {"100"}, "page": {strconv.Itoa(page)},
			"_fields": {"name,link,description"},
		}
		tags, pages, err := p.fetchTags(ctx, query)
		if err != nil {
			return nil, err
		}
		for _, tag := range tags {
			if normalizeName(tag.Name) != name {
				continue
			}
			info := parseActress(tag, p.baseURL)
			if info == nil {
				continue
			}
			if result != nil && result.ProfileURL != info.ProfileURL {
				return nil, fmt.Errorf("avwiki: ambiguous actress name %q", name)
			}
			result = info
		}
		if page >= pages {
			break
		}
	}
	if result == nil {
		return nil, metadata.ErrNotFound
	}
	return result, nil
}

func (p *AVWikiClient) fetchTags(ctx context.Context, query url.Values) ([]actressTag, int, error) {
	resp, err := p.request(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	logging.Info("avwiki response status: %s", resp.Status)
	if resp.Header.Get("cf-mitigated") == "challenge" {
		return nil, 0, fmt.Errorf("avwiki: http %d: cloudflare browser verification required", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		// An unavailable API must not become a cached actress-not-found result.
		return nil, 0, fmt.Errorf("avwiki: http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, 0, fmt.Errorf("avwiki: read tags: %w", err)
	}
	if len(body) > maxResponseBytes {
		return nil, 0, fmt.Errorf("avwiki: tags response too large")
	}
	var tags []actressTag
	if err := json.Unmarshal(body, &tags); err != nil {
		return nil, 0, fmt.Errorf("avwiki: decode tags: %w", err)
	}
	if tags == nil {
		return nil, 0, fmt.Errorf("avwiki: expected tags array")
	}
	pages := 1
	if raw := resp.Header.Get("X-WP-TotalPages"); raw != "" {
		pages, err = strconv.Atoi(raw)
		if err != nil || pages < 0 {
			return nil, 0, fmt.Errorf("avwiki: invalid pagination header")
		}
	}
	return tags, pages, nil
}

func (p *AVWikiClient) request(ctx context.Context, query url.Values) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/wp-json/wp/v2/tags?"+query.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("avwiki: build request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "ja-JP,ja;q=0.9,en;q=0.7")
	if err := p.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	logging.Info("avwiki request: %s", req.URL)
	return p.httpClient.Do(req)
}

func (p *AVWikiClient) OriginURL() string { return p.baseURL }

func normalizeName(value string) string { return strings.Join(strings.Fields(value), "") }
