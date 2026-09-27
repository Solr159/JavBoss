package manager

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
	"javboss/internal/jav"
	"javboss/internal/jav/javdb"
	"javboss/internal/util"
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

var lookupJavByCode = jav.LookupJavByCode

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
	_, ok := FindCoverPath(m.coverDir, code)
	return ok
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
	providers := m.providers
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(code)), "FC2-PPV-") {
		// FC2 metadata (including the cover URL) comes from JavDB API. The
		// general cover sources do not resolve these numbers.
		providers = []jav.Provider{jav.ProviderJavDBAPI, jav.ProviderAvsox}
	}
	var lastErr error
	for _, provider := range providers {
		info, err := lookupJavByCode(ctx, code, provider)
		if err != nil {
			if errors.Is(err, jav.ErrNotFound) {
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
	code = normalizeCode(code)
	if code == "" {
		return errors.New("empty code")
	}
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
	if (encoded || written < minValidCoverSizeBytes) && !isDecodableCoverFile(tmp) {
		_ = os.Remove(tmp)
		return fmt.Errorf("%w: file (%d bytes) is not a decodable image", errInvalidCover, written)
	}
	removeCoverFiles(m.coverDir, code)
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

// DownloadCoverFromURL downloads a user-provided cover URL and replaces any existing cover for code.
func DownloadCoverFromURL(ctx context.Context, coverDir, code, coverURL string) error {
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

	manager := &CoverManager{coverDir: coverDir}
	return manager.downloadCover(ctx, code, coverURL)
}

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
	// FC2 covers are validated when downloaded. Loading them only checks the
	// filename and file metadata, avoiding another image decode for small files.
	if strings.HasPrefix(strings.ToLower(filepath.Base(path)), "fc2-ppv-") {
		return info.Size() > 0
	}
	return info.Size() >= minValidCoverSizeBytes
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

func compactCoverProviders(providers []jav.Provider) []jav.Provider {
	if len(providers) == 0 {
		return nil
	}
	compact := make([]jav.Provider, 0, len(providers))
	for _, provider := range providers {
		provider = jav.ParseProvider(int(provider))
		if provider != jav.ProviderUnknown && provider != jav.ProviderUser && provider != jav.ProviderManualScrape {
			compact = append(compact, provider)
		}
	}
	return compact
}
