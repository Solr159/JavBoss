package cover

import (
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/jav/scrape"
	"javboss/internal/jav/scrape/javdb"
	"javboss/internal/util"
)

// queue coordinates background cover downloads.
type queue struct {
	tasks     chan string
	coverDir  string
	workers   int
	providers []scrape.Provider
	mu        sync.Mutex
	scheduled map[string]struct{}
}

const (
	minValidCoverSizeBytes int64 = 5 * 1024
	coverProviderTimeout         = 8 * time.Second
)

var errInvalidCover = errors.New("invalid cover")
var errCoverNotFound = errors.New("cover not found")

var lookupJavByCode = scrape.LookupJavByCode

// newQueue creates a queue with the built-in cover providers.
func newQueue(coverDir string) *queue {
	coverDir = strings.TrimSpace(coverDir)
	if coverDir == "" {
		return nil
	}
	return &queue{
		tasks:    make(chan string, 5000), // larger buffer to reduce producer blocking
		coverDir: coverDir,
		workers:  8,
		providers: []scrape.Provider{
			scrape.ProviderJavBus,
			scrape.ProviderJavDatabase,
			scrape.ProviderThePornDB,
			scrape.ProviderJavDBAPI,
			scrape.ProviderAvsox,
		},
		scheduled: make(map[string]struct{}),
	}
}

// Start launches the worker; safe to call with nil queue.
func (m *queue) Start(ctx context.Context) {
	if m == nil {
		return
	}
	if m.workers <= 0 {
		m.workers = 1
	}
	for i := 0; i < m.workers; i++ {
		go m.worker(ctx)
	}
}

// Enqueue schedules a cover download; blocks when queue is full.
func (m *queue) Enqueue(code string) {
	if m == nil {
		return
	}
	code = normalizeCode(code)
	if code == "" {
		return
	}
	if m.tasks == nil {
		return
	}

	m.mu.Lock()
	if m.scheduled == nil {
		m.scheduled = make(map[string]struct{})
	}
	if _, ok := m.scheduled[code]; ok {
		m.mu.Unlock()
		return
	}
	m.scheduled[code] = struct{}{}
	m.mu.Unlock()

	m.tasks <- code
}

func (m *queue) worker(ctx context.Context) {
	if m == nil {
		return
	}
	_ = os.MkdirAll(m.coverDir, 0o755)
	for {
		select {
		case <-ctx.Done():
			return
		case code := <-m.tasks:
			func() {
				defer m.clearScheduled(code)
				if err := handleTask(ctx, m.coverDir, m.providers, code); err != nil {
					logging.Error("jav cover: code=%s err=%v", code, err)
				}
			}()
		}
	}
}

func (m *queue) clearScheduled(code string) {
	if m == nil {
		return
	}
	code = normalizeCode(code)
	if code == "" {
		return
	}
	m.mu.Lock()
	delete(m.scheduled, code)
	m.mu.Unlock()
}

func handleTask(ctx context.Context, coverDir string, providers []scrape.Provider, code string) error {
	code = normalizeCode(code)
	if code == "" {
		return errors.New("empty code")
	}
	if exists(coverDir, code) {
		return nil
	}

	if err := downloadCoverFromProviders(ctx, coverDir, providers, code); err != nil {
		if errors.Is(err, errCoverNotFound) {
			return nil
		}
		return err
	}
	return nil
}

func downloadCoverFromProviders(ctx context.Context, coverDir string, providers []scrape.Provider, code string) error {
	var lastErr error
	for _, provider := range providers {
		if err := ctx.Err(); err != nil {
			return err
		}
		// Each provider gets its own budget for both metadata and image download.
		providerCtx, cancel := context.WithTimeout(ctx, coverProviderTimeout)
		info, err := lookupJavByCode(providerCtx, code, provider)
		if err != nil {
			cancel()
			if errors.Is(err, scrape.ErrNotFound) {
				continue
			}
			lastErr = err
			logging.Error("fetch cover metadata failed: provider=%s code=%s err=%v", provider.String(), code, err)
			continue
		}

		coverURL := ""
		if info != nil {
			coverURL = strings.TrimSpace(info.CoverURL)
		}
		if coverURL == "" {
			cancel()
			continue
		}
		err = downloadCover(providerCtx, coverDir, code, coverURL)
		cancel()
		if err != nil {
			if errors.Is(err, errCoverNotFound) || errors.Is(err, errInvalidCover) {
				lastErr = err
				continue
			}
			lastErr = err
			logging.Error("download cover failed: provider=%s code=%s err=%v", provider.String(), code, err)
			continue
		}
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if lastErr != nil {
		return fmt.Errorf("download cover from providers: %w", lastErr)
	}
	return errCoverNotFound
}

func downloadCover(ctx context.Context, coverDir, code, coverURL string) error {
	code = normalizeCode(code)
	if code == "" {
		return errors.New("empty code")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, coverURL, nil)
	if err != nil {
		return fmt.Errorf("build cover request: %w", err)
	}
	setCoverDownloadHeaders(req)
	resp, err := util.DefaultCachedHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("download cover: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return errCoverNotFound
		}
		return fmt.Errorf("download cover: status %s", resp.Status)
	}

	ext := strings.ToLower(path.Ext(resp.Request.URL.Path))
	if ext == "" || len(ext) > 5 {
		ext = guessExt(resp.Header.Get("Content-Type"))
	}
	if ext == "" {
		ext = ".jpg"
	}

	target := filepath.Join(coverDir, code+ext)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("ensure cover dir: %w", err)
	}
	tmp := target + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	body, encoded := javdb.DecodeImageBody(resp.Body)
	written, err := io.Copy(out, body)
	if err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("write cover: %w", err)
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close cover: %w", err)
	}
	if written < minValidCoverSizeBytes {
		_ = os.Remove(tmp)
		return fmt.Errorf("%w: size %d below minimum %d", errInvalidCover, written, minValidCoverSizeBytes)
	}
	blacklisted, err := isBlacklistedCoverFile(tmp, written)
	if err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("check cover blacklist: %w", err)
	}
	if blacklisted {
		_ = os.Remove(tmp)
		return fmt.Errorf("%w: known placeholder image", errInvalidCover)
	}
	if encoded && !isDecodableCoverFile(tmp) {
		_ = os.Remove(tmp)
		return fmt.Errorf("%w: file (%d bytes) is not a decodable image", errInvalidCover, written)
	}
	removeCoverFiles(coverDir, code)
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("finalize cover: %w", err)
	}
	return nil
}

func removeCoverFiles(coverDir, code string) {
	code = normalizeCode(code)
	if coverDir == "" || code == "" {
		return
	}
	for _, ext := range knownExts {
		p := filepath.Join(coverDir, code+ext)
		_ = os.Remove(p)
	}
}

func setCoverDownloadHeaders(req *http.Request) {
	util.SetJavImageRequestHeaders(req)
}

var knownExts = []string{".jpg", ".jpeg", ".png", ".webp"}

// DownloadFromURL downloads a user-provided cover URL and replaces any existing cover for code.
func DownloadFromURL(ctx context.Context, coverDir, code, coverURL string) error {
	coverDir = strings.TrimSpace(coverDir)
	code = normalizeCode(code)
	coverURL = strings.TrimSpace(coverURL)
	if coverDir == "" {
		return errors.New("cover dir is not configured")
	}
	if code == "" {
		return errors.New("empty code")
	}
	if coverURL == "" {
		return errors.New("cover url is required")
	}
	u, err := url.Parse(coverURL)
	if err != nil || u == nil || u.Hostname() == "" {
		return errors.New("invalid cover url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("cover url must be http or https")
	}

	return downloadCover(ctx, coverDir, code, coverURL)
}

func normalizeCode(code string) string {
	return strings.ToLower(strings.TrimSpace(code))
}

// FindPath returns the existing cover file path for the given code within dir.
func FindPath(dir, code string) (string, bool) {
	code = normalizeCode(code)
	if code == "" {
		return "", false
	}
	for _, ext := range knownExts {
		p := filepath.Join(dir, code+ext)
		if isValidCoverFile(p) {
			return p, true
		}
	}
	return "", false
}

func isValidCoverFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	if info.Size() < minValidCoverSizeBytes {
		return false
	}
	blacklisted, err := isBlacklistedCoverFile(path, info.Size())
	return err == nil && !blacklisted
}

func isDecodableCoverFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return err == nil && !img.Bounds().Empty()
}

func guessExt(ct string) string {
	ct = strings.ToLower(strings.TrimSpace(ct))
	switch {
	case strings.Contains(ct, "webp"):
		return ".webp"
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "jpeg"), strings.Contains(ct, "jpg"):
		return ".jpg"
	default:
		return ""
	}
}

func exists(coverDir, code string) bool {
	_, ok := FindPath(coverDir, code)
	return ok
}
