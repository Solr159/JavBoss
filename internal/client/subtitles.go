package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"javboss/internal/mpv"
	"javboss/internal/subtitle"
)

func (c *Client) subtitleResolver(videoID, locationID int64, cookie string, paths ...string) mpv.SubtitleResolver {
	remote, dataDir := c.remoteState(), c.clientDataDir()
	return func(ctx context.Context, request mpv.SubtitleRequest) (*mpv.SubtitleFile, error) {
		if remote == nil {
			return nil, fmt.Errorf("subtitle server unavailable")
		}
		target := *remote.base
		target.Path = "/videos/" + strconv.FormatInt(videoID, 10) + "/subtitles/resolve"
		payload := map[string]any{"location_id": locationID, "format": "srt", "settings_only": request.SettingsOnly, "manual": request.Manual, "download_only": request.DownloadOnly, "cache_only": request.CacheOnly}
		if locationID == 0 && len(paths) == 2 {
			payload["path"], payload["dir_path"] = paths[0], paths[1]
		}
		body, _ := json.Marshal(payload)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		// Pin both the server and its credentials to the playback request.
		req.Header.Set("Cookie", cookie)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.transport.RoundTrip(req)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
		track, err := url.PathUnescape(res.Header.Get("X-Subtitle-Track"))
		if err != nil {
			return nil, err
		}
		key, err := url.PathUnescape(res.Header.Get("X-Subtitle-Key"))
		if err != nil {
			return nil, err
		}
		downloaded := res.Header.Get("X-Subtitle-Downloaded") == "true"
		offsetMS := int64(0)
		if raw := res.Header.Get("X-Subtitle-Offset-Ms"); raw != "" {
			offsetMS, err = strconv.ParseInt(raw, 10, 64)
			if err != nil || offsetMS < -3600000 || offsetMS > 3600000 {
				return nil, fmt.Errorf("invalid subtitle offset")
			}
		}
		if res.StatusCode == http.StatusNoContent {
			return &mpv.SubtitleFile{OffsetSec: float64(offsetMS) / 1000, Status: res.Header.Get("X-Subtitle-Status"), Downloaded: downloaded, Track: track, Key: key}, nil
		}
		if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("fetch subtitle: HTTP %d", res.StatusCode)
		}
		data, err := io.ReadAll(io.LimitReader(res.Body, subtitle.MaxBytes+1))
		if err == nil {
			data, err = subtitle.NormalizeSRT(data)
		}
		if err != nil {
			return nil, err
		}
		_, parameters, err := mime.ParseMediaType(res.Header.Get("Content-Disposition"))
		name := parameters["filename"]
		if err != nil || name == "" || filepath.Base(name) != name || strings.ContainsAny(name, `/\:*?"<>|`) || !strings.EqualFold(filepath.Ext(name), ".srt") {
			return nil, fmt.Errorf("invalid subtitle filename")
		}
		path := filepath.Join(dataDir, "subtitles", name)
		// A server-side sidecar can share its filename with a downloaded track.
		// Keep the fetched local copy distinct so both remain selectable in MPV.
		if res.Header.Get("X-Subtitle-Source") == "local" {
			path = filepath.Join(dataDir, "subtitles", "local", strconv.FormatInt(videoID, 10), name)
		}
		if err := subtitle.WriteAtomic(path, data); err != nil {
			return nil, err
		}
		return &mpv.SubtitleFile{Path: path, Language: res.Header.Get("X-Subtitle-Language"), Title: res.Header.Get("X-Subtitle-Source"), OffsetSec: float64(offsetMS) / 1000, Status: res.Header.Get("X-Subtitle-Status"), Downloaded: downloaded, Track: track, Key: key}, nil
	}
}

func (c *Client) subtitleOffsetSaver(videoID int64, cookie string) mpv.SubtitleOffsetSaver {
	if videoID <= 0 {
		return nil
	}
	remote := c.remoteState()
	return func(ctx context.Context, offsetMS int64) error {
		if remote == nil {
			return fmt.Errorf("subtitle server unavailable")
		}
		if offsetMS < -3600000 || offsetMS > 3600000 {
			return fmt.Errorf("invalid subtitle offset")
		}
		target := *remote.base
		target.Path = "/videos/" + strconv.FormatInt(videoID, 10) + "/subtitle-settings"
		body, _ := json.Marshal(map[string]int64{"offset_ms": offsetMS})
		req, err := http.NewRequestWithContext(ctx, http.MethodPatch, target.String(), bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Cookie", cookie)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.transport.RoundTrip(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("save subtitle offset: HTTP %d", res.StatusCode)
		}
		return nil
	}
}

func (c *Client) subtitleSelectionSaver(videoID int64, cookie string) mpv.SubtitleSelectionSaver {
	if videoID <= 0 {
		return nil
	}
	remote := c.remoteState()
	return func(ctx context.Context, track string) error {
		if remote == nil {
			return fmt.Errorf("subtitle server unavailable")
		}
		target := *remote.base
		target.Path = "/videos/" + strconv.FormatInt(videoID, 10) + "/subtitle-settings"
		body, _ := json.Marshal(map[string]string{"track": track})
		req, err := http.NewRequestWithContext(ctx, http.MethodPatch, target.String(), bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Cookie", cookie)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.transport.RoundTrip(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("save subtitle visibility: HTTP %d", res.StatusCode)
		}
		return nil
	}
}
