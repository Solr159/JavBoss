package western

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"javboss/internal/util"
)

const thePornDBBaseURL = "https://api.theporndb.net"

var ErrThePornDBUnauthorized = errors.New("theporndb token is invalid or unauthorized")

// SearchOptions mirrors the matching inputs used by the Emby provider.
type SearchOptions struct {
	Query string
	Hash  string
}

// SearchThePornDB searches Western scenes using ThePornDB's filename parser.
func SearchThePornDB(ctx context.Context, token, query string) ([]Metadata, error) {
	return SearchThePornDBWithOptions(ctx, token, SearchOptions{Query: query})
}

// SearchThePornDBWithOptions searches by parsed filename and, when available,
// the OpenSubtitles hash used by Emby-compatible providers.
func SearchThePornDBWithOptions(ctx context.Context, token string, options SearchOptions) ([]Metadata, error) {
	token = strings.TrimSpace(token)
	query := strings.TrimSpace(options.Query)
	if token == "" {
		return nil, errors.New("theporndb token is not configured")
	}
	if query == "" {
		return nil, errors.New("search query is required")
	}

	requestCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	params := url.Values{}
	params.Set("parse", query)
	if hash := strings.TrimSpace(options.Hash); hash != "" {
		params.Set("hash", hash)
	}
	params.Set("per_page", "12")
	params.Set("orderBy", "most_relevant")
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, thePornDBBaseURL+"/scenes?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; JavBoss/1.0)")

	resp, err := util.DoRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return []Metadata{}, nil
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrThePornDBUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("theporndb: http %d", resp.StatusCode)
	}

	var payload thePornDBResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4*1024*1024)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("theporndb: decode response: %w", err)
	}
	result := make([]Metadata, 0, len(payload.Data))
	for _, item := range payload.Data {
		if metadata := item.metadata(); metadata != nil {
			result = append(result, *metadata)
		}
	}
	return result, nil
}

type thePornDBResponse struct {
	Data []thePornDBScene `json:"data"`
}

type thePornDBScene struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Type        string `json:"type"`
	ExternalID  string `json:"external_id"`
	Description string `json:"description"`
	Date        string `json:"date"`
	URL         string `json:"url"`
	Image       string `json:"image"`
	Poster      string `json:"poster"`
	Background  struct {
		Full string `json:"full"`
	} `json:"background"`
	Performers []struct {
		Name   string `json:"name"`
		Parent *struct {
			Name     string `json:"name"`
			FullName string `json:"full_name"`
		} `json:"parent"`
	} `json:"performers"`
	Site *thePornDBSite `json:"site"`
	Tags []struct {
		Name string `json:"name"`
	} `json:"tags"`
}

func (item thePornDBScene) metadata() *Metadata {
	result := &Metadata{
		Title:       strings.TrimSpace(item.Title),
		ContentType: strings.ToLower(strings.TrimSpace(item.Type)),
		MatchStatus: "matched",
		Description: strings.TrimSpace(item.Description),
		ReleaseDate: strings.TrimSpace(item.Date),
		Source:      "theporndb",
		SourceID:    strings.TrimSpace(item.ID),
		SourceURL:   strings.TrimSpace(item.URL),
		CoverURL:    firstNonEmpty(item.Background.Full, item.Image, item.Poster),
	}
	if result.ContentType == "" {
		result.ContentType = "scene"
	}
	if item.Site != nil {
		result.Studio = firstNonEmpty(item.Site.Name, item.Site.Network.Name, item.Site.Parent.Name)
	}
	for _, performer := range item.Performers {
		name := strings.TrimSpace(performer.Name)
		if performer.Parent != nil {
			name = firstNonEmpty(performer.Parent.FullName, performer.Parent.Name, name)
		}
		result.Performers = append(result.Performers, name)
	}
	for _, tag := range item.Tags {
		result.Labels = append(result.Labels, tag.Name)
	}
	result.Performers = cleanList(result.Performers)
	result.Labels = cleanList(result.Labels)
	result.Genres = append([]string(nil), result.Labels...)
	if result.Title == "" || result.SourceID == "" {
		return nil
	}
	return result
}

type thePornDBSite struct {
	Name    string `json:"name"`
	Network struct {
		Name string `json:"name"`
	} `json:"network"`
	Parent struct {
		Name string `json:"name"`
	} `json:"parent"`
}
