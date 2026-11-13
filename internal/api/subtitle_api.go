package api

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"javboss/internal/common"
	"javboss/internal/common/logging"
	dbpkg "javboss/internal/db"
	"javboss/internal/models"
	"javboss/internal/mpv"
	"javboss/internal/subtitle"
	"javboss/internal/subtitle/subtitlecat"
	"javboss/internal/util"
)

// Source registration belongs at the application boundary. Neither the service
// nor any player knows which concrete providers are installed.
var subtitleService = subtitle.NewService(subtitlecat.New(util.DefaultHTTPClient()))

// POST /videos/:id/subtitles/resolve accepts {location_id, format: "srt"|"vtt"}.
// location_id=0 selects the primary location. 200 returns a subtitle file;
// 204 carries X-Subtitle-Status (disabled/not_found/existing/ambiguous).
// Legacy clients may use {path, dir_path} instead of location_id.
// settings_only=true skips subtitle lookup and returns 204 with X-Subtitle-Offset-Ms.
// Existing local/cached subtitles are always eligible for loading.
// manual=true skips local sidecars and permits downloading even when automatic downloading is disabled.
// download_only=true skips sidecars; cache_only=true forbids provider lookups.
// SRT stays unchanged; VTT includes the video-specific subtitle offset.
// Requests are authenticated by the same middleware as video streaming.
func resolveSubtitle(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	var req struct {
		LocationID   int64  `json:"location_id"`
		Format       string `json:"format"`
		Path         string `json:"path"`
		DirPath      string `json:"dir_path"`
		SettingsOnly bool   `json:"settings_only"`
		Manual       bool   `json:"manual"`
		DownloadOnly bool   `json:"download_only"`
		CacheOnly    bool   `json:"cache_only"`
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 || c.ShouldBindJSON(&req) != nil || req.LocationID < 0 || (req.Format != "" && req.Format != "srt" && req.Format != "vtt") {
		respondLocalizedError(c, http.StatusBadRequest, "字幕请求无效", "Invalid subtitle request")
		return
	}
	fullPath := ""
	if req.LocationID == 0 && (req.Path != "" || req.DirPath != "") {
		fullPath, _, err = resolveVideoPath(req.Path, req.DirPath)
		if err != nil {
			respondLocalizedError(c, 400, "视频路径无效", "Invalid video path")
			return
		}
	}
	video, err := subtitleVideo(c.Request.Context(), id, req.LocationID, fullPath)
	if err != nil {
		respondLocalizedError(c, 500, "加载视频失败", "Failed to load video")
		return
	}
	if video == nil {
		respondLocalizedError(c, 404, "视频位置不存在", "Video location not found")
		return
	}
	c.Header("X-Subtitle-Offset-Ms", strconv.FormatInt(video.SubtitleSettings.OffsetMS, 10))
	c.Header("X-Subtitle-Track", url.PathEscape(video.SubtitleSettings.Track))
	c.Header("Cache-Control", "no-store")
	if req.SettingsOnly {
		cfg, err := dbpkg.Default().ListConfig(c.Request.Context())
		if err != nil {
			respondLocalizedError(c, 500, "读取字幕设置失败", "Could not read subtitle settings")
			return
		}
		if cfg["subtitle_auto_download"] != "true" {
			c.Header("X-Subtitle-Status", "disabled")
		}
		c.Status(http.StatusNoContent)
		return
	}
	// The shared provider task can take 45 seconds. Extend only this response's
	// deadline beyond the server's ordinary 30-second write timeout.
	if err := http.NewResponseController(c.Writer).SetWriteDeadline(time.Now().Add(55 * time.Second)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		logging.Error("set subtitle response deadline: %v", err)
	}
	result, status, err := resolveVideoSubtitle(c.Request.Context(), video, mpv.SubtitleRequest{Manual: req.Manual, DownloadOnly: req.DownloadOnly, CacheOnly: req.CacheOnly})
	if c.Request.Context().Err() != nil {
		return
	}
	if err != nil {
		logging.Error("resolve subtitle for video %d: %v", id, err)
		respondLocalizedError(c, 502, "获取字幕失败，可稍后重试", "Could not fetch subtitles; try again later")
		return
	}
	c.Header("X-Subtitle-Status", status)
	if result == nil {
		c.Status(http.StatusNoContent)
		return
	}
	data, err := os.ReadFile(result.Path)
	if err == nil && req.Format == "vtt" {
		data, err = subtitle.WebVTTWithOffset(data, video.SubtitleSettings.OffsetMS)
	}
	if err != nil {
		respondLocalizedError(c, 500, "读取字幕失败", "Could not read subtitles")
		return
	}
	c.Header("X-Subtitle-Key", url.PathEscape(result.SelectionKey()))
	c.Header("X-Subtitle-Language", result.Language)
	c.Header("X-Subtitle-Downloaded", strconv.FormatBool(result.Downloaded))
	c.Header("X-Subtitle-Source", result.Source)
	c.Header("X-Content-Type-Options", "nosniff")
	contentType := "application/x-subrip; charset=utf-8"
	if req.Format == "vtt" {
		contentType = "text/vtt; charset=utf-8"
	}
	filename := filepath.Base(result.Path)
	if req.Format == "vtt" {
		filename = strings.TrimSuffix(filename, filepath.Ext(filename)) + ".vtt"
	}
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	c.Data(http.StatusOK, contentType, data)
}

func subtitleVideo(ctx context.Context, id, locationID int64, fullPath string) (*models.Video, error) {
	if locationID > 0 {
		return dbpkg.Default().GetVideoForLocation(ctx, id, locationID)
	}
	if fullPath == "" {
		loc, err := dbpkg.Default().GetPrimaryVideoLocation(ctx, id)
		if err != nil || loc == nil {
			return nil, err
		}
		return dbpkg.Default().GetVideoForLocation(ctx, id, loc.ID)
	}
	video, err := dbpkg.Default().GetVideo(ctx, id)
	if err != nil || video == nil {
		return video, err
	}
	// Legacy playback requests specify a path. Resolve the actual location rather
	// than attributing another location's JAV metadata to it.
	for _, loc := range video.Locations {
		if filepath.Clean(filepath.Join(loc.DirectoryRef.Path, loc.RelativePath)) == filepath.Clean(fullPath) {
			return dbpkg.Default().GetVideoForLocation(ctx, id, loc.ID)
		}
	}
	return nil, nil
}

func resolveVideoSubtitle(ctx context.Context, video *models.Video, request mpv.SubtitleRequest) (*subtitle.Result, string, error) {
	cfg, err := dbpkg.Default().ListConfig(ctx)
	if err != nil {
		return nil, "", err
	}
	allowDownload := !request.CacheOnly && (request.Manual || cfg["subtitle_auto_download"] == "true")
	languages := []string{"zh-CN", "zh-TW"}
	if cfg["subtitle_language"] == "zh-TW" {
		languages = []string{"zh-TW", "zh-CN"}
	}
	fullPath, _, err := resolveVideoPath(video.Path, video.DirectoryRef.Path)
	if err != nil {
		return nil, "", err
	}
	if !request.Manual && !request.DownloadOnly {
		if local, exists := localSubtitle(fullPath, languages); exists {
			return local, "existing", nil
		}
	}
	code := ""
	if video.Jav != nil {
		code = strings.TrimSpace(video.Jav.Code)
	}
	if code == "" {
		return nil, "ambiguous", nil
	}
	// Do not silently apply a whole-film subtitle to explicitly split files.
	if subtitle.IsSplit(code, video.Filename) {
		return nil, "ambiguous", nil
	}
	appConfig := common.CurrentConfig()
	if appConfig == nil {
		return nil, "", fmt.Errorf("subtitle cache configuration unavailable")
	}
	req := subtitle.Request{
		Query:     subtitle.Query{Code: code},
		CacheDir:  filepath.Join(filepath.Dir(appConfig.DatabasePath), "subtitles"),
		Languages: languages,
	}
	cached, err := subtitleService.Cached(ctx, req)
	// A fallback cache must not bypass the preferred language when downloads
	// are allowed. Resolve checks each language's cache before its providers.
	if err == nil && (!allowDownload || cached.Language == languages[0]) {
		return cached, "ready", nil
	}
	if err != nil && !errors.Is(err, subtitle.ErrNotFound) {
		return nil, "", err
	}
	if !allowDownload {
		return nil, "disabled", nil
	}
	result, err := subtitleService.Resolve(ctx, req)
	// Keep an existing subtitle usable even if the preferred lookup times out.
	// A cancelled playback request must still stop instead of returning a track.
	if err != nil && cached != nil && ctx.Err() == nil {
		return cached, "ready", nil
	}
	if errors.Is(err, subtitle.ErrNotFound) {
		return nil, "not_found", nil
	}
	return result, "ready", err
}

func localSubtitle(videoPath string, languages []string) (*subtitle.Result, bool) {
	base := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))
	entries, err := os.ReadDir(filepath.Dir(videoPath))
	if err != nil {
		return nil, false
	}
	// Prefer explicitly named Chinese tracks, then the unqualified sidecar.
	suffixes := []string{"." + languages[0], "." + languages[1], ".zh", ""}
	for _, suffix := range suffixes {
		for _, ext := range []string{".srt", ".ass", ".ssa", ".sub", ".vtt"} {
			path := ""
			for _, entry := range entries {
				if strings.EqualFold(entry.Name(), filepath.Base(base+suffix+ext)) {
					path = filepath.Join(filepath.Dir(videoPath), entry.Name())
					break
				}
			}
			if path == "" {
				continue
			}
			info, err := os.Stat(path)
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			if ext != ".srt" || info.Size() > subtitle.MaxBytes {
				return nil, true
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, true
			}
			if _, err = subtitle.NormalizeSRT(data); err != nil {
				return nil, true
			}
			lang := languages[0]
			if suffix == "."+languages[1] {
				lang = languages[1]
			}
			return &subtitle.Result{Path: path, Language: lang, Source: subtitle.SourceLocal}, true
		}
	}
	return nil, false
}

func localSubtitleResolver(videoID, locationID int64, fullPath string) mpv.SubtitleResolver {
	if videoID <= 0 {
		return nil
	}
	return func(ctx context.Context, request mpv.SubtitleRequest) (*mpv.SubtitleFile, error) {
		video, err := subtitleVideo(ctx, videoID, locationID, fullPath)
		if err != nil || video == nil {
			return nil, err
		}
		file := &mpv.SubtitleFile{OffsetSec: float64(video.SubtitleSettings.OffsetMS) / 1000, Track: video.SubtitleSettings.Track}
		if request.SettingsOnly {
			cfg, err := dbpkg.Default().ListConfig(ctx)
			if err != nil {
				return nil, err
			}
			if cfg["subtitle_auto_download"] != "true" {
				file.Status = "disabled"
			}
			return file, nil
		}
		result, status, err := resolveVideoSubtitle(ctx, video, request)
		if err != nil {
			return nil, err
		}
		if result != nil {
			file.Path, file.Language, file.Title = result.Path, result.Language, result.Source
			file.Downloaded = result.Downloaded
			file.Key = result.SelectionKey()
		}
		file.Status = status
		return file, nil
	}
}
