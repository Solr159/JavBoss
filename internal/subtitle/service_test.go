package subtitle

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

const sampleSRT = "1\n00:00:01,000 --> 00:00:02,000\nHello\n\n"

type fakeProvider struct {
	id        string
	language  string
	searches  atomic.Int32
	downloads atomic.Int32
	err       error
	data      []byte
	started   chan struct{}
	release   chan struct{}
}

func (p *fakeProvider) ID() string { return p.id }
func (p *fakeProvider) Search(ctx context.Context, q Query) ([]Candidate, error) {
	p.searches.Add(1)
	if p.started != nil {
		close(p.started)
		select {
		case <-p.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if p.err != nil {
		return nil, p.err
	}
	return []Candidate{{ID: "detail", Code: q.Code, Title: q.Code}}, nil
}
func (p *fakeProvider) Tracks(context.Context, Candidate) ([]Track, error) {
	return []Track{{ID: "download", Language: p.language, Format: "srt"}}, nil
}
func (p *fakeProvider) Download(context.Context, Track) ([]byte, error) {
	p.downloads.Add(1)
	if p.data != nil {
		return p.data, nil
	}
	return []byte(sampleSRT), nil
}

func request(t *testing.T) Request {
	return Request{Query: Query{Code: "ABC-123"}, CacheDir: t.TempDir(), Languages: []string{"zh-CN", "zh-TW"}}
}

func TestResolveLanguagePreferenceFallbackAndDiskCache(t *testing.T) {
	a := &fakeProvider{id: "first", language: "zh-TW"}
	b := &fakeProvider{id: "second", language: "zh-CN"}
	r := request(t)
	s := NewService(a, b)
	result, err := s.Resolve(context.Background(), r)
	if err != nil || result.Source != SourceDownload || !result.Downloaded || b.downloads.Load() != 1 || a.downloads.Load() != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if result.Path != filepath.Join(r.CacheDir, "ABC-123.zh-CN.srt") {
		t.Fatalf("unexpected filename: %s", result.Path)
	}
	if _, err := os.Stat(filepath.Join(r.CacheDir, "ABC-123.zh-CN.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("unexpected JSON sidecar", err)
	}
	data, err := os.ReadFile(result.Path)
	if err != nil || string(data) != sampleSRT {
		t.Fatalf("saved subtitle=%q err=%v", data, err)
	}
	// A fresh service reuses the SRT alone and restores its language from the filename.
	if cached, err := NewService(a, b).Resolve(context.Background(), r); err != nil || cached.Downloaded || cached.Language != "zh-CN" || cached.Source != SourceDownload {
		t.Fatalf("cache marked as fresh download: %+v %v", cached, err)
	}
	if a.searches.Load() != 1 || b.downloads.Load() != 1 {
		t.Fatal("disk cache missed")
	}
	r.Code = "abc-123"
	other, err := s.Resolve(context.Background(), r)
	if err != nil || other.Path != result.Path || b.downloads.Load() != 1 {
		t.Fatalf("same code did not share cache: %v", err)
	}
	r.Languages = []string{"zh-TW", "zh-CN"}
	other, err = s.Resolve(context.Background(), r)
	if err != nil || other.Path != filepath.Join(r.CacheDir, "ABC-123.zh-TW.srt") || other.Source != SourceDownload || a.downloads.Load() != 1 {
		t.Fatalf("language preference ignored: %+v %v", other, err)
	}
}

func TestResolveSkipsBrokenSourcesAndInvalidDownloads(t *testing.T) {
	a := &fakeProvider{id: "offline", err: errors.New("offline")}
	b := &fakeProvider{id: "html", language: "zh-CN", data: []byte("<html>challenge</html>")}
	c := &fakeProvider{id: "good", language: "zh-CN"}
	result, err := NewService(a, b, c).Resolve(context.Background(), request(t))
	if err != nil || result.Source != SourceDownload || c.downloads.Load() != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestResolveCancellationDoesNotCancelSharedDownload(t *testing.T) {
	p := &fakeProvider{id: "test", language: "zh-CN", started: make(chan struct{}), release: make(chan struct{})}
	s := NewService(p)
	r := request(t)
	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan error, 1)
	go func() { _, err := s.Resolve(ctx, r); first <- err }()
	<-p.started
	cancel()
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	close(p.release)
	result, err := s.Resolve(context.Background(), r)
	if err != nil || result == nil || p.searches.Load() != 1 {
		t.Fatalf("shared result=%v err=%v requests=%d", result, err, p.searches.Load())
	}
}

func TestResolveNegativeCacheSeparatesMissingFromFailures(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		ttl  time.Duration
	}{
		{"missing", nil, 24 * time.Hour}, {"network", errors.New("HTTP 403"), 2 * time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &fakeProvider{id: "test", language: "ja", err: tc.err}
			s := NewService(p)
			r := request(t)
			r.Languages = []string{"zh-CN"}
			for i := 0; i < 2; i++ {
				if _, err := s.Resolve(context.Background(), r); err == nil {
					t.Fatal("expected failure")
				}
			}
			if p.searches.Load() != 1 {
				t.Fatal("negative cache missed")
			}
			for _, f := range s.failures {
				remaining := time.Until(f.until)
				if remaining > tc.ttl || remaining < tc.ttl-time.Minute {
					t.Fatalf("TTL=%v", remaining)
				}
			}
		})
	}
}

func TestCachedLookupNeverContactsProviders(t *testing.T) {
	p := &fakeProvider{id: "test", language: "zh-TW"}
	s := NewService(p)
	r := request(t)
	if _, err := s.Cached(context.Background(), r); !errors.Is(err, ErrNotFound) {
		t.Fatalf("empty cache: %v", err)
	}
	if p.searches.Load() != 0 || len(s.failures) != 0 {
		t.Fatal("local lookup contacted providers or cached a failure")
	}
	// Populate only the fallback language, then verify offline preference/fallback.
	onlyTW := r
	onlyTW.Languages = []string{"zh-TW"}
	cachedTW, err := s.Resolve(context.Background(), onlyTW)
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.Cached(context.Background(), r)
	if err != nil || result.Path != cachedTW.Path || p.searches.Load() != 1 {
		t.Fatalf("cached fallback not reused: %+v %v", result, err)
	}
	p.language = "zh-CN"
	if _, err := s.Resolve(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	result, err = s.Cached(context.Background(), r)
	if err != nil || result.Language != "zh-CN" || p.searches.Load() != 2 {
		t.Fatalf("cached preference not honored: %+v %v", result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Cached(ctx, r); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestStandaloneSRTCacheIgnoresMetadata(t *testing.T) {
	for _, metadata := range []string{"", "invalid JSON"} {
		t.Run(metadata, func(t *testing.T) {
			r := request(t)
			path := filepath.Join(r.CacheDir, "ABC-123.zh-TW.srt")
			if err := os.WriteFile(path, []byte(sampleSRT), 0600); err != nil {
				t.Fatal(err)
			}
			if metadata != "" {
				if err := os.WriteFile(filepath.Join(r.CacheDir, "ABC-123.zh-TW.json"), []byte(metadata), 0600); err != nil {
					t.Fatal(err)
				}
			}
			p := &fakeProvider{id: "offline", err: errors.New("provider must not be contacted")}
			s := NewService(p)
			cached, err := s.Cached(context.Background(), r)
			if err != nil || cached.Path != path || cached.Language != "zh-TW" || cached.Source != SourceDownload || cached.Downloaded {
				t.Fatalf("standalone cache: %+v %v", cached, err)
			}
			r.Languages = []string{"zh-TW"}
			if _, err := s.Resolve(context.Background(), r); err != nil {
				t.Fatal(err)
			}
			if p.searches.Load() != 0 {
				t.Fatal("cache lookup contacted provider")
			}
		})
	}
}
