// Package subtitle coordinates subtitle sources without depending on a player or database.
package subtitle

import (
	"context"
	"path/filepath"
)

// Provider IDs identify sources in diagnostics.
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

const (
	SourceLocal    = "local"
	SourceDownload = "download"
)

type Result struct {
	Downloaded bool // True only when this resolve produced a fresh download.
	Path       string
	Source     string // SourceLocal or SourceDownload, independent of the provider.
	Language   string
}

// SelectionKey is stable across players and cache refreshes. Local sidecars
// identify a file; downloaded tracks identify the preferred language copy.
func (r *Result) SelectionKey() string {
	if r.Source == SourceLocal {
		return "local:" + filepath.Base(r.Path)
	}
	return "download:" + r.Language
}
