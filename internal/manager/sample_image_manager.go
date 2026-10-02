package manager

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"javboss/internal/common/logging"
	"javboss/internal/jav/javdb"
	"javboss/internal/util"
)

// SampleImageManager caches JAV sample images on disk. Remote image hosts are
// slow and flaky, so each decoded image is written once and served locally on
// subsequent requests. Warming fills the cache in the background.
type SampleImageManager struct {
	dir     string
	tasks   chan string
	workers int
	group   singleflight.Group

	mu        sync.Mutex
	scheduled map[string]struct{}
}

const (
	maxSampleImageBytes = 16 << 20
	sampleImageWorkers  = 4
	sampleImageQueue    = 4096
	sampleImageTimeout  = 30 * time.Second
)

var errInvalidSampleImage = errors.New("invalid sample image")

var sampleImageExts = []string{".jpg", ".jpeg", ".png", ".webp", ".gif"}

// NewSampleImageManager creates a manager when dir is provided.
func NewSampleImageManager(dir string) *SampleImageManager {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil
	}
	return &SampleImageManager{
		dir:       dir,
		tasks:     make(chan string, sampleImageQueue),
		workers:   sampleImageWorkers,
		scheduled: make(map[string]struct{}),
	}
}

// Start launches the background warm workers; safe to call with a nil manager.
func (m *SampleImageManager) Start(ctx context.Context) {
	if m == nil {
		return
	}
	_ = os.MkdirAll(m.dir, 0o755)
	workers := m.workers
	if workers <= 0 {
		workers = 1
	}
	for i := 0; i < workers; i++ {
		go m.worker(ctx)
	}
}

// Get returns the local path for sourceURL, downloading and caching it on miss.
// Concurrent requests for the same URL share a single download.
func (m *SampleImageManager) Get(ctx context.Context, sourceURL string) (string, error) {
	if m == nil {
		return "", errors.New("sample image manager not configured")
	}
	sourceURL = strings.TrimSpace(sourceURL)
	if sourceURL == "" {
		return "", errInvalidSampleImage
	}
	if path, ok := m.cached(sourceURL); ok {
		return path, nil
	}

	key := sampleImageCacheKey(sourceURL)
	value, err, _ := m.group.Do(key, func() (any, error) {
		if path, ok := m.cached(sourceURL); ok {
			return path, nil
		}
		// Detach from the caller so a client disconnect does not abort a
		// download other requests are waiting on.
		downloadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sampleImageTimeout)
		defer cancel()
		return m.download(downloadCtx, sourceURL)
	})
	if err != nil {
		return "", err
	}
	return value.(string), nil
}

// Warm queues a background download; safe to call with a nil manager.
func (m *SampleImageManager) Warm(sourceURL string) {
	if m == nil || m.tasks == nil {
		return
	}
	sourceURL = strings.TrimSpace(sourceURL)
	if sourceURL == "" {
		return
	}
	if _, ok := m.cached(sourceURL); ok {
		return
	}

	m.mu.Lock()
	if _, ok := m.scheduled[sourceURL]; ok {
		m.mu.Unlock()
		return
	}
	m.scheduled[sourceURL] = struct{}{}
	m.mu.Unlock()

	select {
	case m.tasks <- sourceURL:
	default:
		// Queue is full: drop the warm request rather than block the caller.
		m.clearScheduled(sourceURL)
	}
}

func (m *SampleImageManager) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case sourceURL := <-m.tasks:
			func() {
				defer m.clearScheduled(sourceURL)
				if _, err := m.Get(ctx, sourceURL); err != nil {
					logging.Info("jav sample image cache miss skipped: url=%s err=%v", sourceURL, err)
				}
			}()
		}
	}
}

func (m *SampleImageManager) clearScheduled(sourceURL string) {
	m.mu.Lock()
	delete(m.scheduled, sourceURL)
	m.mu.Unlock()
}

func (m *SampleImageManager) cached(sourceURL string) (string, bool) {
	key := sampleImageCacheKey(sourceURL)
	for _, ext := range sampleImageExts {
		path := filepath.Join(m.dir, key[:2], key+ext)
		info, err := os.Stat(path)
		if err == nil && info.Mode().IsRegular() && info.Size() > 0 {
			return path, true
		}
	}
	return "", false
}

func (m *SampleImageManager) download(ctx context.Context, sourceURL string) (string, error) {
	parsed, err := url.Parse(sourceURL)
	if err != nil || parsed == nil || parsed.Hostname() == "" ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errInvalidSampleImage
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return "", errInvalidSampleImage
	}
	util.SetJavImageRequestHeaders(req)

	resp, err := util.DefaultCachedHTTPClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("sample image returned %s", resp.Status)
	}

	body, encoded := javdb.DecodeImageBody(resp.Body)
	data, err := io.ReadAll(io.LimitReader(body, maxSampleImageBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) == 0 || len(data) > maxSampleImageBytes {
		return "", errInvalidSampleImage
	}
	contentType := http.DetectContentType(data)
	if !strings.HasPrefix(contentType, "image/") {
		return "", errInvalidSampleImage
	}
	if encoded {
		if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
			return "", errInvalidSampleImage
		}
	}

	key := sampleImageCacheKey(sourceURL)
	target := filepath.Join(m.dir, key[:2], key+sampleImageExtForContentType(contentType))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", fmt.Errorf("ensure sample image dir: %w", err)
	}
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", fmt.Errorf("write sample image: %w", err)
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("finalize sample image: %w", err)
	}
	return target, nil
}

func sampleImageCacheKey(sourceURL string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(sourceURL)))
	return hex.EncodeToString(sum[:])
}

func sampleImageExtForContentType(ct string) string {
	ct = strings.ToLower(ct)
	switch {
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "webp"):
		return ".webp"
	case strings.Contains(ct, "gif"):
		return ".gif"
	default:
		return ".jpg"
	}
}
