package manager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"pornboss/internal/common/logging"
	"pornboss/internal/jav"
	"pornboss/internal/util"
)

// CoverManager coordinates background cover downloads.
type CoverManager struct {
	tasks     chan string
	coverDir  string
	workers   int
	providers []jav.Provider
	mu        sync.Mutex
	scheduled map[string]struct{}
}

const minValidCoverSizeBytes int64 = 30 * 1024

var errInvalidCover = errors.New("invalid cover")

var lookupCoverURLByCode = jav.LookupCoverURLByCode

// NewCoverManager creates a manager when coverDir and providers are provided.
func NewCoverManager(coverDir string, providers []jav.Provider) *CoverManager {
	coverDir = strings.TrimSpace(coverDir)
	providers = compactCoverProviders(providers)
	if coverDir == "" || len(providers) == 0 {
		return nil
	}
	return &CoverManager{
		tasks:     make(chan string, 5000), // larger buffer to reduce producer blocking
		coverDir:  coverDir,
		workers:   8,
		providers: providers,
		scheduled: make(map[string]struct{}),
	}
}

// Start launches the worker; safe to call with nil manager.
func (m *CoverManager) Start(ctx context.Context) {
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
func (m *CoverManager) Enqueue(code string) {
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

// Exists reports whether a cover file already exists for the code (any known extension).
func (m *CoverManager) Exists(code string) bool {
	if m == nil {
		return false
	}
	path, ok := FindCoverPath(m.coverDir, code)
	if !ok {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Size() >= minValidCoverSizeBytes
}

func (m *CoverManager) worker(ctx context.Context) {
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
				if err := m.handleTask(ctx, code); err != nil {
					logging.Error("jav cover: code=%s err=%v", code, err)
				}
			}()
		}
	}
}

func (m *CoverManager) clearScheduled(code string) {
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

func (m *CoverManager) handleTask(parent context.Context, code string) error {
	code = normalizeCode(code)
	if code == "" {
		return errors.New("empty code")
	}
	if m.Exists(code) {
		return nil
	}

	ctx, cancel := context.WithTimeout(parent, 45*time.Second)
	defer cancel()

	if err := m.downloadCoverFromProviders(ctx, code); err != nil {
		if errors.Is(err, util.ErrCachedNotFound) {
			return nil
		}
		return err
	}
	return nil
}

func (m *CoverManager) downloadCoverFromProviders(ctx context.Context, code string) error {
	if m == nil {
		return errors.New("cover manager not configured")
	}
	var lastErr error
	for _, provider := range m.providers {
		coverURL, err := lookupCoverURLByCode(code, provider)
		if err != nil {
			if errors.Is(err, jav.ResourceNotFonud) {
				continue
			}
			lastErr = err
			logging.Error("fetch cover url failed: provider=%s code=%s err=%v", provider.String(), code, err)
			continue
		}

		coverURL = strings.TrimSpace(coverURL)
		if coverURL == "" {
			continue
		}
		if err := m.downloadCover(ctx, code, coverURL); err != nil {
			if errors.Is(err, util.ErrCachedNotFound) || errors.Is(err, errInvalidCover) {
				lastErr = err
				continue
			}
			lastErr = err
			logging.Error("download cover failed: provider=%s code=%s err=%v", provider.String(), code, err)
			continue
		}
		return nil
	}
	if lastErr != nil {
		return fmt.Errorf("download cover from providers: %w", lastErr)
	}
	return util.ErrCachedNotFound
}

func (m *CoverManager) downloadCover(ctx context.Context, code, coverURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, coverURL, nil)
	if err != nil {
		return fmt.Errorf("build cover request: %w", err)
	}
	setCoverDownloadHeaders(req)
	resp, err := util.DoRequest(req)
	if err != nil {
		if errors.Is(err, util.ErrCachedNotFound) {
			return err
		}
		return fmt.Errorf("download cover: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return util.ErrCachedNotFound
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

	target := filepath.Join(m.coverDir, code+ext)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("ensure cover dir: %w", err)
	}
	tmp := target + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	written, err := io.Copy(out, resp.Body)
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
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("finalize cover: %w", err)
	}
	return nil
}

func setCoverDownloadHeaders(req *http.Request) {
	if req == nil || req.URL == nil {
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; JavCoverBot/1.0)")
	host := strings.ToLower(req.URL.Hostname())
	if host == "javbus.com" || strings.HasSuffix(host, ".javbus.com") {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
		req.Header.Set("Referer", "https://www.javbus.com/")
		req.Header.Set("Cookie", "age=verified; existmag=mag")
	}
}

var knownExts = []string{".jpg", ".jpeg", ".png", ".webp"}

func normalizeCode(code string) string {
	return strings.ToLower(strings.TrimSpace(code))
}

// FindCoverPath returns the existing cover file path for the given code within dir.
func FindCoverPath(dir, code string) (string, bool) {
	code = normalizeCode(code)
	if code == "" {
		return "", false
	}
	for _, ext := range knownExts {
		p := filepath.Join(dir, code+ext)
		info, err := os.Stat(p)
		if err == nil && info.Size() >= minValidCoverSizeBytes {
			return p, true
		}
	}
	return "", false
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

func compactCoverProviders(providers []jav.Provider) []jav.Provider {
	if len(providers) == 0 {
		return nil
	}
	compact := make([]jav.Provider, 0, len(providers))
	for _, provider := range providers {
		provider = jav.ParseProvider(int(provider))
		if provider != jav.ProviderUnknown && provider != jav.ProviderUser {
			compact = append(compact, provider)
		}
	}
	return compact
}
