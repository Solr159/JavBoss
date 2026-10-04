package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"javboss/internal/playback"
)

func (c *Client) playbackReporter(videoID, locationID int64, cookie string, paths ...string) func() *playback.Reporter {
	remote := c.remoteState()
	return func() *playback.Reporter {
		request := func(ctx context.Context, method, suffix string, body any) (*http.Response, error) {
			// Pin the destination to the server that supplied this video.
			if remote == nil {
				return nil, fmt.Errorf("playback server unavailable")
			}
			target := *remote.base
			target.Path = "/videos/" + strconv.FormatInt(videoID, 10) + "/playback-sessions" + suffix
			data, err := json.Marshal(body)
			if err != nil {
				return nil, err
			}
			req, err := http.NewRequestWithContext(ctx, method, target.String(), bytes.NewReader(data))
			if err != nil {
				return nil, err
			}
			credentials := cookie
			if c.remoteState() == remote {
				if fresh := c.latestScreenshotCookie(); fresh != "" {
					credentials = fresh
				}
			}
			req.Header.Set("Cookie", credentials)
			req.Header.Set("Content-Type", "application/json")
			return c.transport.RoundTrip(req)
		}
		return playback.NewReporter(func(ctx context.Context) (string, error) {
			body := map[string]any{"location_id": locationID}
			if locationID == 0 && len(paths) == 2 {
				body["path"], body["dir_path"] = paths[0], paths[1]
			}
			res, err := request(ctx, http.MethodPost, "", body)
			if err != nil {
				return "", err
			}
			defer res.Body.Close()
			if res.StatusCode != http.StatusCreated {
				return "", fmt.Errorf("create playback session: HTTP %d", res.StatusCode)
			}
			var data struct {
				SessionID string `json:"session_id"`
			}
			if err := json.NewDecoder(io.LimitReader(res.Body, 4096)).Decode(&data); err != nil {
				return "", err
			}
			if data.SessionID == "" {
				return "", fmt.Errorf("empty playback session")
			}
			return data.SessionID, nil
		}, func(ctx context.Context, id string, total int64) error {
			res, err := request(ctx, http.MethodPut, "/"+id, map[string]int64{"watched_ms": total})
			if err != nil {
				return err
			}
			defer res.Body.Close()
			_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
			if res.StatusCode == http.StatusGone {
				return playback.ErrExpired
			}
			if res.StatusCode != http.StatusNoContent {
				return fmt.Errorf("report playback session: HTTP %d", res.StatusCode)
			}
			return nil
		})
	}
}
