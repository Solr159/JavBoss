package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"pornboss/internal/common"
	"pornboss/internal/common/logging"
	dbpkg "pornboss/internal/db"
	"pornboss/internal/manager"
)

func listJavStudios(c *gin.Context) {
	limit := queryInt(c, "limit", 100)
	offset := queryInt(c, "offset", 0)
	search := strings.TrimSpace(c.Query("search"))
	directoryIDs := parseDirectoryIDs(c.Query("directory_ids"))

	items, total, err := dbpkg.ListJavStudios(c.Request.Context(), search, limit, offset, directoryIDs)
	if err != nil {
		logging.Error("list jav studios: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	enrichJavStudioSummaries(c.Request.Context(), items, directoryIDs)

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
	})
}

func enrichJavStudioSummaries(ctx context.Context, items []dbpkg.JavStudioSummary, directoryIDs []int64) {
	cfg := common.AppConfig
	coverDir := ""
	if cfg != nil {
		coverDir = cfg.JavCoverDir
	}
	for i := range items {
		enrichJavStudioSummary(ctx, &items[i], coverDir, directoryIDs)
	}
}

func enrichJavStudioSummary(ctx context.Context, item *dbpkg.JavStudioSummary, coverDir string, directoryIDs []int64) {
	item.Name = strings.TrimSpace(item.Name)
	item.SampleCode = strings.TrimSpace(item.SampleCode)

	if coverDir == "" {
		return
	}
	if _, ok := manager.FindCoverPath(coverDir, item.SampleCode); ok {
		return
	}
	codes, err := dbpkg.ListStudioCoverCodes(ctx, item.ID, directoryIDs)
	if err != nil {
		logging.Error("list studio cover codes id=%d: %v", item.ID, err)
		return
	}
	var chosen string
	for _, code := range codes {
		if _, ok := manager.FindCoverPath(coverDir, code); ok {
			chosen = code
			break
		}
	}
	if chosen == "" && len(codes) > 0 {
		chosen = codes[0]
	}
	if chosen != "" {
		item.SampleCode = chosen
		if common.CoverManager != nil && !common.CoverManager.Exists(chosen) {
			common.CoverManager.Enqueue(chosen)
		}
	}
}
