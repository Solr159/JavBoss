package util

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBackgroundCombinedOutputHelper(t *testing.T) {
	if len(os.Args) < 4 || os.Args[2] != "--" {
		return
	}
	switch os.Args[3] {
	case "output":
		fmt.Fprint(os.Stdout, "output")
		fmt.Fprint(os.Stderr, "diagnostic")
	case "failure":
		fmt.Fprint(os.Stderr, "diagnostic")
		os.Exit(7)
	case "args":
		json.NewEncoder(os.Stdout).Encode(os.Args[4:])
	case "large":
		fmt.Fprint(os.Stdout, strings.Repeat("x", 1024*1024))
	case "wait":
		if err := os.WriteFile(os.Args[4], []byte("ready"), 0600); err != nil {
			os.Exit(2)
		}
		for {
			time.Sleep(time.Second)
		}
	}
	os.Exit(0)
}

func TestBackgroundCombinedOutput(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		mode   string
		output string
		code   int
	}{
		{"output", "outputdiagnostic", 0},
		{"failure", "diagnostic", 7},
		{"large", strings.Repeat("x", 1024*1024), 0},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			output, err := BackgroundCombinedOutput(ctx, executable, "-test.run=^TestBackgroundCombinedOutputHelper$", "--", tt.mode)
			if tt.code == 0 && err != nil {
				t.Fatal(err)
			}
			if tt.code != 0 {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) || exitErr.ExitCode() != tt.code {
					t.Fatalf("expected exit code %d, got %v", tt.code, err)
				}
			}
			if string(output) != tt.output {
				t.Fatalf("unexpected output (%d bytes)", len(output))
			}
		})
	}
}

func TestBackgroundCombinedOutputArguments(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// Exercise an executable path with spaces and Unicode as well as arguments.
	copyPath := filepath.Join(t.TempDir(), "后台 tool.exe")
	source, err := os.Open(executable)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	dest, err := os.OpenFile(copyPath, os.O_CREATE|os.O_WRONLY, 0700)
	if err != nil {
		t.Fatal(err)
	}
	_, copyErr := io.Copy(dest, source)
	closeErr := dest.Close()
	if copyErr != nil || closeErr != nil {
		t.Fatalf("copy helper: %v, %v", copyErr, closeErr)
	}
	want := []string{"", "影片 路径", `C:\videos with spaces\`, `a"b`, `a\"b`, "& | %PATH%"}
	args := append([]string{"-test.run=^TestBackgroundCombinedOutputHelper$", "--", "args"}, want...)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output, err := BackgroundCombinedOutput(ctx, copyPath, args...)
	if err != nil {
		t.Fatalf("run helper: %v: %s", err, output)
	}
	var got []string
	if err := json.Unmarshal(output, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("arguments: got %q, want %q", got, want)
	}
}

func TestBackgroundCombinedOutputCancellation(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	readyPath := filepath.Join(t.TempDir(), "ready")
	done := make(chan error, 1)
	go func() {
		_, err := BackgroundCombinedOutput(ctx, executable, "-test.run=^TestBackgroundCombinedOutputHelper$", "--", "wait", readyPath)
		done <- err
	}()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			t.Fatalf("child exited before cancellation: %v", err)
		case <-ctx.Done():
			t.Fatal("child did not become ready")
		case <-ticker.C:
			if _, err := os.Stat(readyPath); err == nil {
				cancel()
				select {
				case err := <-done:
					if err == nil {
						t.Fatal("cancelled command succeeded")
					}
				case <-time.After(5 * time.Second):
					t.Fatal("cancelled child did not exit")
				}
				return
			}
		}
	}
}

func TestBackgroundCombinedOutputStartErrors(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := BackgroundCombinedOutput(ctx, executable); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-cancelled context: %v", err)
	}
	if _, err := BackgroundCombinedOutput(context.Background(), "javboss-missing-background-tool"); !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("missing executable: %v", err)
	}
	if _, err := BackgroundCombinedOutput(context.Background(), executable, "invalid\x00argument"); err == nil {
		t.Fatal("accepted an argument containing NUL")
	}
}
