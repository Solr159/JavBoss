package server

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	dbpkg "javboss/internal/db"
	"javboss/internal/models"
	"javboss/internal/playback"
)

var playbackSessions = playback.NewSessions(saveWatchedTime)

// POST /videos/:id/playback-sessions accepts {location_id}; zero selects the
// primary location. Legacy callers may supply {path, dir_path} instead.
// The response is 201 {session_id}; the JAV is resolved here, never supplied by clients.
func createPlaybackSession(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		LocationID int64  `json:"location_id"`
		Path       string `json:"path"`
		DirPath    string `json:"dir_path"`
	}
	if err != nil || id <= 0 || c.ShouldBindJSON(&req) != nil || req.LocationID < 0 {
		respondLocalizedError(c, http.StatusBadRequest, "播放会话参数无效", "Invalid playback session")
		return
	}
	var loc *models.VideoLocation
	if req.LocationID > 0 {
		loc, err = dbpkg.GetActiveVideoLocation(c.Request.Context(), id, req.LocationID)
	} else if req.Path != "" && req.DirPath != "" {
		fullPath, dirPath, pathErr := resolveVideoPath(req.Path, req.DirPath)
		if pathErr != nil {
			respondLocalizedError(c, http.StatusBadRequest, "视频路径无效", "Invalid video path")
			return
		}
		rel, pathErr := filepath.Rel(dirPath, fullPath)
		if pathErr != nil {
			respondLocalizedError(c, http.StatusBadRequest, "视频路径无效", "Invalid video path")
			return
		}
		loc, err = dbpkg.GetVideoLocationByPath(c.Request.Context(), dirPath, filepath.ToSlash(rel))
	} else {
		loc, err = dbpkg.GetPrimaryVideoLocation(c.Request.Context(), id)
	}
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "加载视频位置失败", "Failed to load video location")
		return
	}
	if loc == nil || loc.VideoID != id {
		respondLocalizedError(c, http.StatusNotFound, "视频位置不存在", "Video location not found")
		return
	}
	javID := int64(0)
	if loc.JavID != nil {
		javID = *loc.JavID
	}
	sessionID, err := playbackSessions.Create(id, javID)
	if err != nil {
		respondLocalizedError(c, http.StatusServiceUnavailable, "创建播放会话失败", "Could not create playback session")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"session_id": sessionID})
}

// PUT /videos/:id/playback-sessions/:session_id accepts cumulative {watched_ms}.
// Repeated/older checkpoints are no-ops. A 410 requires a new session and baseline.
func reportPlaybackSession(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		WatchedMS *int64 `json:"watched_ms"`
	}
	if err != nil || id <= 0 || c.ShouldBindJSON(&req) != nil || req.WatchedMS == nil {
		respondLocalizedError(c, http.StatusBadRequest, "播放时长参数无效", "Invalid playback checkpoint")
		return
	}
	err = playbackSessions.Report(c.Request.Context(), c.Param("session_id"), id, *req.WatchedMS)
	switch {
	case errors.Is(err, playback.ErrExpired):
		respondLocalizedError(c, http.StatusGone, "播放会话已失效", "Playback session expired")
	case errors.Is(err, playback.ErrInvalid):
		respondLocalizedError(c, http.StatusBadRequest, "播放时长参数无效", "Invalid playback checkpoint")
	case err != nil:
		respondLocalizedError(c, http.StatusInternalServerError, "保存播放时长失败", "Failed to save watched time")
	default:
		c.Status(http.StatusNoContent)
	}
}

// Resolve the JAV again when an entry actually starts (including playlist replays),
// then freeze that attribution for the lifetime of this playback session.
func localPlaybackReporter(videoID, locationID int64, dirPath, fullPath string) func() *playback.Reporter {
	if videoID <= 0 {
		return nil
	}
	return func() *playback.Reporter {
		var javID int64
		resolved := false
		return playback.NewReporter(func(ctx context.Context) (string, error) {
			if !resolved {
				var loc *models.VideoLocation
				var err error
				if locationID > 0 {
					loc, err = dbpkg.GetActiveVideoLocation(ctx, videoID, locationID)
				} else if fullPath != "" {
					var rel string
					rel, err = filepath.Rel(dirPath, fullPath)
					if err == nil {
						loc, err = dbpkg.GetVideoLocationByPath(ctx, dirPath, filepath.ToSlash(rel))
					}
				}
				if err != nil {
					return "", err
				}
				if loc != nil && loc.VideoID == videoID && loc.JavID != nil {
					javID = *loc.JavID
				}
				resolved = true
			}
			return playbackSessions.Create(videoID, javID)
		}, func(ctx context.Context, id string, total int64) error {
			return playbackSessions.Report(ctx, id, videoID, total)
		})
	}
}
