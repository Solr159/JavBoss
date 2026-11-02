package update

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"javboss/internal/util"

	_ "github.com/mattn/go-sqlite3"
	"github.com/shirou/gopsutil/v4/process"
)

const readyEnvironment = "JAVBOSS_UPDATE_READY"

// Capture this before AcknowledgeStartup consumes the environment marker.
var restartedAfterUpdate = os.Getenv(readyEnvironment) != ""

// RestartedAfterUpdate reports whether the update helper started this process.
// Desktop startup uses it to leave the existing browser page to reconnect.
func RestartedAfterUpdate() bool {
	return restartedAfterUpdate
}

type installPlan struct {
	BaseDir      string   `json:"base_dir"`
	StageDir     string   `json:"stage_dir"`
	ParentPID    int      `json:"parent_pid"`
	Args         []string `json:"args"`
	DatabasePath string   `json:"database_path,omitempty"`
	Version      string   `json:"version"`
}

type Result struct {
	Version    string `json:"version"`
	Error      string `json:"error,omitempty"`
	FinishedAt int64  `json:"finished_at"`
}

// AcquireRuntimeLock also protects client mode. Holding this for the process
// lifetime prevents another application instance from entering during replacement.
func AcquireRuntimeLock(base string) (*util.FileLock, error) {
	if err := os.MkdirAll(filepath.Join(base, "data"), 0755); err != nil {
		return nil, err
	}
	return util.AcquireFileLock(filepath.Join(base, "data", "update-runtime.lock"))
}

// The operation lock must outlive .updates so cleanup and a new download cannot
// acquire different lock files while that directory is being removed.
func acquireUpdateLock(base string) (*util.FileLock, error) {
	if err := os.MkdirAll(filepath.Join(base, "data"), 0755); err != nil {
		return nil, err
	}
	return util.AcquireFileLock(filepath.Join(base, "data", "update-operation.lock"))
}

// RunHelper must be dispatched before flags, logging, tray setup, or DB access.
func RunHelper(planPath string) error {
	var plan installPlan
	if err := readJSON(planPath, &plan); err != nil {
		return err
	}
	if err := validatePlan(planPath, plan); err != nil {
		return err
	}
	err := waitForParent(plan)
	if err == nil {
		var lock *util.FileLock
		lock, err = acquireUpdateLock(plan.BaseDir)
		if err == nil {
			defer lock.Release()
			err = applyPlan(plan)
		}
	}
	result := Result{Version: plan.Version, FinishedAt: time.Now().UnixMilli()}
	if err != nil {
		result.Error = err.Error()
	}
	if writeErr := writeJSON(filepath.Join(plan.BaseDir, "data", "update-result.json"), result); writeErr != nil {
		return errors.Join(err, writeErr)
	}
	// A stage-local result identifies exactly which workspace may be removed.
	// The new application waits for this helper to exit before inspecting it.
	if writeErr := writeJSON(filepath.Join(plan.StageDir, "result.json"), result); writeErr != nil {
		return errors.Join(err, writeErr)
	}
	return err
}

func validatePlan(planPath string, plan installPlan) error {
	if !filepath.IsAbs(plan.BaseDir) || !filepath.IsAbs(plan.StageDir) || plan.ParentPID <= 0 || !releaseTag.MatchString(plan.Version) {
		return errors.New("invalid update plan")
	}
	if filepath.Dir(plan.StageDir) != filepath.Join(plan.BaseDir, ".updates") || filepath.Clean(planPath) != filepath.Join(plan.StageDir, "plan.json") {
		return errors.New("update plan is outside staging directory")
	}
	exe, err := os.Executable()
	if err != nil || filepath.Dir(exe) != plan.StageDir {
		return errors.New("update helper must run from staging directory")
	}
	return validatePayload(filepath.Join(plan.StageDir, "payload"))
}

func waitForParent(plan installPlan) error {
	parent, err := process.NewProcess(int32(plan.ParentPID))
	if err != nil {
		return fmt.Errorf("find original application: %w", err)
	}
	// Cache creation time before telling the parent it can exit (PID reuse).
	if _, err := parent.CreateTime(); err != nil {
		return err
	}
	self, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		return err
	}
	createdAt, err := self.CreateTime()
	if err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(plan.StageDir, "helper.json"), helperIdentity{PID: self.Pid, CreatedAt: createdAt}); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(plan.StageDir, "helper-ready"), []byte("ready"), 0600); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if err := waitForExit(ctx, parent); err != nil {
		return fmt.Errorf("application did not exit; no files changed: %w", err)
	}
	return nil
}

func applyPlan(plan installPlan) error {
	lock, err := AcquireRuntimeLock(plan.BaseDir)
	if err != nil {
		return fmt.Errorf("another application instance started; no files changed: %w", err)
	}
	locked := true
	defer func() {
		if locked {
			_ = lock.Release()
		}
	}()
	backup := filepath.Join(plan.StageDir, "backup")
	restartOld := func(cause error) error {
		_ = lock.Release()
		locked = false
		cmd, startErr := startDetached(filepath.Join(plan.BaseDir, executableName()), plan.Args, plan.BaseDir, "")
		if startErr == nil {
			_ = cmd.Process.Release()
		}
		return errors.Join(cause, startErr)
	}
	if err := os.MkdirAll(backup, 0700); err != nil {
		return restartOld(err)
	}
	if plan.DatabasePath != "" {
		if err := backupDatabase(plan.DatabasePath, filepath.Join(backup, "database.sqlite")); err != nil {
			return restartOld(fmt.Errorf("database backup failed; no files changed: %w", err))
		}
	}
	payload := filepath.Join(plan.StageDir, "payload")
	if err := os.WriteFile(filepath.Join(payload, ".version"), []byte(plan.Version+"\n"), 0644); err != nil {
		return restartOld(err)
	}
	transaction, err := replacePayload(plan.BaseDir, payload, backup)
	if err != nil {
		if transaction.rollbackFailed {
			return fmt.Errorf("update recovery needs manual attention in %s: %w", backup, err)
		}
		return restartOld(fmt.Errorf("update files restored: %w", err))
	}
	_ = lock.Release()
	locked = false
	marker := filepath.Join(plan.StageDir, "application-ready")
	cmd, err := startDetached(filepath.Join(plan.BaseDir, executableName()), plan.Args, plan.BaseDir, marker)
	if err != nil {
		// Start failed before a new process existed: file-only rollback is safe.
		lock, lockErr := AcquireRuntimeLock(plan.BaseDir)
		if lockErr != nil {
			return errors.Join(err, lockErr)
		}
		rollbackErr := transaction.rollback()
		_ = lock.Release()
		if rollbackErr != nil {
			return errors.Join(err, rollbackErr)
		}
		old, startErr := startDetached(filepath.Join(plan.BaseDir, executableName()), plan.Args, plan.BaseDir, "")
		if startErr == nil {
			_ = old.Process.Release()
		}
		return errors.Join(err, startErr)
	}
	_ = cmd.Process.Release()
	startupCtx, startupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer startupCancel()
	if err := waitFile(startupCtx, marker, plan.Version); err != nil {
		return fmt.Errorf("new version did not confirm startup; backup retained in %s (database was not automatically restored): %w", backup, err)
	}
	return nil
}

func waitForExit(ctx context.Context, parent *process.Process) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		running, err := parent.IsRunning()
		if err != nil {
			return err
		}
		if !running {
			return nil
		}
		states, _ := parent.Status()
		for _, state := range states {
			if state == process.Zombie {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func backupDatabase(source, dest string) error {
	if _, err := os.Stat(source); err != nil {
		return err
	}
	database, err := sql.Open("sqlite3", databaseURI(source))
	if err != nil {
		return err
	}
	defer database.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	_, err = database.ExecContext(ctx, "VACUUM INTO ?", dest)
	return err
}

func databaseURI(source string) string {
	path := filepath.ToSlash(source)
	// A Windows drive belongs in the URI path, not its authority (file:///C:/...).
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	u := url.URL{Scheme: "file", Path: path, RawQuery: "mode=rw&_busy_timeout=5000"}
	return u.String()
}

func waitFile(ctx context.Context, name, expected string) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		data, err := os.ReadFile(name)
		if err == nil && string(data) == expected {
			return nil
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func readJSON(name string, value any) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewDecoder(io.LimitReader(f, 1<<20)).Decode(value)
}

func writeJSON(name string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(name), ".update-json-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), name)
}

func copyFile(source, dest string, mode os.FileMode) error {
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(dst, src)
	syncErr := dst.Sync()
	closeErr := dst.Close()
	return errors.Join(copyErr, syncErr, closeErr)
}
