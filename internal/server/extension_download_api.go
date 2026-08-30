package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const javBossExtensionOrigin = "chrome-extension://iikdjhkpjihfkehccfmkpkdmenmbaacn"

func registerExtensionDownloadRoutes(router *gin.Engine) {
	router.OPTIONS("/extension/downloads", extensionDownloadsPreflight)
	router.POST("/extension/downloads", createExtensionDownloadJob)
}

func extensionDownloadsPreflight(c *gin.Context) {
	if !allowJavBossExtensionOrigin(c) {
		respondLocalizedError(c, http.StatusForbidden, "扩展来源无效", "Invalid extension origin")
		return
	}
	c.Header("Access-Control-Allow-Headers", "Content-Type")
	c.Header("Access-Control-Allow-Methods", http.MethodPost+", "+http.MethodOptions)
	c.Status(http.StatusNoContent)
}

func createExtensionDownloadJob(c *gin.Context) {
	if !allowJavBossExtensionOrigin(c) {
		respondLocalizedError(c, http.StatusForbidden, "扩展来源无效", "Invalid extension origin")
		return
	}
	var request struct {
		MagnetURL string `json:"magnet_url"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "下载请求格式不正确", "Invalid download request")
		return
	}
	enqueueDownloadJob(c, request.MagnetURL)
}

func allowJavBossExtensionOrigin(c *gin.Context) bool {
	origin := strings.TrimSuffix(strings.TrimSpace(c.GetHeader("Origin")), "/")
	if origin != javBossExtensionOrigin {
		return false
	}
	c.Header("Access-Control-Allow-Origin", javBossExtensionOrigin)
	c.Header("Vary", "Origin")
	return true
}
