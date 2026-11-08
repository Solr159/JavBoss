// Package subtitle coordinates subtitle sources without depending on a player or database.
package subtitle

import (
	"context"
	"path/filepath"
)

// Provider IDs must be stable: they identify subtitle provenance.
// Candidate and Track IDs are opaque to the coordinator and owned by the provider.
// Providers return only existing subtitles; they must not start remote translation jobs.
// Search returns candidates in the provider's preferred order, best first.
type Provider interface {
	ID() string
	Search(context.Context, Query) ([]Candidate, error)
	Tracks(context.Context, Candidate) ([]Track, error)
	Download(context.Context, Track) ([]byte, error)
}

type Query struct {
	Code string
}

type Candidate struct {
	ID    string
	Code  string
	Title string
}

type Track struct {
	ID       string
	Language string // BCP 47, e.g. zh-CN.
	Format   string // Currently srt; other formats must be normalized by their adapter.
}

type Request struct {
	Query
	CacheDir  string
	Languages []string // Ordered preference, independent of provider order.
}

type Result struct {
	Downloaded  bool   `json:"-"` // True only when this resolve produced a fresh download.
	Path        string `json:"-"`
	Source      string `json:"source"`
	CandidateID string `json:"candidate_id"`
	TrackID     string `json:"track_id"`
	Language    string `json:"language"`
}

// SelectionKey is stable across players and cache refreshes. Local sidecars
// identify a file; downloaded tracks identify the preferred language copy.
func (r *Result) SelectionKey() string {
	if r.Source == "local" {
		return "local:" + filepath.Base(r.Path)
	}
	return "download:" + r.Language
}
