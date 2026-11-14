package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	dbpkg "javboss/internal/db"
	"javboss/internal/mpv"
)

// GET/PATCH /videos/:id/subtitle-settings reads or updates
// {offset_ms?: integer -3600000..3600000, track?: string (max 1024 bytes)}.
// Track is empty for the default, "off" to disable, or a stable track key.
// The setting belongs to the video
// content, so copies share it while different edits of one JAV remain separate.
func videoSubtitleSettings(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, 400, "视频 ID 无效", "Invalid video ID")
		return
	}
	if c.Request.Method == http.MethodGet {
		c.Header("Cache-Control", "no-store")
		video, err := dbpkg.Default().GetVideo(c.Request.Context(), id)
		if err != nil {
			respondLocalizedError(c, 500, "加载字幕设置失败", "Failed to load subtitle settings")
			return
		}
		if video == nil {
			respondLocalizedError(c, 404, "视频不存在", "Video not found")
			return
		}
		c.JSON(http.StatusOK, video.SubtitleSettings)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	var req struct {
		OffsetMS *int64  `json:"offset_ms"`
		Track    *string `json:"track"`
	}
	if c.ShouldBindJSON(&req) != nil || (req.Track != nil && len(*req.Track) > 1024) || (req.OffsetMS == nil && req.Track == nil) || (req.OffsetMS != nil && (*req.OffsetMS < -3600000 || *req.OffsetMS > 3600000)) {
		respondLocalizedError(c, 400, "字幕设置无效，偏移须在 -3600 到 3600 秒之间", "Invalid subtitle settings; offset must be between -3600 and 3600 seconds")
		return
	}
	found, err := dbpkg.Default().UpdateVideoSubtitleSettings(c.Request.Context(), id, req.OffsetMS, req.Track)
	if err != nil {
		respondLocalizedError(c, 500, "保存字幕设置失败", "Failed to save subtitle settings")
		return
	}
	if !found {
		respondLocalizedError(c, 404, "视频不存在", "Video not found")
		return
	}
	response := gin.H{}
	if req.Track != nil {
		response["track"] = *req.Track
	}
	if req.OffsetMS != nil {
		response["offset_ms"] = *req.OffsetMS
	}
	c.JSON(http.StatusOK, response)
}

func localSubtitleSelectionSaver(videoID int64) mpv.SubtitleSelectionSaver {
	if videoID <= 0 {
		return nil
	}
	return func(ctx context.Context, track string) error {
		found, err := dbpkg.Default().UpdateVideoSubtitleSettings(ctx, videoID, nil, &track)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("video %d not found", videoID)
		}
		return nil
	}
}

func localSubtitleOffsetSaver(videoID int64) mpv.SubtitleOffsetSaver {
	if videoID <= 0 {
		return nil
	}
	return func(ctx context.Context, offsetMS int64) error {
		found, err := dbpkg.Default().UpdateVideoSubtitleOffset(ctx, videoID, offsetMS)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("video %d not found", videoID)
		}
		return nil
	}
}
