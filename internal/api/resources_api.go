package api

import (
	"context"
	"net/http"
	"path/filepath"
	"time"

	"javboss/internal/common"
	"javboss/internal/monitor"

	"github.com/gin-gonic/gin"
)

// GET /system/resources has no query parameters. Metrics describe this server,
// not the browser or a remote-mode desktop client. Unsupported values are null.
func getResources(c *gin.Context) {
	dataPath := "data"
	if cfg := common.CurrentConfig(); cfg != nil && cfg.DatabasePath != "" {
		dataPath = filepath.Dir(cfg.DatabasePath)
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	c.Header("Cache-Control", "no-store")
	snapshot, err := monitor.Sample(ctx, dataPath, common.ContainerMode())
	if err != nil {
		respondLocalizedError(c, http.StatusServiceUnavailable, "资源信息暂不可用", "Resource metrics are temporarily unavailable")
		return
	}
	c.JSON(http.StatusOK, snapshot)
}
