package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"javboss/internal/common"
)

func getTools(c *gin.Context) {
	if common.FFmpegToolManager == nil {
		respondLocalizedError(c, http.StatusServiceUnavailable, "工具服务不可用", "Tool service is unavailable")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ffmpeg": common.FFmpegToolManager.Status(),
	})
}

func downloadFFmpeg(c *gin.Context) {
	if common.FFmpegToolManager == nil {
		respondLocalizedError(c, http.StatusServiceUnavailable, "工具服务不可用", "Tool service is unavailable")
		return
	}
	started, err := common.FFmpegToolManager.StartDownload()
	if err != nil {
		respondLocalizedError(c, http.StatusUnprocessableEntity, "当前平台不支持自动下载 FFmpeg", err.Error())
		return
	}
	status := http.StatusOK
	if started {
		status = http.StatusAccepted
	}
	c.JSON(status, common.FFmpegToolManager.Status())
}
