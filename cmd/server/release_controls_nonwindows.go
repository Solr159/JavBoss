//go:build !windows

package main

import (
	"context"
	"fmt"
	"log"
)

func runReleaseControls(ctx context.Context, stop context.CancelFunc, url, remoteURL string, logger *log.Logger) {
	if remoteURL == "" {
		printReleaseStartupHint(url)
	} else {
		printReleaseClientStartupHint(url, remoteURL)
	}
	openReleasePage(url, logger)
	startReleaseKeyboardControls(ctx, stop, url, logger)
	<-ctx.Done()
}

func notifyAlreadyRunning(message string) {
	fmt.Println(message)
	waitForUserExit()
}
