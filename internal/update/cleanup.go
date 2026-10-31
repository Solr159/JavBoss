package update

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/util"

	"github.com/shirou/gopsutil/v4/process"
)

type helperIdentity struct {
	PID       int32 `json:"pid"`
	CreatedAt int64 `json:"created_at"`
}

// AcknowledgeStartup confirms initialization to the helper. Cleanup belongs to
// the application and runs only after the helper has recorded success and exited.
// Ordinary startups also retry cleanup interrupted by an earlier shutdown.
func AcknowledgeStartup(ctx context.Context, base, version string) {
	marker := os.Getenv(readyEnvironment)
	_ = os.Unsetenv(readyEnvironment)
	if resolved, err := filepath.EvalSymlinks(base); err == nil {
		base = resolved
	}
	var stage string
	if marker != "" {
		stage = filepath.Dir(marker)
		if filepath.Base(marker) != "application-ready" || validateCleanupStage(base, stage) != nil {
			logging.Error("invalid update startup marker: %s", marker)
			return
		}
		if err := os.WriteFile(marker, []byte(version), 0600); err != nil {
			logging.Error("acknowledge update startup: %v", err)
			return
		}
	}
	go func() {
		cleanupCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		if stage != "" {
			if err := cleanupUpdate(cleanupCtx, base, stage); err != nil {
				logging.Error("clean completed update %s: %v", stage, err)
			}
		}
		if err := cleanupCompletedUpdates(cleanupCtx, base); err != nil {
			logging.Error("clean completed updates: %v", err)
		}
	}()
}

func validateCleanupStage(base, stage string) error {
	updates := filepath.Join(base, ".updates")
	if !filepath.IsAbs(stage) || filepath.Clean(stage) != stage || filepath.Dir(stage) != updates || !strings.HasPrefix(filepath.Base(stage), "update-") {
		return fmt.Errorf("update workspace is outside staging directory")
	}
	for _, dir := range []string{updates, stage} {
		info, err := os.Lstat(dir)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("update workspace is not a regular directory: %s", dir)
		}
	}
	return nil
}

func waitForHelperExit(ctx context.Context, identity helperIdentity) error {
	if identity.PID <= 0 || identity.CreatedAt <= 0 || identity.PID == int32(os.Getpid()) {
		return fmt.Errorf("invalid update helper identity")
	}
	helper, err := process.NewProcessWithContext(ctx, identity.PID)
	if errors.Is(err, process.ErrorProcessNotRunning) {
		return nil
	}
	if err != nil {
		return err
	}
	createdAt, err := helper.CreateTimeWithContext(ctx)
	if err != nil {
		return err
	}
	if createdAt != identity.CreatedAt {
		return nil // The PID has been reused; the original helper has exited.
	}
	return waitForExit(ctx, helper)
}

func waitForUpdateLock(ctx context.Context, base string) (*util.FileLock, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		lock, err := acquireUpdateLock(base)
		if !errors.Is(err, util.ErrLockHeld) {
			return lock, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func cleanupUpdate(ctx context.Context, base, stage string) error {
	if err := validateCleanupStage(base, stage); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var identity helperIdentity
	if err := readJSON(filepath.Join(stage, "helper.json"), &identity); err != nil {
		return err
	}
	if err := waitForHelperExit(ctx, identity); err != nil {
		return err
	}
	lock, err := waitForUpdateLock(ctx, base)
	if err != nil {
		return err
	}
	defer lock.Release()
	if err := validateCleanupStage(base, stage); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var result Result
	if err := readJSON(filepath.Join(stage, "result.json"), &result); err != nil {
		return err
	}
	if result.Error != "" || result.FinishedAt <= 0 || !releaseTag.MatchString(result.Version) {
		return nil // Failed or incomplete updates retain their recovery files.
	}
	if err := os.RemoveAll(stage); err != nil {
		return err
	}
	return removeEmptyUpdates(base)
}

// Call only while holding the operation lock. Never recursively remove .updates:
// a failed or unfinished workspace must remain available for recovery.
func removeEmptyUpdates(base string) error {
	updates := filepath.Join(base, ".updates")
	entries, err := os.ReadDir(updates)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || len(entries) > 0 {
		return err
	}
	return os.Remove(updates)
}

func cleanupCompletedUpdates(ctx context.Context, base string) error {
	updates := filepath.Join(base, ".updates")
	entries, err := os.ReadDir(updates)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var failures []error
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "update-") {
			continue
		}
		stage := filepath.Join(updates, entry.Name())
		var result Result
		if readJSON(filepath.Join(stage, "result.json"), &result) != nil || result.Error != "" || result.FinishedAt <= 0 {
			continue
		}
		if err := cleanupUpdate(ctx, base, stage); err != nil {
			failures = append(failures, err)
		}
	}
	lock, err := waitForUpdateLock(ctx, base)
	if err != nil {
		return errors.Join(append(failures, err)...)
	}
	defer lock.Release()
	return errors.Join(append(failures, removeEmptyUpdates(base))...)
}
