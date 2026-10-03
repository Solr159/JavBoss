package util

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestBackgroundCommandConsoleHelper(t *testing.T) {
	if os.Getenv("JAVBOSS_BACKGROUND_TEST_MODE") != "console" {
		return
	}
	window, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetConsoleWindow").Call()
	if window != 0 {
		fmt.Fprint(os.Stderr, "background process has a console window")
		os.Exit(1)
	}
	fmt.Fprint(os.Stdout, "no console")
	os.Exit(0)
}

func TestBackgroundCommandHasNoConsole(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := backgroundTestCommand(t, ctx, "TestBackgroundCommandConsoleHelper", "console")
	output, err := cmd.CombinedOutput()
	if err != nil || string(output) != "no console" {
		t.Fatalf("background console check: err=%v output=%q", err, output)
	}
}

func TestBackgroundCombinedOutputStartupHelper(t *testing.T) {
	if len(os.Args) != 4 || os.Args[2] != "--" || os.Args[3] != "--background-startup-helper" {
		return
	}
	var startup windows.StartupInfo
	if err := windows.GetStartupInfo(&startup); err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
	if startup.Flags&startfForceOffFeedback == 0 {
		fmt.Fprint(os.Stderr, "startup feedback is not disabled")
		os.Exit(1)
	}
	window, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetConsoleWindow").Call()
	if window != 0 {
		fmt.Fprint(os.Stderr, "background process has a console window")
		os.Exit(1)
	}
	fmt.Fprint(os.Stdout, "no feedback or console")
	os.Exit(0)
}

func TestBackgroundCombinedOutputDisablesStartupFeedback(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Keep the helper marker out of the testing package's flag parser.
	output, err := BackgroundCombinedOutput(ctx, executable, "-test.run=^TestBackgroundCombinedOutputStartupHelper$", "--", "--background-startup-helper")
	if err != nil || string(output) != "no feedback or console" {
		t.Fatalf("startup feedback check: err=%v output=%q", err, output)
	}
}
