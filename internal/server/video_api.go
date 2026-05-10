package server

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"pornboss/internal/common"
	"pornboss/internal/common/logging"
	dbpkg "pornboss/internal/db"
	"pornboss/internal/manager"
	"pornboss/internal/models"
	"pornboss/internal/mpv"
	"pornboss/internal/util"
)

func listVideos(c *gin.Context) {
	limit := queryInt(c, "limit", 100)
	offset := queryInt(c, "offset", 0)
	tagFilter := parseTagQuery(c.Query("tags"))
	directoryIDs := parseDirectoryIDs(c.Query("directory_ids"))
	search := strings.TrimSpace(c.Query("search"))
	sort := strings.TrimSpace(c.Query("sort"))
	hideJav := queryBool(c, "hide_jav", true)
	seedParam := strings.TrimSpace(c.Query("seed"))
	var seed *int64
	if seedParam != "" {
		parsed, err := strconv.ParseInt(seedParam, 10, 64)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid seed"})
			return
		}
		seed = &parsed
	}

	videos, err := dbpkg.ListVideos(c.Request.Context(), limit, offset, tagFilter, search, sort, seed, directoryIDs, hideJav)
	if err != nil {
		logging.Error("list videos error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	total, err := dbpkg.CountVideos(c.Request.Context(), tagFilter, search, directoryIDs, hideJav)
	if err != nil {
		logging.Error("count videos error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": videos,
		"total": total,
	})
}

func getVideo(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	video, err := dbpkg.GetVideo(c.Request.Context(), id)
	if err != nil {
		logging.Error("get video error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if video == nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.JSON(http.StatusOK, video)
}

func incrementVideoPlayCount(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := dbpkg.IncrementVideoPlayCount(c.Request.Context(), id); err != nil {
		logging.Error("increment play count error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

type playbackSource struct {
	Kind     string `json:"kind"`
	Src      string `json:"src"`
	MimeType string `json:"mime_type"`
	Label    string `json:"label"`
}

type playbackInfo struct {
	VideoID       int64            `json:"video_id"`
	PreferredKind string           `json:"preferred_kind"`
	Sources       []playbackSource `json:"sources"`
}

type videoScreenshotInfo struct {
	Name       string    `json:"name"`
	URL        string    `json:"url"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modified_at"`
}

func getVideoStreams(c *gin.Context) {
	video, fullPath, err := resolveVideoStreamTarget(c)
	if err != nil {
		respondPlaybackError(c, err)
		return
	}

	probe, err := util.ProbePlaybackSupport(fullPath)
	if err != nil {
		logging.Error("probe playback support error: %v", err)
		respondPlaybackError(c, err)
		return
	}

	info := playbackInfo{
		VideoID: video.ID,
	}
	if !probe.SupportsDirect {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": "browser playback is not supported for this file format",
		})
		return
	}
	info.PreferredKind = "direct"
	info.Sources = []playbackSource{{
		Kind:     "direct",
		Src:      buildDirectStreamURL(video),
		MimeType: directMimeType(probe.Container),
		Label:    "Direct",
	}}

	c.JSON(http.StatusOK, info)
}

func streamVideo(c *gin.Context) {
	fullPath, err := resolveStreamPathFromQuery(c)
	if err != nil {
		_, fullPath, err = resolveVideoStreamTarget(c)
		if err != nil {
			respondPlaybackError(c, err)
			return
		}
	}
	serveVideoFile(c, fullPath)
}

func resolveStreamPathFromQuery(c *gin.Context) (string, error) {
	rawPath := strings.TrimSpace(c.Query("path"))
	rawDirPath := strings.TrimSpace(c.Query("dir_path"))
	fullPath, _, err := resolveVideoPath(rawPath, rawDirPath)
	return fullPath, err
}

func resolveVideoStreamTarget(c *gin.Context) (*models.Video, string, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		return nil, "", errors.New("invalid id")
	}

	video, err := dbpkg.GetVideo(c.Request.Context(), id)
	if err != nil {
		return nil, "", err
	}
	if video == nil {
		return nil, "", os.ErrNotExist
	}

	fullPath, err := resolveVideoPrimaryPath(c.Request.Context(), video)
	if err != nil {
		return nil, "", err
	}
	if _, err := os.Stat(fullPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", err
		}
		return nil, "", err
	}

	return video, fullPath, nil
}

func respondPlaybackError(c *gin.Context, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, os.ErrNotExist):
		c.Status(http.StatusNotFound)
	case errors.Is(err, context.Canceled):
		c.Status(499)
	case strings.Contains(err.Error(), "ffprobe not found"):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
	case strings.Contains(err.Error(), "browser playback is not supported"):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	case strings.Contains(err.Error(), "invalid id"), strings.Contains(err.Error(), "invalid path"):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}

func directMimeType(container string) string {
	switch strings.ToLower(strings.TrimSpace(container)) {
	case "webm":
		return "video/webm"
	default:
		return "video/mp4"
	}
}

func buildDirectStreamURL(video *models.Video) string {
	if video == nil {
		return ""
	}
	return "/videos/" + strconv.FormatInt(video.ID, 10) + "/stream"
}

func resolveVideoPrimaryPath(ctx context.Context, video *models.Video) (string, error) {
	if video == nil {
		return "", errors.New("video is nil")
	}
	loc, err := dbpkg.GetPrimaryVideoLocation(ctx, video.ID)
	if err != nil {
		return "", err
	}
	if loc != nil {
		fullPath, _, err := resolveVideoPath(loc.RelativePath, loc.DirectoryRef.Path)
		return fullPath, err
	}
	return "", errors.New("video location missing")
}

func serveVideoFile(c *gin.Context, fullPath string) {
	if _, err := os.Stat(fullPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.Status(http.StatusNotFound)
			return
		}
		logging.Error("stat stream file error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.File(fullPath)
}

func openVideoFile(c *gin.Context) {
	fullPath, dirPath, err := resolveVideoPathFromBody(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := ensureVideoFileExists(c, fullPath); err != nil {
		return
	}
	if err := util.OpenFile(fullPath); err != nil {
		logging.Error("open video file error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "open file failed"})
		return
	}
	incrementPlayCountByPath(c.Request.Context(), dirPath, fullPath)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func playVideoFile(c *gin.Context) {
	req, fullPath, dirPath, err := resolveVideoPathRequestFromBody(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := ensureVideoFileExists(c, fullPath); err != nil {
		return
	}
	videoID := resolvePlaybackVideoID(c.Request.Context(), req.VideoID, dirPath, fullPath)
	dataDir := ""
	if common.AppConfig != nil {
		dataDir = filepath.Dir(common.AppConfig.DatabasePath)
	}
	if err := mpv.PlayVideo(fullPath, mpv.PlayOptions{
		DataDir:      dataDir,
		VideoID:      videoID,
		StartTimeSec: req.StartTimeSec,
	}); err != nil {
		logging.Error("play video file error: %v", err)
		if strings.Contains(err.Error(), "mpv not found") {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "play file failed"})
		return
	}
	if videoID > 0 {
		if err := dbpkg.IncrementVideoPlayCount(c.Request.Context(), videoID); err != nil {
			logging.Error("increment play count error: %v", err)
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func revealVideoLocation(c *gin.Context) {
	fullPath, _, err := resolveVideoPathFromBody(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := ensureVideoFileExists(c, fullPath); err != nil {
		return
	}
	if err := util.RevealFile(fullPath); err != nil {
		logging.Error("reveal video file error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "reveal file failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

type videoPathRequest struct {
	VideoID      int64   `json:"video_id"`
	Path         string  `json:"path"`
	DirPath      string  `json:"dir_path"`
	StartTimeSec float64 `json:"start_time"`
}

func resolveVideoPathFromBody(c *gin.Context) (string, string, error) {
	_, fullPath, dirPath, err := resolveVideoPathRequestFromBody(c)
	return fullPath, dirPath, err
}

func resolveVideoPathRequestFromBody(c *gin.Context) (videoPathRequest, string, string, error) {
	var req videoPathRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return req, "", "", errors.New("invalid payload")
	}
	if req.StartTimeSec < 0 {
		return req, "", "", errors.New("invalid start_time")
	}
	fullPath, dirPath, err := resolveVideoPath(req.Path, req.DirPath)
	return req, fullPath, dirPath, err
}

func resolveVideoPath(rawPath, rawDirPath string) (string, string, error) {
	if strings.TrimSpace(rawPath) == "" || strings.TrimSpace(rawDirPath) == "" {
		return "", "", errors.New("path and dir_path are required")
	}

	dirPath := filepath.Clean(rawDirPath)
	if dirPath == "." || !filepath.IsAbs(dirPath) {
		return "", "", errors.New("invalid dir_path")
	}

	cleanPath := filepath.Clean(filepath.FromSlash(rawPath))
	if cleanPath == "." {
		return "", "", errors.New("invalid path")
	}

	fullPath := cleanPath
	if !filepath.IsAbs(cleanPath) {
		fullPath = filepath.Join(dirPath, cleanPath)
	}

	relCheck, err := filepath.Rel(dirPath, fullPath)
	if err != nil || relCheck == ".." || strings.HasPrefix(relCheck, ".."+string(os.PathSeparator)) {
		return "", "", errors.New("invalid path")
	}
	return fullPath, dirPath, nil
}

func ensureVideoFileExists(c *gin.Context, fullPath string) error {
	if _, err := os.Stat(fullPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.Status(http.StatusNotFound)
			return err
		}
		logging.Error("stat stream file error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return err
	}
	return nil
}

func incrementPlayCountByPath(ctx context.Context, dirPath, fullPath string) {
	if strings.TrimSpace(dirPath) == "" || strings.TrimSpace(fullPath) == "" {
		return
	}
	relPath, err := filepath.Rel(dirPath, fullPath)
	if err != nil {
		logging.Error("resolve relative path for play count: %v", err)
		return
	}
	relPath = filepath.ToSlash(filepath.Clean(relPath))
	if relPath == "." || strings.HasPrefix(relPath, "..") {
		return
	}
	if err := dbpkg.IncrementVideoPlayCountByPath(ctx, dirPath, relPath); err != nil {
		logging.Error("increment play count by path error: %v", err)
	}
}

func resolvePlaybackVideoID(ctx context.Context, requestedID int64, dirPath, fullPath string) int64 {
	if requestedID > 0 {
		video, err := dbpkg.GetVideo(ctx, requestedID)
		if err != nil {
			logging.Error("get playback video error: %v", err)
		} else if video != nil {
			if candidate, err := resolveVideoPrimaryPath(ctx, video); err == nil && sameCleanPath(candidate, fullPath) {
				return video.ID
			}
		}
	}

	if strings.TrimSpace(dirPath) == "" || strings.TrimSpace(fullPath) == "" {
		return 0
	}
	relPath, err := filepath.Rel(dirPath, fullPath)
	if err != nil {
		logging.Error("resolve relative path for playback video id: %v", err)
		return 0
	}
	relPath = filepath.ToSlash(filepath.Clean(relPath))
	if relPath == "." || strings.HasPrefix(relPath, "..") {
		return 0
	}
	videoID, err := dbpkg.GetVideoIDByPath(ctx, dirPath, relPath)
	if err != nil {
		logging.Error("lookup playback video id by path error: %v", err)
		return 0
	}
	return videoID
}

func sameCleanPath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

func getThumbnail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	video, err := dbpkg.GetVideo(c.Request.Context(), id)
	if err != nil {
		logging.Error("get screenshot error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if video == nil {
		c.Status(http.StatusNotFound)
		return
	}

	second, ok := manager.PickScreenshotSecond(video.DurationSec)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}

	if common.AppConfig == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	dataDir := filepath.Dir(common.AppConfig.DatabasePath)
	screenshotPath := manager.ScreenshotPath(dataDir, video.ID, second)
	if screenshotPath == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if _, err := os.Stat(screenshotPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			common.ScreenshotManager.EnqueueForVideo(video)
			c.Status(http.StatusNotFound)
			return
		}
		logging.Error("stat screenshot error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.File(screenshotPath)
}

func listVideoScreenshots(c *gin.Context) {
	id, screenshotDir, ok := resolveVideoScreenshotDir(c)
	if !ok {
		return
	}

	entries, err := os.ReadDir(screenshotDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusOK, gin.H{"items": []videoScreenshotInfo{}})
			return
		}
		logging.Error("read video screenshots error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	items := make([]videoScreenshotInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !isScreenshotImageName(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			logging.Error("stat video screenshot error: %v", err)
			continue
		}
		name := entry.Name()
		imageURL := "/videos/" + strconv.FormatInt(id, 10) + "/screenshots/" + url.PathEscape(name)
		imageURL += "?mtime=" + strconv.FormatInt(info.ModTime().UnixNano(), 10)
		items = append(items, videoScreenshotInfo{
			Name:       name,
			URL:        imageURL,
			Size:       info.Size(),
			ModifiedAt: info.ModTime(),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Name != items[j].Name {
			return items[i].Name < items[j].Name
		}
		return items[i].ModifiedAt.Before(items[j].ModifiedAt)
	})

	c.JSON(http.StatusOK, gin.H{"items": items})
}

func getVideoScreenshot(c *gin.Context) {
	_, screenshotDir, ok := resolveVideoScreenshotDir(c)
	if !ok {
		return
	}

	name := filepath.Base(strings.TrimSpace(c.Param("name")))
	if !isScreenshotImageName(name) || name != strings.TrimSpace(c.Param("name")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid screenshot name"})
		return
	}

	screenshotPath := filepath.Join(screenshotDir, name)
	if _, err := os.Stat(screenshotPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.Status(http.StatusNotFound)
			return
		}
		logging.Error("stat video screenshot error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.File(screenshotPath)
}

func deleteVideoScreenshot(c *gin.Context) {
	_, screenshotDir, ok := resolveVideoScreenshotDir(c)
	if !ok {
		return
	}

	name := filepath.Base(strings.TrimSpace(c.Param("name")))
	if !isScreenshotImageName(name) || name != strings.TrimSpace(c.Param("name")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid screenshot name"})
		return
	}

	screenshotPath := filepath.Join(screenshotDir, name)
	if err := os.Remove(screenshotPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.Status(http.StatusNotFound)
			return
		}
		logging.Error("delete video screenshot error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func resolveVideoScreenshotDir(c *gin.Context) (int64, string, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0, "", false
	}

	video, err := dbpkg.GetVideo(c.Request.Context(), id)
	if err != nil {
		logging.Error("get video for screenshots error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return 0, "", false
	}
	if video == nil {
		c.Status(http.StatusNotFound)
		return 0, "", false
	}
	if common.AppConfig == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return 0, "", false
	}

	dataDir := filepath.Dir(common.AppConfig.DatabasePath)
	return id, filepath.Join(dataDir, "video", strconv.FormatInt(id, 10), "screenshot"), true
}

func isScreenshotImageName(name string) bool {
	if strings.TrimSpace(name) == "" || filepath.Base(name) != name {
		return false
	}
	if !strings.HasPrefix(name, "mpv_") {
		return false
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".webp":
		return true
	default:
		return false
	}
}
