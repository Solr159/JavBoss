package server

import (
	"context"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"javboss/internal/common"
	"javboss/internal/monitor"
	"javboss/internal/runtimeconfig"
)

var resourceSampler = monitor.NewSampler()

// GET /system/resources has no query parameters. Metrics describe this server,
// not the browser or a remote-mode desktop client. Unsupported values are null.
func getResources(c *gin.Context) {
	dataPath := "data"
	if common.AppConfig != nil && common.AppConfig.DatabasePath != "" {
		dataPath = filepath.Dir(common.AppConfig.DatabasePath)
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	c.Header("Cache-Control", "no-store")
	snapshot, err := resourceSampler.Sample(ctx, dataPath, runtimeconfig.ContainerMode())
	if err != nil {
		respondLocalizedError(c, http.StatusServiceUnavailable, "资源信息暂不可用", "Resource metrics are temporarily unavailable")
		return
	}
	c.JSON(http.StatusOK, snapshot)
}
