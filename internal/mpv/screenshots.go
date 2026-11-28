package mpv

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"strconv"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/video/screenshot"
)

// ScreenshotResolver returns timestamps for the video bound to a playlist entry.
// Refreshes do not overlap within a file load; switching files cancels the old
// request. Implementations must honor cancellation.
type ScreenshotResolver func(context.Context) ([]float64, error)

// ScreenshotDeleter deletes the images represented by one timestamp.
type ScreenshotDeleter func(context.Context, float64) error

type screenshotSnapshot struct {
	generation string
	seconds    []float64
}

func (e *playlistEvents) setScreenshotDeleters(deleters map[int64]ScreenshotDeleter) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.screenshotDeleters = deleters
}

func playbackScreenshotResolver(options PlayOptions) ScreenshotResolver {
	if options.ResolveScreenshots != nil {
		return options.ResolveScreenshots
	}
	if options.DataDir == "" || options.VideoID <= 0 {
		return nil
	}
	return func(ctx context.Context) ([]float64, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return screenshot.ListPlaybackSeconds(options.DataDir, options.VideoID)
	}
}

func (e *playlistEvents) setScreenshots(resolvers map[int64]ScreenshotResolver) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.screenshots = resolvers
}

// Called under e.mu. ModernZ polls only while a file is loaded. The Lua
// generation guard makes validating the reply and updating the OSC atomic.
func (e *playlistEvents) screenshotCallback(args []string) func() {
	if len(args) != 3 || !e.loaded || e.screenshotContext != nil || e.screenshotDeleteContext != nil {
		return nil
	}
	generation, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil || generation < 1 {
		return nil
	}
	entry, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil || entry != e.entryID {
		return nil
	}
	resolver := e.screenshots[entry]
	if resolver == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	e.screenshotContext, e.screenshotCancel = ctx, cancel
	done := make(chan struct{})
	e.screenshotDone = done
	endpoint := e.endpoint
	return func() {
		defer close(done)
		defer func() {
			cancel()
			e.mu.Lock()
			if e.screenshotContext == ctx {
				e.screenshotContext, e.screenshotCancel = nil, nil
			}
			e.mu.Unlock()
		}()
		seconds, err := resolver(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			logging.Error("read playback screenshot markers: %v", err)
			return
		}
		if seconds == nil {
			seconds = []float64{}
		}
		payload, err := json.Marshal(seconds)
		if err != nil {
			return
		}
		e.mu.Lock()
		if e.screenshotContext != ctx || ctx.Err() != nil {
			e.mu.Unlock()
			return
		}
		e.screenshotSnapshot = &screenshotSnapshot{generation: args[1], seconds: slices.Clone(seconds)}
		e.mu.Unlock()
		if err := runIPCCommand(endpoint, []any{"script-message-to", "modernz", "javboss-screenshot-markers", args[1], args[2], string(payload)}); err != nil {
			logging.Error("update playback screenshot markers: %v", err)
		}
	}
}

func (e *playlistEvents) cancelScreenshots() {
	if e.screenshotCancel != nil {
		e.screenshotCancel()
	}
	e.screenshotContext, e.screenshotCancel = nil, nil
	e.screenshotSnapshot, e.screenshotDone = nil, nil
	if e.screenshotDeleteCancel != nil {
		e.screenshotDeleteCancel()
	}
	e.screenshotDeleteContext, e.screenshotDeleteCancel = nil, nil
}

// Called under e.mu. Only timestamps from the current displayed snapshot can
// be deleted. Capture the entry's deleter before doing any asynchronous work.
func (e *playlistEvents) screenshotDeleteCallback(args []string) func() {
	if len(args) != 4 || !e.loaded || e.screenshotDeleteContext != nil {
		return nil
	}
	entry, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil || entry != e.entryID {
		return nil
	}
	second, err := strconv.ParseFloat(args[3], 64)
	if err != nil || second < 0 || math.IsNaN(second) || math.IsInf(second, 0) {
		return nil
	}
	if e.screenshotSnapshot == nil || args[1] != e.screenshotSnapshot.generation || !slices.Contains(e.screenshotSnapshot.seconds, second) {
		// A refresh can remove a marker between the user's click and this
		// request. Acknowledge rejection so the UI does not stay pending.
		endpoint := e.endpoint
		return func() {
			_ = runIPCCommand(endpoint, []any{"script-message-to", "modernz", "javboss-screenshot-deleted", args[1], args[2], args[3], "error"})
		}
	}
	deleteScreenshot := e.screenshotDeleters[entry]
	previous := e.screenshotDone
	if e.screenshotCancel != nil {
		e.screenshotCancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	e.screenshotDeleteContext, e.screenshotDeleteCancel = ctx, cancel
	endpoint := e.endpoint
	return func() {
		status := "error"
		defer func() {
			canceled := errors.Is(ctx.Err(), context.Canceled)
			cancel()
			e.mu.Lock()
			if e.screenshotDeleteContext == ctx {
				e.screenshotDeleteContext, e.screenshotDeleteCancel = nil, nil
			}
			e.mu.Unlock()
			if !canceled {
				_ = runIPCCommand(endpoint, []any{"script-message-to", "modernz", "javboss-screenshot-deleted", args[1], args[2], args[3], status})
			}
		}()
		// Drain the old refresh before deleting so it cannot reintroduce the
		// removed timestamp after the acknowledgement reaches ModernZ.
		if previous != nil {
			select {
			case <-previous:
			case <-ctx.Done():
				return
			}
		}
		if ctx.Err() != nil || deleteScreenshot == nil {
			return
		}
		if err := deleteScreenshot(ctx, second); err != nil {
			logging.Error("delete playback screenshot: %v", err)
			return
		}
		status = "deleted"
	}
}
