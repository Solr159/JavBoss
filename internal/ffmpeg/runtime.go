package ffmpeg

import (
	"context"
	"errors"
	"sync/atomic"
)

var active atomic.Pointer[installer]

var ErrUnavailable = errors.New("ffmpeg installer is unavailable")

// Init configures installation paths and download lifetime. Call once before consumers.
func Init(ctx context.Context, baseDir string) { active.Store(newInstaller(ctx, baseDir)) }

// GetStatus returns a snapshot of installation and download progress.
func GetStatus() (Status, error) {
	i := active.Load()
	if i == nil {
		return Status{}, ErrUnavailable
	}
	return i.Status(), nil
}

// StartDownload starts an asynchronous download unless already installed or running.
func StartDownload() (bool, error) {
	i := active.Load()
	if i == nil {
		return false, ErrUnavailable
	}
	return i.StartDownload()
}
