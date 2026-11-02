package update

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Exercise real process handoff, lock release, replacement, restart arguments,
// and readiness without downloading or replacing the developer's installation.
func TestHelperReplacesOnlyAfterParentExitAndRestarts(t *testing.T) {
	// Executable and startup paths are canonicalized by the OS and updater.
	// Keep the plan consistent when the temporary directory has symlink ancestors.
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(base, ".updates", "update-test")
	payload := filepath.Join(stage, "payload")
	putFile(t, payload, "web/dist/index.html", "new frontend")
	putFile(t, base, "web/dist/index.html", "old frontend")
	putFile(t, base, "config.toml", "user settings")
	putFile(t, base, executableName(), "old executable")
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := copyFile(self, filepath.Join(payload, executableName()), 0755); err != nil {
		t.Fatal(err)
	}
	helperPath := filepath.Join(stage, executableName())
	if err := copyFile(self, helperPath, 0755); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(stage, "plan.json")
	plan := installPlan{BaseDir: base, StageDir: stage, Version: "v2.0.0", Args: []string{"-test.run=^TestUpdateStartedSubprocess$"}}
	if err := writeJSON(planPath, plan); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "JAVBOSS_TEST_UPDATE_PLAN="+planPath)
	parent := exec.Command(self, "-test.run=^TestUpdateParentSubprocess$")
	parent.Env = env
	if err := parent.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = parent.Process.Kill() })
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	if err := waitFile(ctx, filepath.Join(stage, "parent-ready"), "ready"); err != nil {
		t.Fatal(err)
	}
	plan.ParentPID = parent.Process.Pid
	if err := writeJSON(planPath, plan); err != nil {
		t.Fatal(err)
	}
	helper := exec.Command(helperPath, "-test.run=^TestUpdateHelperSubprocess$")
	helper.Env = env
	if err := helper.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = helper.Process.Kill() })
	if err := waitFile(ctx, filepath.Join(stage, "helper-ready"), "ready"); err != nil {
		t.Fatal(err)
	}
	requireFile(t, base, executableName(), "old executable")
	requireFile(t, base, "web/dist/index.html", "old frontend")
	putFile(t, stage, "parent-exit", "exit")
	if err := parent.Wait(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- helper.Wait() }()
	// The helper has recorded success, but is deliberately kept alive. The new
	// process must not delete its executable, acknowledgement, or backups yet.
	for {
		var result Result
		if readJSON(filepath.Join(stage, "result.json"), &result) == nil && result.FinishedAt > 0 {
			if result.Error != "" {
				t.Fatal(result.Error)
			}
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
	requireFile(t, stage, "application-ready", "v2.0.0")
	requireFile(t, stage, "backup/files/"+executableName(), "old executable")
	putFile(t, stage, "allow-helper-exit", "exit")
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err := waitFile(ctx, filepath.Join(base, "data", "cleanup-done"), "done"); err != nil {
		t.Fatal(err)
	}
	requireFile(t, base, "web/dist/index.html", "new frontend")
	requireFile(t, base, "config.toml", "user settings")
	requireFile(t, base, ".version", "v2.0.0\n")
	if _, err := os.Stat(filepath.Join(base, ".updates")); !os.IsNotExist(err) {
		t.Fatalf("update workspace remained after helper exit: %v", err)
	}
	requireFile(t, base, "data/update-operation.lock", "")
	var result Result
	if err := readJSON(filepath.Join(base, "data/update-result.json"), &result); err != nil || result.Error != "" || result.Version != "v2.0.0" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func subprocessPlan(t *testing.T) (string, installPlan) {
	t.Helper()
	path := os.Getenv("JAVBOSS_TEST_UPDATE_PLAN")
	if path == "" {
		t.Skip("subprocess only")
	}
	var plan installPlan
	if err := readJSON(path, &plan); err != nil {
		t.Fatal(err)
	}
	return path, plan
}

func TestUpdateParentSubprocess(t *testing.T) {
	_, plan := subprocessPlan(t)
	lock, err := AcquireRuntimeLock(plan.BaseDir)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	putFile(t, plan.StageDir, "parent-ready", "ready")
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	if err := waitFile(ctx, filepath.Join(plan.StageDir, "parent-exit"), "exit"); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateHelperSubprocess(t *testing.T) {
	path, plan := subprocessPlan(t)
	if err := RunHelper(path); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	if err := waitFile(ctx, filepath.Join(plan.StageDir, "allow-helper-exit"), "exit"); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateStartedSubprocess(t *testing.T) {
	_, plan := subprocessPlan(t)
	lock, err := AcquireRuntimeLock(plan.BaseDir)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	requireFile(t, plan.BaseDir, "config.toml", "user settings")
	requireFile(t, plan.StageDir, "backup/files/"+executableName(), "old executable")
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	AcknowledgeStartup(ctx, plan.BaseDir, "v2.0.0")
	for {
		if _, err := os.Stat(plan.StageDir); os.IsNotExist(err) {
			putFile(t, plan.BaseDir, "data/cleanup-done", "done")
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
}
