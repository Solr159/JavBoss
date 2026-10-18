package api

import (
	"errors"
	"net/http"

	"javboss/internal/ffmpeg"

	"github.com/gin-gonic/gin"
)

func getTools(c *gin.Context) {
	toolStatus, err := ffmpeg.GetStatus()
	if err != nil {
		respondLocalizedError(c, http.StatusServiceUnavailable, "工具服务不可用", "Tool service is unavailable")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ffmpeg": toolStatus,
	})
}

func downloadFFmpeg(c *gin.Context) {
	started, err := ffmpeg.StartDownload()
	if errors.Is(err, ffmpeg.ErrUnavailable) {
		respondLocalizedError(c, http.StatusServiceUnavailable, "工具服务不可用", "Tool service is unavailable")
		return
	}
	if err != nil {
		respondLocalizedError(c, http.StatusUnprocessableEntity, "当前平台不支持自动下载 FFmpeg", err.Error())
		return
	}
	status := http.StatusOK
	if started {
		status = http.StatusAccepted
	}
	toolStatus, err := ffmpeg.GetStatus()
	if err != nil {
		respondLocalizedError(c, http.StatusServiceUnavailable, "工具服务不可用", "Tool service is unavailable")
		return
	}
	c.JSON(status, toolStatus)
}
