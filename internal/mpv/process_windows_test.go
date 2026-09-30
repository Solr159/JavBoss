//go:build windows

package mpv

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func windowsJobHelperCommand(t *testing.T, role, dir string) *exec.Cmd {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-test.run=^TestWindowsPlayerJobHelper$")
	cmd.Env = append(os.Environ(), "JAVBOSS_JOB_TEST_ROLE="+role, "JAVBOSS_JOB_TEST_DIR="+dir)
	return cmd
}

func TestWindowsPlayerJobHelper(t *testing.T) {
	role := os.Getenv("JAVBOSS_JOB_TEST_ROLE")
	if role == "" {
		return
	}
	dir := os.Getenv("JAVBOSS_JOB_TEST_DIR")
	switch role {
	case "parent":
		// No deferred cleanup: the test forcibly terminates this parent.
		if _, _, err := startPlatformPlayer(windowsJobHelperCommand(t, "player", dir)); err != nil {
			t.Fatal(err)
		}
	case "player":
		if err := windowsJobHelperCommand(t, "thumbnail-worker", dir).Start(); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, role), []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		t.Fatal(err)
	}
	for {
		time.Sleep(time.Second)
	}
}

func TestWindowsPlayerJobKillsDescendantsOnParentExit(t *testing.T) {
	dir := t.TempDir()
	parent := windowsJobHelperCommand(t, "parent", dir)
	if err := parent.Start(); err != nil {
		t.Fatal(err)
	}
	parentDone := make(chan struct{})
	go func() { _ = parent.Wait(); close(parentDone) }()
	t.Cleanup(func() { _ = parent.Process.Kill(); <-parentDone })
	var descendants []windows.Handle
	for _, role := range []string{"player", "thumbnail-worker"} {
		path := filepath.Join(dir, role)
		waitForPlayerTestFile(t, path)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		pid, err := strconv.Atoi(string(raw))
		if err != nil {
			t.Fatal(err)
		}
		handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_TERMINATE, false, uint32(pid))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = windows.TerminateProcess(handle, 1); _ = windows.CloseHandle(handle) })
		descendants = append(descendants, handle)
	}
	if err := parent.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	<-parentDone
	for i, handle := range descendants {
		status, err := windows.WaitForSingleObject(handle, 5000)
		if err != nil || status != windows.WAIT_OBJECT_0 {
			t.Fatal(fmt.Sprintf("descendant %d survived parent exit: status=%d err=%v", i, status, err))
		}
	}
}
