package mpv

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/video/screenshot"
)

// ScreenshotResolver returns timestamps for the video bound to a playlist entry.
// Refreshes do not overlap within a file load; switching files cancels the old
// request. Implementations must honor cancellation.
type ScreenshotResolver func(context.Context) ([]float64, error)

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
	if len(args) != 3 || !e.loaded || e.screenshotContext != nil {
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
	endpoint := e.endpoint
	return func() {
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
}
