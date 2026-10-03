package util

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

func backgroundTestCommand(t *testing.T, ctx context.Context, testName, mode string) *exec.Cmd {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := BackgroundCommandContext(ctx, executable, "-test.run=^"+testName+"$")
	cmd.Env = append(os.Environ(), "JAVBOSS_BACKGROUND_TEST_MODE="+mode)
	return cmd
}

func TestBackgroundCommandHelper(t *testing.T) {
	switch os.Getenv("JAVBOSS_BACKGROUND_TEST_MODE") {
	case "success":
		fmt.Fprint(os.Stdout, "output")
		fmt.Fprint(os.Stderr, "diagnostic")
		os.Exit(0)
	case "failure":
		fmt.Fprint(os.Stdout, "output")
		fmt.Fprint(os.Stderr, "diagnostic")
		os.Exit(7)
	case "wait":
		fmt.Fprintln(os.Stdout, "ready")
		for {
			time.Sleep(time.Second)
		}
	}
}

func TestBackgroundCommandOutputAndExitStatus(t *testing.T) {
	for _, mode := range []string{"success", "failure"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := backgroundTestCommand(t, ctx, "TestBackgroundCommandHelper", mode)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			stdout, err := cmd.Output()
			if mode == "success" && err != nil {
				t.Fatal(err)
			}
			if mode == "failure" {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) || exitErr.ExitCode() != 7 {
					t.Fatalf("expected exit code 7, got %v", err)
				}
			}
			if string(stdout) != "output" || stderr.String() != "diagnostic" {
				t.Fatalf("unexpected output: stdout=%q stderr=%q", stdout, stderr.String())
			}
		})
	}
}

func TestBackgroundCommandCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := backgroundTestCommand(t, ctx, "TestBackgroundCommandHelper", "wait")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(stdout)
	ready := scanner.Scan() && scanner.Text() == "ready"
	cancel()
	err = cmd.Wait()
	if !ready {
		t.Fatalf("child did not become ready: %v", scanner.Err())
	}
	if err == nil {
		t.Fatal("cancelled command succeeded")
	}
	if cmd.ProcessState == nil || !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatal("child did not exit after explicit cancellation")
	}
}
