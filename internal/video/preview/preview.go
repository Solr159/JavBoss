// Package preview generates on-demand timeline thumbnails separately
// from user screenshots. Uncached work is never queued behind other videos.
package preview

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"javboss/internal/video/screenshot"
)

const Interval = 3

var ErrUnavailable = errors.New("preview generation is unavailable or busy")
var ErrInvalidTime = errors.New("invalid preview timestamp")
var active atomic.Pointer[cache]

type captureFunc func(context.Context, string, float64, string) error
type job struct {
	done    chan struct{}
	cancel  context.CancelFunc
	waiters int
	data    []byte
	err     error
}
type cache struct {
	ctx     context.Context
	dir     string
	capture captureFunc
	mu      sync.Mutex
	jobs    map[string]*job
	workers int
	ready   bool
}

func Start(ctx context.Context, dir string) {
	active.Store(newCache(ctx, dir, screenshot.CapturePreviewFile))
}

func newCache(ctx context.Context, dir string, capture captureFunc) *cache {
	return &cache{ctx: ctx, dir: dir, capture: capture, jobs: make(map[string]*job)}
}

// Second maps a hover position to a three-second bucket, including the end edge.
func Second(second, duration float64) (int64, error) {
	if math.IsNaN(second) || math.IsInf(second, 0) || second < 0 ||
		math.IsNaN(duration) || math.IsInf(duration, 0) || duration <= 0 || second > duration || duration >= math.MaxInt64 {
		return 0, ErrInvalidTime
	}
	second = math.Min(second, math.Max(0, duration-0.001))
	return int64(second/Interval) * Interval, nil
}

// Get returns a cached image or generates it on demand.
func Get(ctx context.Context, videoID int64, source string, second int64) ([]byte, error) {
	return active.Load().get(ctx, videoID, source, second)
}

func (c *cache) get(ctx context.Context, videoID int64, source string, second int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c == nil || c.dir == "" || c.ctx.Err() != nil {
		return nil, ErrUnavailable
	}
	info, err := os.Stat(source)
	if err != nil {
		return nil, fmt.Errorf("stat preview source: %w", err)
	}
	// The scanner owns video identity; copies at different locations share previews.
	key := fmt.Sprintf("%d_%d.jpg", videoID, second)
	c.mu.Lock()
	if err := c.initLocked(); err != nil {
		c.mu.Unlock()
		return nil, err
	}
	if data, err := os.ReadFile(filepath.Join(c.dir, key)); err == nil {
		c.mu.Unlock()
		return data, nil
	}
	j := c.jobs[key]
	if j == nil {
		if c.workers >= 2 {
			c.mu.Unlock()
			return nil, ErrUnavailable
		}
		workCtx, cancel := context.WithTimeout(c.ctx, 20*time.Second)
		j = &job{done: make(chan struct{}), cancel: cancel}
		c.jobs[key] = j
		c.workers++
		go c.generate(workCtx, key, source, second, info, j)
	}
	j.waiters++
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		j.waiters--
		if j.waiters == 0 && c.jobs[key] == j {
			delete(c.jobs, key)
			j.cancel()
		}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-j.done:
		return j.data, j.err
	}
}

func (c *cache) generate(ctx context.Context, key, source string, second int64, original os.FileInfo, j *job) {
	defer j.cancel()
	tempDir, err := os.MkdirTemp(c.dir, ".preview-")
	var data []byte
	path := filepath.Join(tempDir, "frame.jpg")
	if err == nil {
		defer os.RemoveAll(tempDir)
		err = c.capture(ctx, source, float64(second), path)
		if err == nil {
			data, err = os.ReadFile(path)
		}
		if err == nil && len(data) == 0 {
			err = errors.New("empty preview image")
		}
		if err == nil {
			err = ctx.Err()
		}
		if err == nil {
			info, statErr := os.Stat(source)
			if statErr != nil {
				err = statErr
			} else if info.Size() != original.Size() || !info.ModTime().Equal(original.ModTime()) {
				err = errors.New("preview source changed during capture")
			}
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err == nil {
		err = ctx.Err()
	}
	if err == nil {
		err = os.Rename(path, filepath.Join(c.dir, key))
	}
	j.data, j.err = data, err
	if c.jobs[key] == j {
		delete(c.jobs, key)
	}
	c.workers--
	close(j.done)
}

func (c *cache) initLocked() error {
	if c.ready {
		return nil
	}
	if err := os.MkdirAll(c.dir, 0755); err != nil {
		return fmt.Errorf("create preview cache: %w", err)
	}
	files, err := os.ReadDir(c.dir)
	if err != nil {
		return fmt.Errorf("read preview cache: %w", err)
	}
	for _, file := range files {
		if strings.HasPrefix(file.Name(), ".preview-") {
			_ = os.RemoveAll(filepath.Join(c.dir, file.Name()))
		}
	}
	c.ready = true
	return nil
}
