package autosub

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/db"
	"javboss/internal/models"
)

const (
	// AutoEnabledKey is the config key that turns the background subtitle sweep on or off.
	AutoEnabledKey = "autosub_auto_enabled"
	// AutoLanguageKey is the config key holding the subtitle language requested from the gateway.
	AutoLanguageKey = "autosub_auto_language"
	// DefaultAutoLanguage mirrors the default used by the manual download endpoint.
	DefaultAutoLanguage = "Chinese Simplified"

	autoSweepInterval    = 10 * time.Minute
	autoSweepBatchSize   = 10
	autoFailureCooldown  = 6 * time.Hour
	autoDownloadTimeout  = 10 * time.Minute
	autoGatewayPingLimit = 5 * time.Second
)

// subtitleExtensions lists sidecar subtitle formats recognized as "already subtitled".
var subtitleExtensions = map[string]struct{}{
	".ass": {},
	".srt": {},
	".ssa": {},
	".sub": {},
	".vtt": {},
}

// Gateway downloads subtitles for one video and reports reachability.
type Gateway interface {
	Download(ctx context.Context, req DownloadRequest) (DownloadResult, error)
	Ping(ctx context.Context) error
}

// LocationSource lists active video locations, both newest-first and in
// ascending ID order from a cursor.
type LocationSource interface {
	ListAutoSubtitleLocations(ctx context.Context, afterID int64, limit int) ([]models.VideoLocation, error)
	ListRecentAutoSubtitleLocations(ctx context.Context, limit int) ([]models.VideoLocation, error)
}

// ConfigSource reads persisted key/value settings.
type ConfigSource interface {
	ListConfig(ctx context.Context) (map[string]string, error)
}

// ErrSweepRunning is returned when a sweep is requested while another pass is
// still in flight.
var ErrSweepRunning = errors.New("auto subtitle sweep already running")

// AutoSweepResult summarizes one pass of the background subtitle sweep.
type AutoSweepResult struct {
	Enabled    bool
	Language   string
	Scanned    int
	Downloaded int
	Skipped    int
	Failed     int
}

// RunSummary describes one finished sweep so the UI can report its outcome.
type RunSummary struct {
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	Enabled    bool      `json:"enabled"`
	Language   string    `json:"language"`
	Scanned    int       `json:"scanned"`
	Downloaded int       `json:"downloaded"`
	Skipped    int       `json:"skipped"`
	Failed     int       `json:"failed"`
	Error      string    `json:"error,omitempty"`
}

// Status reports whether a sweep is in flight and the last finished pass.
type Status struct {
	Running bool        `json:"running"`
	LastRun *RunSummary `json:"last_run"`
}

// AutoService periodically downloads subtitles for videos that do not have a
// sidecar subtitle yet. It walks the library in location-ID order so each pass
// continues where the previous one stopped.
type AutoService struct {
	gateway   Gateway
	locations LocationSource
	config    ConfigSource

	interval        time.Duration
	batchSize       int
	failureCooldown time.Duration
	now             func() time.Time

	mu       sync.Mutex
	cursor   int64
	failures map[int64]time.Time
	running  bool
	lastRun  *RunSummary
}

// NewAutoService constructs an AutoService. The caller must start it with Start.
func NewAutoService(gateway Gateway, locations LocationSource, config ConfigSource) *AutoService {
	return &AutoService{
		gateway:         gateway,
		locations:       locations,
		config:          config,
		interval:        autoSweepInterval,
		batchSize:       autoSweepBatchSize,
		failureCooldown: autoFailureCooldown,
		now:             time.Now,
		failures:        make(map[int64]time.Time),
	}
}

var (
	defaultMu      sync.Mutex
	defaultService *AutoService
)

// StartAutoSweep starts the background sweep against the application database
// and registers it as the default so the API can trigger passes and read status.
func StartAutoSweep(ctx context.Context) *AutoService {
	client := NewClient()
	client.HTTP.Timeout = autoDownloadTimeout
	store := db.Default()
	service := NewAutoService(client, store, store)
	SetDefault(service)
	service.Start(ctx)
	return service
}

// SetDefault registers the sweep the API layer triggers and reports on.
func SetDefault(service *AutoService) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultService = service
}

// Default returns the registered sweep, or nil when no sweep has been started.
func Default() *AutoService {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	return defaultService
}

// Start runs the sweep immediately and then on every interval until ctx is done.
func (s *AutoService) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			if _, err := s.Sweep(ctx); err != nil && ctx.Err() == nil && !errors.Is(err, ErrSweepRunning) {
				logging.Error("auto subtitle sweep failed: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// Sweep performs one pass over the next batch of videos, downloading subtitles
// for those that have none. Videos that fail are skipped for a cooldown period.
func (s *AutoService) Sweep(ctx context.Context) (AutoSweepResult, error) {
	return s.runSweep(ctx, false)
}

// RunNow performs one pass even when the automatic sweep is disabled, so the
// settings screen can trigger a sweep on demand.
func (s *AutoService) RunNow(ctx context.Context) (AutoSweepResult, error) {
	return s.runSweep(ctx, true)
}

// RunAsync starts a forced sweep in the background and reports whether it
// started. The pass keeps running after the triggering request finishes.
func (s *AutoService) RunAsync(ctx context.Context) bool {
	startedAt := s.beginRun()
	if startedAt.IsZero() {
		return false
	}
	go func() {
		result, err := s.sweep(context.WithoutCancel(ctx), true)
		s.finishRun(startedAt, result, err)
		if err != nil {
			logging.Error("auto subtitle sweep failed: %v", err)
		}
	}()
	return true
}

// Status reports whether a sweep is running and the last finished pass.
func (s *AutoService) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := Status{Running: s.running}
	if s.lastRun != nil {
		summary := *s.lastRun
		status.LastRun = &summary
	}
	return status
}

func (s *AutoService) runSweep(ctx context.Context, force bool) (AutoSweepResult, error) {
	startedAt := s.beginRun()
	if startedAt.IsZero() {
		return AutoSweepResult{}, ErrSweepRunning
	}
	result, err := s.sweep(ctx, force)
	if err == nil && !force && !result.Enabled {
		// A disabled sweep changes nothing, so keep the previous summary.
		s.endRun()
		return result, nil
	}
	s.finishRun(startedAt, result, err)
	return result, err
}

// beginRun marks a sweep as in flight and returns its start time, or the zero
// time when another sweep already holds the slot.
func (s *AutoService) beginRun() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return time.Time{}
	}
	s.running = true
	return s.now()
}

func (s *AutoService) endRun() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
}

func (s *AutoService) finishRun(startedAt time.Time, result AutoSweepResult, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
	summary := &RunSummary{
		StartedAt:  startedAt,
		FinishedAt: s.now(),
		Enabled:    result.Enabled,
		Language:   result.Language,
		Scanned:    result.Scanned,
		Downloaded: result.Downloaded,
		Skipped:    result.Skipped,
		Failed:     result.Failed,
	}
	if err != nil {
		summary.Error = err.Error()
	}
	s.lastRun = summary
}

func (s *AutoService) sweep(ctx context.Context, force bool) (AutoSweepResult, error) {
	result := AutoSweepResult{Language: DefaultAutoLanguage}

	cfg, err := s.config.ListConfig(ctx)
	if err != nil {
		return result, fmt.Errorf("load auto subtitle settings: %w", err)
	}
	result.Enabled = parseConfigBool(cfg[AutoEnabledKey])
	if !result.Enabled && !force {
		return result, nil
	}
	if language := strings.TrimSpace(cfg[AutoLanguageKey]); language != "" {
		result.Language = language
	}

	pingCtx, cancel := context.WithTimeout(ctx, autoGatewayPingLimit)
	err = s.gateway.Ping(pingCtx)
	cancel()
	if err != nil {
		return result, fmt.Errorf("auto subtitle gateway is unavailable: %w", err)
	}

	recent, backlog, err := s.nextBatches(ctx)
	if err != nil {
		return result, err
	}
	backlogIDs := make(map[int64]struct{}, len(backlog))
	for _, loc := range backlog {
		backlogIDs[loc.ID] = struct{}{}
	}
	seen := make(map[int64]struct{}, len(recent)+len(backlog))
	locations := make([]models.VideoLocation, 0, len(recent)+len(backlog))
	for _, batch := range [][]models.VideoLocation{recent, backlog} {
		for _, loc := range batch {
			if _, ok := seen[loc.ID]; ok {
				continue
			}
			seen[loc.ID] = struct{}{}
			locations = append(locations, loc)
		}
	}
	result.Scanned = len(locations)
	for _, loc := range locations {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if _, ok := backlogIDs[loc.ID]; ok {
			s.advanceCursor(loc.ID)
		}
		downloaded, skipped, downloadErr := s.processLocation(ctx, loc, result.Language)
		if downloadErr != nil {
			result.Failed++
			s.markFailure(loc.ID)
			logging.Error("auto subtitle: %s: %v", locationPath(loc), downloadErr)
			continue
		}
		if skipped {
			result.Skipped++
		}
		if downloaded {
			result.Downloaded++
		}
	}
	return result, nil
}

func (s *AutoService) processLocation(ctx context.Context, loc models.VideoLocation, language string) (downloaded, skipped bool, err error) {
	path := locationPath(loc)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, true, nil
		}
		return false, false, fmt.Errorf("inspect video: %w", err)
	}
	if info.IsDir() {
		return false, true, nil
	}

	hasSubtitle, err := hasSubtitleSidecar(path)
	if err != nil {
		return false, false, fmt.Errorf("inspect subtitles: %w", err)
	}
	if hasSubtitle || s.inFailureCooldown(loc.ID) {
		return false, true, nil
	}

	downloadCtx, cancel := context.WithTimeout(ctx, autoDownloadTimeout)
	defer cancel()
	result, err := s.gateway.Download(downloadCtx, DownloadRequest{
		Video: Video{
			Path:       path,
			Name:       loc.Filename,
			SizeBytes:  info.Size(),
			ModifiedAt: float64(info.ModTime().Unix()),
		},
		Language: language,
	})
	if err != nil {
		return false, false, err
	}
	if !result.Success {
		message := strings.TrimSpace(result.Error)
		if message == "" {
			message = "no subtitle found"
		}
		return false, false, errors.New(message)
	}

	s.clearFailure(loc.ID)
	logging.Info("auto subtitle: downloaded %s for %s", result.SubtitlePath, loc.Filename)
	return true, false, nil
}

func (s *AutoService) nextBatches(ctx context.Context) (recent, backlog []models.VideoLocation, err error) {
	recent, err = s.locations.ListRecentAutoSubtitleLocations(ctx, s.batchSize)
	if err != nil {
		return nil, nil, fmt.Errorf("list recent auto subtitle videos: %w", err)
	}

	s.mu.Lock()
	cursor := s.cursor
	s.mu.Unlock()

	backlog, err = s.locations.ListAutoSubtitleLocations(ctx, cursor, s.batchSize)
	if err != nil {
		return nil, nil, fmt.Errorf("list auto subtitle videos: %w", err)
	}
	if len(backlog) == 0 && cursor > 0 {
		s.mu.Lock()
		s.cursor = 0
		s.mu.Unlock()
		backlog, err = s.locations.ListAutoSubtitleLocations(ctx, 0, s.batchSize)
		if err != nil {
			return nil, nil, fmt.Errorf("list auto subtitle videos: %w", err)
		}
	}
	return recent, backlog, nil
}

func (s *AutoService) advanceCursor(locationID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if locationID > s.cursor {
		s.cursor = locationID
	}
}

func (s *AutoService) markFailure(locationID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for id, at := range s.failures {
		if now.Sub(at) >= s.failureCooldown {
			delete(s.failures, id)
		}
	}
	s.failures[locationID] = now
}

func (s *AutoService) clearFailure(locationID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.failures, locationID)
}

func (s *AutoService) inFailureCooldown(locationID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	at, ok := s.failures[locationID]
	if !ok {
		return false
	}
	if s.now().Sub(at) >= s.failureCooldown {
		delete(s.failures, locationID)
		return false
	}
	return true
}

// hasSubtitleSidecar reports whether the video already has a subtitle file in
// its directory, matching either "video.srt" or "video.<tag>.srt" naming.
func hasSubtitleSidecar(videoPath string) (bool, error) {
	dir := filepath.Dir(videoPath)
	base := filepath.Base(videoPath)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	prefix := strings.ToLower(stem) + "."

	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, fmt.Errorf("read video directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if _, ok := subtitleExtensions[strings.ToLower(filepath.Ext(name))]; !ok {
			continue
		}
		subtitleStem := strings.TrimSuffix(name, filepath.Ext(name))
		if strings.EqualFold(subtitleStem, stem) || strings.HasPrefix(strings.ToLower(subtitleStem), prefix) {
			return true, nil
		}
	}
	return false, nil
}

func locationPath(loc models.VideoLocation) string {
	return filepath.Join(loc.DirectoryRef.Path, filepath.FromSlash(loc.RelativePath))
}

func parseConfigBool(value string) bool {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	return err == nil && parsed
}
