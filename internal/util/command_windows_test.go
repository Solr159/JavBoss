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
