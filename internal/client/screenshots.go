package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"javboss/internal/mpv"
	"javboss/internal/util"
	"javboss/internal/video/screenshot"
)

func (c *Client) screenshotResolver(videoID int64, cookie string) mpv.ScreenshotResolver {
	remote, dataDir := c.remoteState(), c.clientDataDir()
	return func(ctx context.Context) ([]float64, error) {
		if remote == nil {
			return nil, fmt.Errorf("screenshot server unavailable")
		}
		// Read pending local files first. Uploads remove them only after the
		// server has saved them, so a concurrent upload cannot lose a marker.
		seconds, err := screenshot.ListPlaybackSeconds(dataDir, videoID)
		if err != nil {
			return nil, err
		}
		names, err := c.remoteScreenshotNames(ctx, remote, videoID, cookie)
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			if second, ok := screenshot.PlaybackSecond(name); ok {
				seconds = append(seconds, second)
			}
		}
		slices.Sort(seconds)
		return slices.Compact(seconds), nil
	}
}

func (c *Client) remoteScreenshotNames(ctx context.Context, remote *remoteState, videoID int64, cookie string) ([]string, error) {
	if remote == nil {
		return nil, fmt.Errorf("screenshot server unavailable")
	}
	target := *remote.base
	target.Path = "/videos/" + strconv.FormatInt(videoID, 10) + "/screenshots"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, err
	}
	// Keep both the server and credentials bound to the playback request.
	req.Header.Set("Cookie", cookie)
	res, err := c.transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch screenshot markers: HTTP %d", res.StatusCode)
	}
	var payload struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode screenshot markers: %w", err)
	}
	names := make([]string, 0, len(payload.Items))
	for _, item := range payload.Items {
		names = append(names, item.Name)
	}
	return names, nil
}

func (c *Client) screenshotDeleter(videoID int64, cookie string) mpv.ScreenshotDeleter {
	remote, dataDir := c.remoteState(), c.clientDataDir()
	return func(ctx context.Context, second float64) error {
		// Wait for any active upload, then keep pending files from being uploaded
		// until both remote and local copies have been deleted.
		// Mutex acquisition must remain cancellable if an upload stalls.
		ticker := time.NewTicker(25 * time.Millisecond)
		defer ticker.Stop()
		for !c.screenshotFilesMu.TryLock() {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
			}
		}
		defer c.screenshotFilesMu.Unlock()
		if err := ctx.Err(); err != nil {
			return err
		}
		names, err := screenshot.PlaybackNamesAt(dataDir, videoID, second)
		if err != nil {
			return err
		}
		remoteNames, err := c.remoteScreenshotNames(ctx, remote, videoID, cookie)
		if err != nil {
			return err
		}
		for _, name := range remoteNames {
			if value, ok := screenshot.PlaybackSecond(name); ok && value == second {
				names = append(names, name)
			}
		}
		slices.Sort(names)
		for _, name := range slices.Compact(names) {
			target := *remote.base
			target.Path = "/videos/" + strconv.FormatInt(videoID, 10) + "/screenshots/" + name
			req, err := http.NewRequestWithContext(ctx, http.MethodDelete, target.String(), nil)
			if err != nil {
				return err
			}
			req.Header.Set("Cookie", cookie)
			res, err := c.transport.RoundTrip(req)
			if err != nil {
				return err
			}
			_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
			res.Body.Close()
			if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNotFound {
				return fmt.Errorf("delete screenshot: HTTP %d", res.StatusCode)
			}
			path := filepath.Join(dataDir, "video", strconv.FormatInt(videoID, 10), "screenshot", name)
			if err := util.MoveFileToTrash(path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		return nil
	}
}
