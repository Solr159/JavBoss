package subtitle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

var ErrNotFound = errors.New("subtitle not found")

type failure struct {
	err   error
	until time.Time
}

type Service struct {
	providers []Provider
	flights   singleflight.Group
	mu        sync.Mutex
	failures  map[string]failure
	gate      chan struct{}
}

func NewService(providers ...Provider) *Service {
	return &Service{providers: append([]Provider(nil), providers...), failures: make(map[string]failure), gate: make(chan struct{}, 2)}
}

func NormalizeCode(code string) string {
	return strings.ToUpper(strings.NewReplacer("-", "", "_", "", " ", "").Replace(strings.TrimSpace(code)))
}

// Cached checks every preferred language on disk without contacting providers
// or waiting for an in-flight download. A miss does not populate the failure cache.
func (s *Service) Cached(ctx context.Context, req Request) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if req.Code == "" || len(req.Languages) == 0 {
		return nil, ErrNotFound
	}
	for _, language := range req.Languages {
		name, err := FileName(strings.ToUpper(strings.TrimSpace(req.Code)), language)
		if err != nil {
			return nil, err
		}
		if result := cached(filepath.Join(req.CacheDir, strings.TrimSuffix(name, ".srt"))); result != nil {
			return result, nil
		}
	}
	return nil, ErrNotFound
}

// Resolve shares CODE.language.srt across all videos with the same code.
// Each language is looked up separately, preserving preference even when a
// lower-priority language is already cached. In-flight work and failures are
// shared by code/language too, so different videos never download it twice.
// A caller leaving playback cancels its wait, not a shared download needed by
// another player. Shared work has its own bounded lifetime.
func (s *Service) Resolve(ctx context.Context, req Request) (*Result, error) {
	if req.Code == "" || len(req.Languages) == 0 {
		return nil, ErrNotFound
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	req.Code = strings.ToUpper(strings.TrimSpace(req.Code))
	var errs []error
	for _, language := range req.Languages {
		name, err := FileName(req.Code, language)
		if err != nil {
			return nil, err
		}
		language = strings.TrimSuffix(strings.TrimPrefix(name, req.Code+"."), ".srt")
		one := req
		one.Languages = []string{language}
		base := filepath.Join(req.CacheDir, strings.TrimSuffix(name, ".srt"))
		result, err := s.resolveLanguage(ctx, one, base)
		if err == nil {
			return result, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !errors.Is(err, ErrNotFound) {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return nil, ErrNotFound
}

func (s *Service) resolveLanguage(ctx context.Context, req Request, base string) (*Result, error) {
	if result := cached(base); result != nil {
		return result, nil
	}
	s.mu.Lock()
	for key, f := range s.failures {
		if time.Now().After(f.until) {
			delete(s.failures, key)
		}
	}
	f, failed := s.failures[base]
	s.mu.Unlock()
	if failed {
		return nil, f.err
	}
	ch := s.flights.DoChan(base, func() (any, error) {
		if result := cached(base); result != nil {
			return result, nil
		}
		work, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		select {
		case s.gate <- struct{}{}:
			defer func() { <-s.gate }()
		case <-work.Done():
			return nil, work.Err()
		}
		result, err := s.resolve(work, req, base)
		if err != nil {
			ttl := 2 * time.Minute
			if errors.Is(err, ErrNotFound) {
				ttl = 24 * time.Hour
			}
			s.mu.Lock()
			s.failures[base] = failure{err, time.Now().Add(ttl)}
			s.mu.Unlock()
		}
		return result, err
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case v := <-ch:
		if v.Err != nil {
			return nil, v.Err
		}
		result := *v.Val.(*Result)
		return &result, nil
	}
}

func (s *Service) resolve(ctx context.Context, req Request, base string) (*Result, error) {
	type option struct {
		provider Provider
		track    Track
	}
	var options []option
	var errs []error
	for _, p := range s.providers {
		candidates, err := p.Search(ctx, req.Query)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s search: %w", p.ID(), err))
			continue
		}
		checked := 0
		for _, c := range candidates {
			if NormalizeCode(c.Code) != NormalizeCode(req.Code) || IsSplit(c.Code, c.Title) {
				continue
			}
			if checked >= 4 {
				break
			}
			checked++
			tracks, err := p.Tracks(ctx, c)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			for _, track := range tracks {
				if track.Format != "srt" {
					continue
				}
				if strings.EqualFold(req.Languages[0], track.Language) {
					track.Language = req.Languages[0]
					options = append(options, option{p, track})
				}
			}
		}
	}
	for _, o := range options {
		data, err := o.provider.Download(ctx, o.track)
		if err == nil {
			data, err = NormalizeSRT(data)
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		result := &Result{Path: base + ".srt", Source: SourceDownload, Language: o.track.Language, Downloaded: true}
		if err := WriteAtomic(result.Path, data); err != nil {
			return nil, err
		}
		return result, nil
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return nil, ErrNotFound
}

// The filename already carries the code and language. The SRT is the entire
// disk cache; no metadata sidecar is needed to reuse it after a restart.
func cached(base string) *Result {
	path := base + ".srt"
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > MaxBytes {
		return nil
	}
	return &Result{Path: path, Source: SourceDownload, Language: strings.TrimPrefix(filepath.Ext(base), ".")}
}

func WriteAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".subtitle-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
