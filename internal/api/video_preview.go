package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"javboss/internal/common/logging"
	"javboss/internal/video/preview"

	"github.com/gin-gonic/gin"
)

// GET /videos/:id/preview?second=N&location_id=N returns a JPEG for the
// ten-second interval containing second. location_id selects the playback file.
func getVideoPreview(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	second, err := strconv.ParseFloat(c.Query("second"), 64)
	if err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "预览时间无效", "Invalid preview timestamp")
		return
	}
	video, source, _, err := resolveVideoStreamTarget(c)
	if err != nil {
		respondPlaybackError(c, err)
		return
	}
	duration := float64(video.DurationSec)
	if video.MediaInfo.DurationSeconds > 0 {
		duration = video.MediaInfo.DurationSeconds
	}
	bucket, err := preview.Second(second, duration)
	if err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "预览时间超出视频时长或时长未知", "Invalid preview timestamp or unknown duration")
		return
	}
	data, err := preview.Get(c.Request.Context(), video.ID, source, bucket)
	if err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, preview.ErrUnavailable) {
			logging.Error("generate video preview (video_id=%d): %v", video.ID, err)
		}
		c.Header("Retry-After", "3")
		respondLocalizedError(c, http.StatusServiceUnavailable, "视频预览暂不可用", "Video preview is temporarily unavailable")
		return
	}
	c.Data(http.StatusOK, "image/jpeg", data)
}
