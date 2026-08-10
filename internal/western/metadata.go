package western

import (
	"encoding/xml"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

var ErrNotFound = errors.New("western metadata not found")

// Metadata is the provider-neutral representation persisted for a video.
type Metadata struct {
	Title         string   `json:"title"`
	OriginalTitle string   `json:"original_title"`
	ContentType   string   `json:"content_type"`
	MatchStatus   string   `json:"match_status"`
	Studio        string   `json:"studio"`
	Description   string   `json:"description"`
	ReleaseDate   string   `json:"release_date"`
	Source        string   `json:"source"`
	SourceID      string   `json:"source_id"`
	SourceURL     string   `json:"source_url"`
	CoverURL      string   `json:"cover_url"`
	Performers    []string `json:"performers"`
	Genres        []string `json:"genres"`
	Labels        []string `json:"labels"`
}

type nfoMovie struct {
	Generator     string      `xml:"generator"`
	Title         string      `xml:"title"`
	OriginalTitle string      `xml:"originaltitle"`
	Plot          string      `xml:"plot"`
	Outline       string      `xml:"outline"`
	Premiered     string      `xml:"premiered"`
	ReleaseDate   string      `xml:"releasedate"`
	Studio        string      `xml:"studio"`
	Genres        []string    `xml:"genre"`
	Tags          []string    `xml:"tag"`
	Actors        []nfoActor  `xml:"actor"`
	UniqueIDs     []nfoUnique `xml:"uniqueid"`
}

type nfoActor struct {
	Name string `xml:"name"`
}

type nfoUnique struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

// ReadNFO reads the Emby/Jellyfin sidecar next to a video. JavBoss-generated
// JAV sidecars are ignored so the two metadata systems remain independent.
func ReadNFO(videoPath string) (*Metadata, error) {
	videoPath = filepath.Clean(strings.TrimSpace(videoPath))
	if videoPath == "" || videoPath == "." {
		return nil, ErrNotFound
	}
	nfoPath := strings.TrimSuffix(videoPath, filepath.Ext(videoPath)) + ".nfo"
	data, err := os.ReadFile(nfoPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	var movie nfoMovie
	if err := xml.Unmarshal(data, &movie); err != nil {
		return nil, err
	}
	if strings.EqualFold(strings.TrimSpace(movie.Generator), "JavBoss") {
		ownedProvider := false
		for _, id := range movie.UniqueIDs {
			if strings.EqualFold(strings.TrimSpace(id.Type), "theporndb") {
				ownedProvider = true
				break
			}
		}
		if !ownedProvider {
			return nil, ErrNotFound
		}
	}

	result := &Metadata{
		Title:         strings.TrimSpace(movie.Title),
		OriginalTitle: strings.TrimSpace(movie.OriginalTitle),
		ContentType:   "movie",
		MatchStatus:   "matched",
		Studio:        strings.TrimSpace(movie.Studio),
		Description:   firstNonEmpty(movie.Plot, movie.Outline),
		ReleaseDate:   firstNonEmpty(movie.Premiered, movie.ReleaseDate),
		Source:        "nfo",
		Genres:        cleanList(movie.Genres),
		Labels:        cleanList(movie.Tags),
	}
	for _, actor := range movie.Actors {
		result.Performers = append(result.Performers, actor.Name)
	}
	result.Performers = cleanList(result.Performers)
	for _, id := range movie.UniqueIDs {
		if value := strings.TrimSpace(id.Value); value != "" {
			result.SourceID = value
			if provider := strings.ToLower(strings.TrimSpace(id.Type)); provider != "" {
				result.Source = "nfo:" + provider
			}
			break
		}
	}
	if result.Title == "" && result.OriginalTitle == "" && result.SourceID == "" {
		return nil, ErrNotFound
	}
	return result, nil
}

func cleanList(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if value == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
