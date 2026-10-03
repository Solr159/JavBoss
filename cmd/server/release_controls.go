package main

import (
	"context"
	"log"

	"javboss/internal/runtimeconfig"
	"javboss/internal/util"
)

func serveWithReleaseControls(ctx context.Context, stop context.CancelFunc, url, remoteURL string, logger *log.Logger, serve func() error) error {
	if buildMode != "release" || runtimeconfig.ContainerMode() {
		return serve()
	}
	return serveWithControls(stop, serve, func() {
		runReleaseControls(ctx, stop, url, remoteURL, logger)
	})
}

// Keep the UI on the calling thread and wait for HTTP shutdown before releasing
// the database, player, logs and single-instance lock in main.
func serveWithControls(stop context.CancelFunc, serve func() error, controls func()) error {
	done := make(chan error, 1)
	go func() {
		done <- serve()
		stop()
	}()
	controls()
	stop()
	return <-done
}

func openReleasePage(url string, logger *log.Logger) {
	if err := util.OpenFile(url); err != nil {
		logger.Printf("open browser failed: %v", err)
	}
}
