package main

/*
#cgo darwin LDFLAGS: -framework Cocoa
void javbossPrepareMenuBarApplication(void);
*/
import "C"

import (
	"context"
	"fmt"
	"log"
	"os"
)

func runReleaseControls(ctx context.Context, stop context.CancelFunc, url, remoteURL string, logger *log.Logger) {
	// systray locks the initial OS thread in init. Keep AppKit and its event
	// loop on that thread, including when launched without an application bundle.
	C.javbossPrepareMenuBarApplication()
	runReleaseTray(ctx, stop, url, remoteURL, logger)
}

func notifyAlreadyRunning(message string) {
	// The background launcher redirects stderr to its log and stdin to /dev/null.
	// Exit immediately instead of waiting for input that can never arrive.
	fmt.Fprintln(os.Stderr, message)
}
