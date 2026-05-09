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
}

const minValidCoverSizeBytes int64 = 30 * 1024

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
		workers:   10,
		providers: providers,
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
	code = strings.TrimSpace(code)
	if code == "" {
		return
	}
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
			if err := m.handleTask(ctx, code); err != nil {
				logging.Error("jav cover: code=%s err=%v", code, err)
			}
		}
	}
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

	coverURL, err := m.fetchCoverURL(code)
	if err != nil {
		if errors.Is(err, util.ErrCachedNotFound) {
			return nil
		}
		return err
	}
	if coverURL == "" {
		return errors.New("cover url not found")
	}

	if err := m.downloadCover(ctx, code, coverURL); err != nil {
		if errors.Is(err, util.ErrCachedNotFound) {
			return nil
		}
		return err
	}
	return nil
}

func (m *CoverManager) fetchCoverURL(code string) (string, error) {
	if m == nil {
		return "", errors.New("cover manager not configured")
	}
	var lastErr error
	for _, provider := range m.providers {
		coverURL, err := jav.LookupCoverURLByCode(code, provider)
		if err == nil {
			coverURL = strings.TrimSpace(coverURL)
			if coverURL != "" {
				return coverURL, nil
			}
			continue
		}
		if errors.Is(err, jav.ResourceNotFonud) {
			continue
		}
		lastErr = err
		logging.Error("fetch cover url failed: provider=%s code=%s err=%v", provider.String(), code, err)
	}
	if lastErr != nil {
		return "", fmt.Errorf("fetch cover url: %w", lastErr)
	}
	return "", util.ErrCachedNotFound
}

func (m *CoverManager) downloadCover(ctx context.Context, code, coverURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, coverURL, nil)
	if err != nil {
		return fmt.Errorf("build cover request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; JavCoverBot/1.0)")
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
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("write cover: %w", err)
	}
	out.Close()

	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("finalize cover: %w", err)
	}
	return nil
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
