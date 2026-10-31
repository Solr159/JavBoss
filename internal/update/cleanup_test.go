package update

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"javboss/internal/util"

	"github.com/shirou/gopsutil/v4/process"
)

func runningHelper(t *testing.T) (helperIdentity, *exec.Cmd) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(self, "-test.run=^TestCleanupIdleSubprocess$")
	cmd.Env = append(os.Environ(), "JAVBOSS_TEST_CLEANUP_CHILD=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	p, err := process.NewProcess(int32(cmd.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	createdAt, err := p.CreateTime()
	if err != nil {
		t.Fatal(err)
	}
	return helperIdentity{PID: p.Pid, CreatedAt: createdAt}, cmd
}

func exitedHelper(t *testing.T) helperIdentity {
	t.Helper()
	identity, cmd := runningHelper(t)
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	return identity
}

func TestCleanupIdleSubprocess(t *testing.T) {
	if os.Getenv("JAVBOSS_TEST_CLEANUP_CHILD") == "" {
		t.Skip("subprocess only")
	}
	time.Sleep(30 * time.Second)
}

func cleanupFixture(t *testing.T, base, name string, identity helperIdentity, result *Result) string {
	t.Helper()
	stage := filepath.Join(base, ".updates", name)
	putFile(t, stage, "backup/database.sqlite", "database backup")
	putFile(t, stage, "helper", "helper binary")
	if err := writeJSON(filepath.Join(stage, "helper.json"), identity); err != nil {
		t.Fatal(err)
	}
	if result != nil {
		if err := writeJSON(filepath.Join(stage, "result.json"), result); err != nil {
			t.Fatal(err)
		}
	}
	return stage
}

func TestCleanupPreservesFailedAndIncompleteUpdates(t *testing.T) {
	identity := exitedHelper(t)
	for _, tc := range []struct {
		name   string
		result *Result
	}{
		{"failed", &Result{Version: "v2.0.0", FinishedAt: 1, Error: "startup timed out"}},
		{"incomplete", nil},
		{"invalid result", &Result{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			stage := cleanupFixture(t, base, "update-retained", identity, tc.result)
			putFile(t, stage, "application-ready", "v2.0.0")
			completed := cleanupFixture(t, base, "update-completed", identity, &Result{Version: "v2.0.0", FinishedAt: 1})
			if err := cleanupCompletedUpdates(t.Context(), base); err != nil {
				t.Fatal(err)
			}
			requireFile(t, stage, "backup/database.sqlite", "database backup")
			if _, err := os.Stat(completed); !os.IsNotExist(err) {
				t.Fatalf("successful stage was not removed: %v", err)
			}
		})
	}
}

func TestCleanupWaitsForHelperExit(t *testing.T) {
	base := t.TempDir()
	identity, helper := runningHelper(t)
	stage := cleanupFixture(t, base, "update-completed", identity, &Result{Version: "v2.0.0", FinishedAt: 1})
	putFile(t, stage, "application-ready", "v2.0.0")
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if err := cleanupUpdate(ctx, base, stage); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cleanup did not wait for the helper: %v", err)
	}
	requireFile(t, stage, "application-ready", "v2.0.0")
	requireFile(t, stage, "backup/database.sqlite", "database backup")
	if err := helper.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = helper.Wait()
	if err := cleanupUpdate(t.Context(), base, stage); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(base, ".updates")); !os.IsNotExist(err) {
		t.Fatalf("workspace remained after helper exit: %v", err)
	}
}

func TestCleanupDoesNotWaitForReusedHelperPID(t *testing.T) {
	base := t.TempDir()
	identity, unrelated := runningHelper(t)
	identity.CreatedAt-- // The recorded helper predates this process with the same PID.
	stage := cleanupFixture(t, base, "update-completed", identity, &Result{Version: "v2.0.0", FinishedAt: 1})
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := cleanupUpdate(ctx, base, stage); err != nil {
		t.Fatal(err)
	}
	if exists, err := process.PidExists(int32(unrelated.Process.Pid)); err != nil || !exists {
		t.Fatalf("unrelated process was affected: exists=%v err=%v", exists, err)
	}
}

func TestCleanupWaitsForOperationLockAndPreservesNewUpdate(t *testing.T) {
	base := t.TempDir()
	stage := cleanupFixture(t, base, "update-completed", exitedHelper(t), &Result{Version: "v2.0.0", FinishedAt: 1})
	lock, err := acquireUpdateLock(base)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if err := cleanupUpdate(ctx, base, stage); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cleanup ignored operation lock: %v", err)
	}
	requireFile(t, stage, "helper", "helper binary")
	putFile(t, base, ".updates/update-new/release.zip", "new download")
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
	if err := cleanupUpdate(t.Context(), base, stage); err != nil {
		t.Fatal(err)
	}
	requireFile(t, base, ".updates/update-new/release.zip", "new download")
	// The same persistent lock still serializes later downloads.
	next, err := acquireUpdateLock(base)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Release()
	if another, err := acquireUpdateLock(base); !errors.Is(err, util.ErrLockHeld) {
		if another != nil {
			_ = another.Release()
		}
		t.Fatalf("operation lock lost exclusivity: %v", err)
	}
}

func TestOrdinaryStartupCleansCompletedWorkspace(t *testing.T) {
	base := t.TempDir()
	cleanupFixture(t, base, "update-completed", exitedHelper(t), &Result{Version: "v2.0.0", FinishedAt: 1})
	putFile(t, base, "data/update-result.json", `{"version":"v2.0.0","finished_at":1}`)
	t.Setenv(readyEnvironment, "")
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	AcknowledgeStartup(ctx, base, "v2.0.0")
	for {
		if _, err := os.Stat(filepath.Join(base, ".updates")); os.IsNotExist(err) {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
	requireFile(t, base, "data/update-result.json", `{"version":"v2.0.0","finished_at":1}`)
}

func TestCleanupRejectsUnsafeWorkspaces(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()
	putFile(t, outside, "keep", "user data")
	if err := cleanupUpdate(t.Context(), base, outside); err == nil {
		t.Fatal("accepted workspace outside installation")
	}
	if err := os.MkdirAll(filepath.Join(base, ".updates"), 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, ".updates", "update-link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := cleanupUpdate(t.Context(), base, link); err == nil {
		t.Fatal("accepted symlink workspace")
	}
	requireFile(t, outside, "keep", "user data")
}
