package preview

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestSecond(t *testing.T) {
	for _, tc := range []struct {
		second, duration float64
		want             int64
		invalid          bool
	}{
		{0, 60, 0, false}, {2.99, 60, 0, false}, {3, 60, 3, false},
		{5.99, 60, 3, false}, {6, 60, 6, false}, {19.9, 60, 18, false}, {60, 60, 57, false},
		{0.5, 0.5, 0, false}, {-1, 60, 0, true}, {61, 60, 0, true},
		{0, 0, 0, true}, {math.NaN(), 60, 0, true}, {math.Inf(1), 60, 0, true},
	} {
		got, err := Second(tc.second, tc.duration)
		if got != tc.want || (err != nil) != tc.invalid {
			t.Fatalf("Second(%v,%v) = %d,%v", tc.second, tc.duration, got, err)
		}
	}
}

func sourceFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "video.mp4")
	if err := os.WriteFile(path, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCacheIdentityPersistenceWithoutEviction(t *testing.T) {
	source := sourceFile(t)
	dir := t.TempDir()
	// A sparse file exceeds the former disk limit without allocating image data.
	largePath := filepath.Join(dir, "999_0.jpg")
	if err := os.WriteFile(largePath, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(largePath, (256<<20)+1); err != nil {
		t.Fatal(err)
	}
	var calls int
	capture := func(_ context.Context, _ string, _ float64, output string) error {
		calls++
		return os.WriteFile(output, []byte("image"), 0600)
	}
	c := newCache(t.Context(), dir, capture)
	get := func(videoID, second int64) {
		t.Helper()
		data, err := c.get(t.Context(), videoID, source, second)
		if err != nil || string(data) != "image" {
			t.Fatalf("get = %q, %v", data, err)
		}
	}
	get(1, 0)
	get(1, 0)
	if _, err := os.Stat(filepath.Join(dir, "1_0.jpg")); err != nil {
		t.Fatalf("expected videoID_second.jpg: %v", err)
	}
	if calls != 1 {
		t.Fatalf("cache miss: %d", calls)
	}
	c = newCache(t.Context(), dir, capture)
	get(1, 0)
	if calls != 1 {
		t.Fatal("cache did not survive restart")
	}
	// A different path and modification time must not duplicate the same video.
	copyPath := sourceFile(t)
	stamp := time.Now().Add(time.Hour)
	if err := os.Chtimes(copyPath, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if data, err := c.get(t.Context(), 1, copyPath, 0); err != nil || string(data) != "image" {
		t.Fatalf("copy preview = %q, %v", data, err)
	}
	if calls != 1 {
		t.Fatal("same video at another location was regenerated")
	}
	if err := os.WriteFile(source, []byte("replacement source"), 0600); err != nil {
		t.Fatal(err)
	}
	// A replacement assigned a different video ID by the scanner gets its own image.
	get(2, 0)
	if calls != 2 {
		t.Fatal("different video ID reused cache")
	}
	get(1, 10)
	get(1, 20)
	for _, name := range []string{"999_0.jpg", "1_0.jpg", "2_0.jpg", "1_10.jpg", "1_20.jpg"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("cached preview %s was removed: %v", name, err)
		}
	}
}

func await(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestSharedJobCancellationAndConcurrencyLimit(t *testing.T) {
	source := sourceFile(t)
	var calls atomic.Int32
	started := make(chan struct{}, 4)
	release := make(chan struct{})
	c := newCache(t.Context(), t.TempDir(), func(ctx context.Context, _ string, _ float64, output string) error {
		calls.Add(1)
		started <- struct{}{}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-release:
			return os.WriteFile(output, []byte("image"), 0600)
		}
	})
	ctx, cancel := context.WithCancel(t.Context())
	first := make(chan error, 1)
	go func() { _, err := c.get(ctx, 1, source, 0); first <- err }()
	<-started
	second := make(chan error, 1)
	go func() { _, err := c.get(t.Context(), 1, source, 0); second <- err }()
	await(t, func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		for _, j := range c.jobs {
			if j.waiters == 2 {
				return true
			}
		}
		return false
	})
	cancel()
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	thirdCtx, cancelThird := context.WithCancel(t.Context())
	third := make(chan error, 1)
	go func() { _, err := c.get(thirdCtx, 1, source, 10); third <- err }()
	<-started
	if _, err := c.get(t.Context(), 1, source, 20); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("third worker admitted: %v", err)
	}
	cancelThird()
	if err := <-third; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	await(t, func() bool { c.mu.Lock(); defer c.mu.Unlock(); return c.workers == 1 })
	close(release)
	if err := <-second; err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("duplicate captures: %d", calls.Load())
	}
	if _, err := c.get(t.Context(), 1, source, 10); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 {
		t.Fatal("cancelled work was cached")
	}
}

func TestCaptureFailureAndChangingSourceAreNotCached(t *testing.T) {
	for _, mode := range []string{"failure", "empty", "changed"} {
		t.Run(mode, func(t *testing.T) {
			source := sourceFile(t)
			c := newCache(t.Context(), t.TempDir(), func(_ context.Context, _ string, _ float64, output string) error {
				if mode == "failure" {
					return errors.New("capture failed")
				}
				if mode == "changed" {
					if err := os.WriteFile(source, []byte("changed source"), 0600); err != nil {
						return err
					}
					return os.WriteFile(output, []byte("image"), 0600)
				}
				return os.WriteFile(output, nil, 0600)
			})
			if _, err := c.get(t.Context(), 1, source, 0); err == nil {
				t.Fatal("expected failure")
			}
			if _, err := os.Stat(filepath.Join(c.dir, "1_0.jpg")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("failed capture was cached: %v", err)
			}
		})
	}
}
