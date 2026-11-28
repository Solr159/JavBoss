package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"

	"javboss/internal/mpv"
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
		target := *remote.base
		target.Path = "/videos/" + strconv.FormatInt(videoID, 10) + "/screenshots"
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
		if err != nil {
			return nil, err
		}
		// Keep the server and credentials bound to the original playback request.
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
		for _, item := range payload.Items {
			if second, ok := screenshot.PlaybackSecond(item.Name); ok {
				seconds = append(seconds, second)
			}
		}
		slices.Sort(seconds)
		return slices.Compact(seconds), nil
	}
}
