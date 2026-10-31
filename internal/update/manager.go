// Package update checks official releases and stages desktop updates. The helper
// runs before normal application initialization and never opens the application DB
// through db.Open (which would execute migrations).
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/util"

	"golang.org/x/mod/semver"
)

type Status struct {
	CurrentVersion string  `json:"current_version"`
	LatestVersion  string  `json:"latest_version,omitempty"`
	Available      bool    `json:"available"`
	CanInstall     bool    `json:"can_install"`
	DisabledReason string  `json:"disabled_reason,omitempty"`
	Phase          string  `json:"phase"`
	ReleaseURL     string  `json:"release_url,omitempty"`
	Notes          string  `json:"notes,omitempty"`
	CheckedAt      int64   `json:"checked_at,omitempty"`
	Downloaded     int64   `json:"downloaded_bytes"`
	Total          int64   `json:"total_bytes"`
	Error          string  `json:"error,omitempty"`
	LastResult     *Result `json:"last_result,omitempty"`
}

type Options struct {
	BaseDir      string
	DatabasePath string // Empty for client mode.
	Version      string
	Release      bool
	Container    bool
	Stop         context.CancelFunc
}

type Manager struct {
	mu       sync.Mutex
	ctx      context.Context
	options  Options
	client   *http.Client
	endpoint string
	platform string
	status   Status
	selected asset
}

var active atomic.Pointer[Manager]

func platformOS() string { return runtime.GOOS }

func newManager(ctx context.Context, options Options) *Manager {
	m := &Manager{ctx: ctx, options: options, client: util.NewHTTPClient(0),
		endpoint: "https://github.com/" + repository + "/releases/latest", platform: platformName(runtime.GOOS, runtime.GOARCH),
		status: Status{CurrentVersion: options.Version, Phase: "idle"}}
	switch {
	case !options.Release || !releaseTag.MatchString(options.Version):
		m.status.DisabledReason = "development"
	case options.Container:
		m.status.DisabledReason = "container"
	case m.platform == "":
		m.status.DisabledReason = "platform"
	}
	return m
}

// Init installs the process-local updater. Checks never close the application;
// installing always requires the explicit POST from the confirmation dialog.
func Init(ctx context.Context, options Options) {
	m := newManager(ctx, options)
	active.Store(m)
	if m.status.DisabledReason == "development" {
		return
	}
	go func() {
		m.StartCheck()
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.StartCheck()
			}
		}
	}()
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	result := m.status
	m.mu.Unlock()
	var last Result
	if readJSON(filepath.Join(m.options.BaseDir, "data", "update-result.json"), &last) == nil {
		result.LastResult = &last
	}
	return result
}

func busy(phase string) bool { return phase != "idle" && phase != "error" }

func (m *Manager) StartCheck() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if busy(m.status.Phase) {
		return errors.New("an update operation is already running")
	}
	if m.ctx.Err() != nil {
		return m.ctx.Err()
	}
	m.status.Phase, m.status.Error = "checking", ""
	go m.check()
	return nil
}

func (m *Manager) check() {
	ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
	defer cancel()
	r, err := latestRelease(ctx, m.client, m.endpoint)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.Phase = "idle"
	m.status.CanInstall = false
	m.selected = asset{}
	if err != nil {
		m.status.Phase, m.status.Error = "error", err.Error()
		return
	}
	m.status.CheckedAt = time.Now().UnixMilli()
	m.status.Available, m.status.CanInstall = false, false
	m.status.LatestVersion, m.status.Notes, m.status.ReleaseURL = "", "", ""
	if r == nil {
		return
	}
	m.status.LatestVersion = r.Tag
	m.status.ReleaseURL = "https://github.com/" + repository + "/releases/tag/" + r.Tag
	m.status.Available = semver.IsValid(m.options.Version) && semver.Compare(r.Tag, m.options.Version) > 0
	if !m.status.Available || m.status.DisabledReason != "" {
		return
	}
	selected, err := releaseAsset(r, m.platform)
	if err != nil {
		m.status.Error = err.Error()
		return
	}
	m.selected, m.status.CanInstall = selected, true
}

func (m *Manager) StartInstall() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if busy(m.status.Phase) {
		return errors.New("an update operation is already running")
	}
	if !m.status.CanInstall || !m.status.Available || m.options.Stop == nil {
		return errors.New("no installable update is available")
	}
	if m.ctx.Err() != nil {
		return m.ctx.Err()
	}
	a, version := m.selected, m.status.LatestVersion
	m.status.Phase, m.status.Error = "downloading", ""
	m.status.Downloaded, m.status.Total = 0, 0
	go func() {
		if err := m.prepare(a, version); err != nil {
			logging.Error("application update failed: %v", err)
			m.mu.Lock()
			m.status.Phase, m.status.Error = "error", err.Error()
			m.mu.Unlock()
		}
	}()
	return nil
}

func (m *Manager) prepare(a asset, version string) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return err
	}
	base, err := filepath.EvalSymlinks(m.options.BaseDir)
	if err != nil {
		return err
	}
	if executable != filepath.Join(base, executableName()) {
		return errors.New("updating a renamed or relocated executable is not supported")
	}
	updates := filepath.Join(base, ".updates")
	if err := os.MkdirAll(updates, 0700); err != nil {
		return fmt.Errorf("create update directory: %w", err)
	}
	lock, err := util.AcquireFileLock(filepath.Join(updates, "download.lock"))
	if err != nil {
		return fmt.Errorf("another updater is active: %w", err)
	}
	defer lock.Release()
	stage, err := os.MkdirTemp(updates, "update-")
	if err != nil {
		return err
	}
	handedOff := false
	defer func() {
		if !handedOff {
			os.RemoveAll(stage)
		}
	}()
	ctx, cancel := context.WithTimeout(m.ctx, 30*time.Minute)
	defer cancel()
	archive := filepath.Join(stage, "release.zip")
	if err := downloadArchive(ctx, m.client, a, archive, func(n, total int64) {
		m.mu.Lock()
		m.status.Downloaded, m.status.Total = n, total
		m.mu.Unlock()
	}); err != nil {
		return err
	}
	m.mu.Lock()
	m.status.Phase = "preparing"
	m.mu.Unlock()
	payload := filepath.Join(stage, "payload")
	if err := extractArchive(ctx, archive, payload, "javboss-"+version+"-"+m.platform); err != nil {
		return err
	}
	if err := validatePayload(payload); err != nil {
		return err
	}
	if err := os.Remove(archive); err != nil {
		return err
	}
	helper := filepath.Join(stage, "helper")
	if runtime.GOOS == "windows" {
		helper += ".exe"
	}
	if err := copyFile(executable, helper, 0700); err != nil {
		return err
	}
	plan := installPlan{BaseDir: base, StageDir: stage, ParentPID: os.Getpid(), Args: os.Args[1:],
		DatabasePath: m.options.DatabasePath, Version: version}
	planPath := filepath.Join(stage, "plan.json")
	if err := writeJSON(planPath, plan); err != nil {
		return err
	}
	cmd, err := startDetached(helper, []string{"--apply-update", planPath}, stage, "")
	if err != nil {
		return err
	}
	readyCtx, readyCancel := context.WithTimeout(ctx, 15*time.Second)
	defer readyCancel()
	if err := waitFile(readyCtx, filepath.Join(stage, "helper-ready"), "ready"); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return fmt.Errorf("update helper did not start: %w", err)
	}
	_ = cmd.Process.Release()
	handedOff = true
	m.mu.Lock()
	m.status.Phase = "restarting"
	m.mu.Unlock()
	// Exactly the same cancellation used by the tray/keyboard Exit action.
	m.options.Stop()
	return nil
}

// ServeHTTP is shared by authenticated server routes and local client routes.
// The caller owns authentication and same-origin checks. No URL/path is accepted
// from the request: installation only uses the release selected by this process.
func ServeHTTP(w http.ResponseWriter, r *http.Request, prefix string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	m := active.Load()
	if m == nil {
		writeHTTPError(w, http.StatusServiceUnavailable, "更新服务不可用", "Updater is unavailable")
		return
	}
	var err error
	switch {
	case r.Method == http.MethodGet && r.URL.Path == prefix:
	case r.Method == http.MethodPost && r.URL.Path == prefix+"/check":
		err = m.StartCheck()
	case r.Method == http.MethodPost && r.URL.Path == prefix+"/install":
		err = m.StartInstall()
	default:
		writeHTTPError(w, http.StatusNotFound, "接口不存在", "Endpoint not found")
		return
	}
	if err != nil {
		writeHTTPError(w, http.StatusConflict, "无法执行更新操作："+err.Error(), err.Error())
		return
	}
	if r.Method == http.MethodPost {
		w.WriteHeader(http.StatusAccepted)
	}
	_ = json.NewEncoder(w).Encode(m.Status())
}

func writeHTTPError(w http.ResponseWriter, status int, zh, en string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error_zh": zh, "error_en": en})
}
