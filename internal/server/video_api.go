package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"javboss/internal/common"
	"javboss/internal/common/logging"
	dbpkg "javboss/internal/db"
	"javboss/internal/jav"
	"javboss/internal/manager"
	"javboss/internal/models"
	"javboss/internal/mpv"
	"javboss/internal/runtimeconfig"
	"javboss/internal/util"
)

func listVideos(c *gin.Context) {
	limit := queryInt(c, "limit", 100)
	offset := queryInt(c, "offset", 0)
	tagFilter := parseTagQuery(c.Query("tags"))
	directoryIDs := parseDirectoryIDs(c.Query("directory_ids"))
	search := strings.TrimSpace(c.Query("search"))
	sort := strings.TrimSpace(c.Query("sort"))
	hideJav := queryBool(c, "hide_jav", false)
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

type renameVideoLocationRequest struct {
	Filename string `json:"filename"`
}

type videoJavScrapeSettingsRequest struct {
	Mode string `json:"mode"`
	Code string `json:"code"`
}

type videoJavManualScrapeRequest struct {
	LocationID   int64    `json:"location_id"`
	Code         string   `json:"code"`
	Title        string   `json:"title"`
	Studio       string   `json:"studio"`
	Series       string   `json:"series"`
	ReleaseDate  string   `json:"release_date"`
	DurationMin  *int     `json:"duration_min"`
	Tags         []string `json:"tags"`
	Actors       []string `json:"actors"`
	CoverURL     string   `json:"cover_url"`
	IsUncensored *bool    `json:"is_uncensored"`
}

type videoJavScrapeInfoResponse struct {
	Code         string   `json:"code"`
	Title        string   `json:"title"`
	Studio       string   `json:"studio"`
	Series       string   `json:"series"`
	ReleaseDate  string   `json:"release_date"`
	ReleaseUnix  int64    `json:"release_unix"`
	DurationMin  int      `json:"duration_min"`
	Tags         []string `json:"tags"`
	Actors       []string `json:"actors"`
	CoverURL     string   `json:"cover_url"`
	IsUncensored *bool    `json:"is_uncensored"`
}

type videoJavPossibleCodesResponse struct {
	Filename      string   `json:"filename"`
	PossibleCodes []string `json:"possible_codes"`
}

func getVideoStreams(c *gin.Context) {
	video, fullPath, locationID, err := resolveVideoStreamTarget(c)
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
		VideoID:       video.ID,
		PreferredKind: "hls",
		Sources:       []playbackSource{},
	}
	if probe.SupportsDirect {
		info.PreferredKind = "direct"
		info.Sources = append(info.Sources, playbackSource{
			Kind:     "direct",
			Src:      buildDirectStreamURL(video, locationID),
			MimeType: directMimeType(probe.Container),
			Label:    "Direct",
		})
	}
	info.Sources = append(info.Sources, playbackSource{
		Kind:     "hls",
		Src:      buildHLSStreamURL(video, locationID),
		MimeType: manager.MimeHLS,
		Label:    "HLS",
	})

	c.JSON(http.StatusOK, info)
}

func streamVideo(c *gin.Context) {
	var fullPath string
	var err error
	if strings.TrimSpace(c.Query("location_id")) != "" {
		fullPath, err = resolveStreamPathFromLocationQuery(c)
	} else {
		fullPath, err = resolveStreamPathFromQuery(c)
	}
	if err != nil {
		_, fullPath, _, err = resolveVideoStreamTarget(c)
		if err != nil {
			respondPlaybackError(c, err)
			return
		}
	}
	serveVideoFile(c, fullPath)
}

func streamHLSManifest(c *gin.Context) {
	video, fullPath, _, err := resolveVideoStreamTarget(c)
	if err != nil {
		respondPlaybackError(c, err)
		return
	}
	if common.StreamManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "stream manager unavailable"})
		return
	}

	resolution := strings.TrimSpace(c.Query("resolution"))
	c.Header("Cache-Control", "no-cache")
	common.StreamManager.ServeManifest(c.Writer, c.Request, fullPath, float64(video.DurationSec), resolution)
}

func streamHLSSegment(c *gin.Context) {
	video, fullPath, locationID, err := resolveVideoStreamTarget(c)
	if err != nil {
		respondPlaybackError(c, err)
		return
	}
	if common.StreamManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "stream manager unavailable"})
		return
	}

	segment := strings.TrimSpace(c.Param("segment"))
	resolution := strings.TrimSpace(c.Query("resolution"))
	c.Header("Cache-Control", "no-cache")
	common.StreamManager.ServeSegment(c.Writer, c.Request, manager.StreamOptions{
		StreamType: manager.StreamTypeHLS,
		SourcePath: fullPath,
		Duration:   float64(video.DurationSec),
		Resolution: resolution,
		Key:        streamCacheKey(video.ID, locationID),
		Segment:    segment,
	})
}

func resolveStreamPathFromLocationQuery(c *gin.Context) (string, error) {
	videoID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || videoID <= 0 {
		return "", errors.New("invalid id")
	}
	locationID, err := parseLocationIDQuery(c)
	if err != nil || locationID <= 0 {
		return "", err
	}
	loc, err := dbpkg.GetActiveVideoLocation(c.Request.Context(), videoID, locationID)
	if err != nil {
		return "", err
	}
	if loc == nil {
		return "", os.ErrNotExist
	}
	fullPath, _, err := resolveVideoPath(loc.RelativePath, loc.DirectoryRef.Path)
	return fullPath, err
}

func resolveStreamPathFromQuery(c *gin.Context) (string, error) {
	rawPath := strings.TrimSpace(c.Query("path"))
	rawDirPath := strings.TrimSpace(c.Query("dir_path"))
	fullPath, _, err := resolveVideoPath(rawPath, rawDirPath)
	return fullPath, err
}

func resolveVideoStreamTarget(c *gin.Context) (*models.Video, string, int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		return nil, "", 0, errors.New("invalid id")
	}

	video, err := dbpkg.GetVideo(c.Request.Context(), id)
	if err != nil {
		return nil, "", 0, err
	}
	if video == nil {
		return nil, "", 0, os.ErrNotExist
	}

	locationID, err := parseLocationIDQuery(c)
	if err != nil {
		return nil, "", 0, err
	}

	var fullPath string
	if locationID > 0 {
		fullPath, err = resolveStreamPathFromLocationQuery(c)
	} else {
		fullPath, err = resolveVideoPrimaryPath(c.Request.Context(), video)
	}
	if err != nil {
		return nil, "", 0, err
	}
	if _, err := os.Stat(fullPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", 0, err
		}
		return nil, "", 0, err
	}

	return video, fullPath, locationID, nil
}

func parseLocationIDQuery(c *gin.Context) (int64, error) {
	raw := strings.TrimSpace(c.Query("location_id"))
	if raw == "" {
		return 0, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid location_id")
	}
	return id, nil
}

func respondPlaybackError(c *gin.Context, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, os.ErrNotExist):
		c.Status(http.StatusNotFound)
	case errors.Is(err, context.Canceled):
		c.Status(499)
	case strings.Contains(err.Error(), "ffmpeg not found"), strings.Contains(err.Error(), "ffprobe not found"):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
	case strings.Contains(err.Error(), "browser playback is not supported"):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	case strings.Contains(err.Error(), "invalid segment"), strings.Contains(err.Error(), "invalid id"), strings.Contains(err.Error(), "invalid location_id"), strings.Contains(err.Error(), "invalid path"):
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

func buildDirectStreamURL(video *models.Video, locationID int64) string {
	if video == nil {
		return ""
	}
	streamURL := "/videos/" + strconv.FormatInt(video.ID, 10) + "/stream"
	if locationID > 0 {
		streamURL += "?location_id=" + strconv.FormatInt(locationID, 10)
	}
	return streamURL
}

func buildHLSStreamURL(video *models.Video, locationID int64) string {
	if video == nil {
		return ""
	}
	streamURL := "/videos/" + strconv.FormatInt(video.ID, 10) + "/stream.m3u8"
	if locationID > 0 {
		streamURL += "?location_id=" + strconv.FormatInt(locationID, 10)
	}
	return streamURL
}

func streamCacheKey(videoID int64, locationID int64) string {
	if locationID > 0 {
		return fmt.Sprintf("%d_location_%d", videoID, locationID)
	}
	return strconv.FormatInt(videoID, 10)
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
	if runtimeconfig.DisableDesktopIntegration() {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "desktop file opening is disabled"})
		return
	}
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
	if runtimeconfig.DisableMPVPlayback() {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "mpv playback is disabled"})
		return
	}
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
	if runtimeconfig.DisableDesktopIntegration() {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "desktop file revealing is disabled"})
		return
	}
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

func renameVideoLocation(c *gin.Context) {
	videoID, locationID, ok := parseVideoLocationParams(c)
	if !ok {
		return
	}

	var req renameVideoLocationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	filename := strings.TrimSpace(req.Filename)
	if !isSafeVideoFilename(filename) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid filename"})
		return
	}

	loc, err := dbpkg.GetActiveVideoLocation(c.Request.Context(), videoID, locationID)
	if err != nil {
		logging.Error("get video location for rename error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if loc == nil {
		c.Status(http.StatusNotFound)
		return
	}

	currentRel := filepath.ToSlash(filepath.Clean(filepath.FromSlash(loc.RelativePath)))
	parentRel := filepath.ToSlash(filepath.Dir(filepath.FromSlash(currentRel)))
	nextRel := filename
	if parentRel != "." && parentRel != "" {
		nextRel = filepath.ToSlash(filepath.Join(parentRel, filename))
	}
	nextRel = filepath.ToSlash(filepath.Clean(filepath.FromSlash(nextRel)))
	if nextRel == "." || strings.HasPrefix(nextRel, "../") || nextRel == ".." {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid filename"})
		return
	}
	if nextRel == currentRel {
		video, err := dbpkg.GetVideoForLocation(c.Request.Context(), videoID, locationID)
		if err != nil {
			logging.Error("load unchanged video location error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		if video == nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.JSON(http.StatusOK, video)
		return
	}

	exists, err := dbpkg.VideoLocationPathExists(c.Request.Context(), loc.DirectoryID, nextRel)
	if err != nil {
		logging.Error("check video location path error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "target path already exists"})
		return
	}

	oldFullPath, dirPath, err := resolveVideoPath(currentRel, loc.DirectoryRef.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	newFullPath, _, err := resolveVideoPath(nextRel, dirPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	info, err := os.Stat(oldFullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.Status(http.StatusNotFound)
			return
		}
		logging.Error("stat video before rename error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if info.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is not a file"})
		return
	}
	if targetInfo, err := os.Stat(newFullPath); err == nil {
		if !os.SameFile(info, targetInfo) {
			c.JSON(http.StatusConflict, gin.H{"error": "target file already exists"})
			return
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		logging.Error("stat video rename target error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if err := os.Rename(oldFullPath, newFullPath); err != nil {
		logging.Error("rename video file error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "rename file failed"})
		return
	}
	modifiedAt := info.ModTime()
	if renamedInfo, err := os.Stat(newFullPath); err == nil {
		modifiedAt = renamedInfo.ModTime()
	}
	if _, err := dbpkg.UpdateVideoLocationPath(c.Request.Context(), locationID, nextRel, modifiedAt); err != nil {
		if rollbackErr := os.Rename(newFullPath, oldFullPath); rollbackErr != nil {
			logging.Error("rollback video file rename failed: %v", rollbackErr)
		}
		if errors.Is(err, dbpkg.ErrVideoLocationPathConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "target path already exists"})
			return
		}
		logging.Error("update video location after rename error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	video, err := dbpkg.GetVideoForLocation(c.Request.Context(), videoID, locationID)
	if err != nil {
		logging.Error("load renamed video location error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if video == nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.JSON(http.StatusOK, video)
}

func updateVideoJavScrapeSettings(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req videoJavScrapeSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	override, ok := normalizeVideoJavScrapeOverride(req)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid jav scrape settings"})
		return
	}

	video, err := dbpkg.UpdateVideoJavScrapeOverride(c.Request.Context(), id, override)
	if err != nil {
		logging.Error("update video jav scrape settings error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if video == nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.JSON(http.StatusOK, video)
}

func getVideoJavScrapePossibleCodes(c *gin.Context) {
	id, ok := parsePositiveVideoID(c)
	if !ok {
		return
	}

	video, err := dbpkg.GetVideo(c.Request.Context(), id)
	if err != nil {
		logging.Error("load video for jav scrape possible codes error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if video == nil {
		c.Status(http.StatusNotFound)
		return
	}

	filename := filepath.Base(filepath.FromSlash(video.Filename))
	c.JSON(http.StatusOK, videoJavPossibleCodesResponse{
		Filename:      filename,
		PossibleCodes: util.ExtractCodeFromName(filename),
	})
}

func lookupVideoJavScrapeJavDB(c *gin.Context) {
	if _, ok := parsePositiveVideoID(c); !ok {
		return
	}
	code := strings.TrimSpace(c.Query("code"))
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code is required"})
		return
	}

	info, err := jav.LookupJavByCode(code, jav.ProviderJavDB)
	if err != nil {
		if errors.Is(err, jav.ResourceNotFonud) {
			c.JSON(http.StatusNotFound, gin.H{"error": "javdb metadata not found"})
			return
		}
		logging.Error("lookup javdb metadata code=%s: %v", code, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if info == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "javdb metadata not found"})
		return
	}
	c.JSON(http.StatusOK, javInfoToVideoScrapeResponse(info))
}

func manualVideoJavScrape(c *gin.Context) {
	id, ok := parsePositiveVideoID(c)
	if !ok {
		return
	}

	var req videoJavManualScrapeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	info, err := manualScrapeRequestToJavInfo(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.LocationID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "location_id is required"})
		return
	}

	loc, err := dbpkg.GetActiveVideoLocation(c.Request.Context(), id, req.LocationID)
	if err != nil {
		logging.Error("load video location for manual jav scrape video=%d location=%d: %v", id, req.LocationID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if loc == nil {
		c.Status(http.StatusNotFound)
		return
	}

	javRec, err := dbpkg.SaveJavInfoAndLinkVideoLocations(c.Request.Context(), info, id)
	if err != nil {
		logging.Error("manual jav scrape save failed video=%d code=%s: %v", id, info.Code, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if javRec == nil {
		c.Status(http.StatusNotFound)
		return
	}

	manualOverride := models.JavScrapeOverrideManualPrefix + info.Code
	if _, err := dbpkg.UpdateVideoJavScrapeOverride(c.Request.Context(), id, manualOverride); err != nil {
		logging.Error("manual jav scrape update override failed video=%d code=%s: %v", id, info.Code, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	video, err := dbpkg.GetVideoForLocation(c.Request.Context(), id, loc.ID)
	if err != nil {
		logging.Error("manual jav scrape reload failed video=%d location=%d code=%s: %v", id, loc.ID, info.Code, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if video == nil {
		c.Status(http.StatusNotFound)
		return
	}

	downloadManualJavCover(c.Request.Context(), info)
	c.JSON(http.StatusOK, video)
}

func parsePositiveVideoID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0, false
	}
	return id, true
}

func manualScrapeRequestToJavInfo(req videoJavManualScrapeRequest) (*jav.JavInfo, error) {
	code := strings.ToUpper(strings.TrimSpace(req.Code))
	if code == "" {
		return nil, errors.New("code is required")
	}
	releaseUnix, err := parseJavEditReleaseUnix(req.ReleaseDate)
	if err != nil {
		return nil, err
	}
	durationMin := 0
	if req.DurationMin != nil {
		durationMin = *req.DurationMin
		if durationMin < 0 {
			return nil, errors.New("duration_min must be non-negative")
		}
	}
	info := &jav.JavInfo{
		Code:         code,
		Title:        strings.TrimSpace(req.Title),
		Studio:       strings.TrimSpace(req.Studio),
		Series:       strings.TrimSpace(req.Series),
		ReleaseUnix:  releaseUnix,
		DurationMin:  durationMin,
		Tags:         normalizeTextList(req.Tags),
		Actors:       normalizeTextList(req.Actors),
		CoverURL:     strings.TrimSpace(req.CoverURL),
		IsUncensored: req.IsUncensored,
		Provider:     jav.ProviderJavDB,
	}
	return info, nil
}

func normalizeTextList(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func javInfoToVideoScrapeResponse(info *jav.JavInfo) videoJavScrapeInfoResponse {
	if info == nil {
		return videoJavScrapeInfoResponse{}
	}
	return videoJavScrapeInfoResponse{
		Code:         info.Code,
		Title:        info.Title,
		Studio:       info.Studio,
		Series:       info.Series,
		ReleaseDate:  formatUnixDate(info.ReleaseUnix),
		ReleaseUnix:  info.ReleaseUnix,
		DurationMin:  info.DurationMin,
		Tags:         append([]string(nil), info.Tags...),
		Actors:       append([]string(nil), info.Actors...),
		CoverURL:     info.CoverURL,
		IsUncensored: info.IsUncensored,
	}
}

func formatUnixDate(unix int64) string {
	if unix <= 0 {
		return ""
	}
	return time.Unix(unix, 0).UTC().Format("2006-01-02")
}

func downloadManualJavCover(ctx context.Context, info *jav.JavInfo) {
	if info == nil || strings.TrimSpace(info.Code) == "" || strings.TrimSpace(info.CoverURL) == "" {
		return
	}
	cfg := common.AppConfig
	if cfg == nil || strings.TrimSpace(cfg.JavCoverDir) == "" {
		return
	}
	coverCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if err := manager.DownloadCoverFromURL(coverCtx, cfg.JavCoverDir, info.Code, info.CoverURL); err != nil {
		logging.Error("manual jav cover download failed code=%s: %v", info.Code, err)
	}
}

func normalizeVideoJavScrapeOverride(req videoJavScrapeSettingsRequest) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(req.Mode)) {
	case "", "auto":
		return "", true
	case "skip":
		return models.JavScrapeOverrideSkip, true
	case "code":
		code, ok := normalizeForcedJavScrapeCode(req.Code)
		return code, ok
	default:
		return "", false
	}
}

func normalizeForcedJavScrapeCode(raw string) (string, bool) {
	code := strings.ToUpper(strings.TrimSpace(raw))
	if code == "" || len(code) > 64 {
		return "", false
	}
	for _, r := range code {
		switch {
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return "", false
		}
	}
	return code, true
}

func deleteVideoLocation(c *gin.Context) {
	videoID, locationID, ok := parseVideoLocationParams(c)
	if !ok {
		return
	}

	loc, err := dbpkg.GetActiveVideoLocation(c.Request.Context(), videoID, locationID)
	if err != nil {
		logging.Error("get video location for delete error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if loc == nil {
		c.Status(http.StatusNotFound)
		return
	}

	fullPath, _, err := resolveVideoPath(loc.RelativePath, loc.DirectoryRef.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			logging.Error("stat video before delete error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
	} else if info.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is not a file"})
		return
	} else if err := util.MoveFileToTrash(fullPath); err != nil {
		logging.Error("delete video file error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete file failed"})
		return
	}

	if err := dbpkg.HideVideoLocationsByIDs(c.Request.Context(), []int64{locationID}); err != nil {
		logging.Error("hide deleted video location error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func parseVideoLocationParams(c *gin.Context) (int64, int64, bool) {
	videoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || videoID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0, 0, false
	}
	locationID, err := strconv.ParseInt(c.Param("location_id"), 10, 64)
	if err != nil || locationID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid location_id"})
		return 0, 0, false
	}
	return videoID, locationID, true
}

func isSafeVideoFilename(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	if strings.ContainsAny(name, `/\`) {
		return false
	}
	return filepath.Base(name) == name
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

func createVideoScreenshot(c *gin.Context) {
	if common.ScreenshotManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "screenshot manager unavailable"})
		return
	}
	video, fullPath, _, err := resolveVideoStreamTarget(c)
	if err != nil {
		respondPlaybackError(c, err)
		return
	}
	if common.AppConfig == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	var req struct {
		Second float64 `json:"second"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if req.Second < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid second"})
		return
	}
	if video.DurationSec > 0 && req.Second > float64(video.DurationSec)+1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid second"})
		return
	}

	dataDir := filepath.Dir(common.AppConfig.DatabasePath)
	screenshotDir := filepath.Join(dataDir, "video", strconv.FormatInt(video.ID, 10), "screenshot")
	name := playbackScreenshotName(req.Second)
	screenshotPath := filepath.Join(screenshotDir, name)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()
	if err := common.ScreenshotManager.CaptureFile(ctx, fullPath, req.Second, screenshotPath); err != nil {
		logging.Error("create video screenshot error: %v", err)
		if strings.Contains(err.Error(), "ffmpeg not found") || strings.Contains(err.Error(), "mpv not found") {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "screenshot failed"})
		return
	}

	info, err := os.Stat(screenshotPath)
	if err != nil {
		logging.Error("stat created video screenshot error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	imageURL := "/videos/" + strconv.FormatInt(video.ID, 10) + "/screenshots/" + url.PathEscape(name)
	imageURL += "?mtime=" + strconv.FormatInt(info.ModTime().UnixNano(), 10)
	c.JSON(http.StatusCreated, videoScreenshotInfo{
		Name:       name,
		URL:        imageURL,
		Size:       info.Size(),
		ModifiedAt: info.ModTime(),
	})
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
	if err := util.MoveFileToTrash(screenshotPath); err != nil {
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

func playbackScreenshotName(second float64) string {
	totalMillis := int64(second*1000 + 0.5)
	if totalMillis < 0 {
		totalMillis = 0
	}
	totalSeconds := totalMillis / 1000
	millis := totalMillis % 1000
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60
	if millis > 0 {
		return fmt.Sprintf("mpv_%02d-%02d-%02d.%03d.jpg", hours, minutes, seconds, millis)
	}
	return fmt.Sprintf("mpv_%02d-%02d-%02d.jpg", hours, minutes, seconds)
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
