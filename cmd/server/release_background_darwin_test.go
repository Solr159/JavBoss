package main

import (
	"encoding/json"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

const backgroundTestModeEnv = "JAVBOSS_BACKGROUND_TEST_MODE"

type backgroundTestState struct {
	Directory string
	Args      []string
	StdinEOF  bool
	Marker    string
}

// This subprocess exercises the same re-exec marker and signal handling as main,
// without starting HTTP, AppKit or a browser in the unit test suite.
func TestReleaseBackgroundChild(t *testing.T) {
	mode := os.Getenv(backgroundTestModeEnv)
	if mode == "" {
		return
	}
	if mode == "exit" {
		os.Exit(0)
	}
	if mode == "fail" {
		os.Exit(17)
	}
	buildMode = "release"
	background, err := startReleaseInBackground(".")
	if err != nil || background {
		os.Exit(18)
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM)
	dir, _ := os.Getwd()
	_, stdinErr := os.Stdin.Read(make([]byte, 1))
	state := backgroundTestState{dir, os.Args[3:], stdinErr == io.EOF, os.Getenv(backgroundChildEnv)}
	data, _ := json.Marshal(state)
	if err := os.WriteFile("child.json", data, 0o600); err != nil {
		os.Exit(19)
	}
	_, _ = os.Stdout.WriteString("background stdout\n")
	_, _ = os.Stderr.WriteString("background stderr\n")
	<-signals
	_ = os.WriteFile("stopped", nil, 0o600)
	os.Exit(0)
}

func TestDetachedReleaseSurvivesHangup(t *testing.T) {
	t.Setenv(backgroundTestModeEnv, "run")
	dir := filepath.Join(t.TempDir(), "directory with spaces")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"--server-url", "http://localhost:8655/path with spaces", "$(touch unexpected)"}
	pid, err := launchDetachedRelease(dir, executable, append([]string{"-test.run=^TestReleaseBackgroundChild$", "--"}, args...))
	if err != nil || pid == 0 {
		t.Fatalf("background launch: pid=%d, err=%v", pid, err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(pid, syscall.SIGTERM)
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(filepath.Join(dir, "stopped")); err == nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		_ = syscall.Kill(pid, syscall.SIGKILL)
		t.Error("background child did not shut down")
	})
	data, err := os.ReadFile(filepath.Join(dir, "child.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state backgroundTestState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	canonicalDir, _ := filepath.EvalSymlinks(dir)
	if state.Directory != canonicalDir || !reflect.DeepEqual(state.Args, args) || !state.StdinEOF || state.Marker != "" {
		t.Fatalf("unexpected child state: %+v", state)
	}
	group, err := syscall.Getpgid(pid)
	if err != nil || group != pid {
		t.Fatalf("child is not in its own process group: group=%d, err=%v", group, err)
	}
	if err := syscall.Kill(pid, syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := syscall.Kill(pid, 0); err != nil {
		t.Fatalf("child did not survive hangup: %v", err)
	}
	output, err := os.ReadFile(filepath.Join(dir, "logs", "launcher.log"))
	if err != nil || !strings.Contains(string(output), "background stdout") || !strings.Contains(string(output), "background stderr") {
		t.Fatalf("missing redirected output: %s, err=%v", output, err)
	}
}

func TestDetachedReleaseEarlyExit(t *testing.T) {
	for _, mode := range []string{"exit", "fail"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv(backgroundTestModeEnv, mode)
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			pid, err := launchDetachedRelease(t.TempDir(), executable, []string{"-test.run=^TestReleaseBackgroundChild$"})
			if pid != 0 || (err != nil) != (mode == "fail") {
				t.Fatalf("unexpected exit: pid=%d, err=%v", pid, err)
			}
		})
	}
}

func TestReleaseNonDesktopModesStayAttached(t *testing.T) {
	t.Setenv(backgroundChildEnv, "")
	previous := buildMode
	t.Cleanup(func() { buildMode = previous })
	for _, tc := range []struct {
		mode      string
		container string
	}{
		{"development", ""},
		{"release", "1"},
	} {
		buildMode = tc.mode
		t.Setenv("JAVBOSS_CONTAINER", tc.container)
		background, err := startReleaseInBackground(t.TempDir())
		if background || err != nil {
			t.Fatalf("mode %s container %s: background=%v err=%v", tc.mode, tc.container, background, err)
		}
	}
}
