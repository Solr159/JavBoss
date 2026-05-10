package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"pornboss/internal/common"
	"pornboss/internal/common/logging"
	dbpkg "pornboss/internal/db"
	"pornboss/internal/manager"
)

func listJavIdols(c *gin.Context) {
	limit := queryInt(c, "limit", 100)
	offset := queryInt(c, "offset", 0)
	search := strings.TrimSpace(c.Query("search"))
	sort := strings.TrimSpace(c.Query("sort"))
	directoryIDs := parseDirectoryIDs(c.Query("directory_ids"))

	items, total, err := dbpkg.ListJavIdols(c.Request.Context(), search, sort, limit, offset, directoryIDs)
	if err != nil {
		logging.Error("list jav idols: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	enrichJavIdolSummaries(c.Request.Context(), items, directoryIDs)

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
	})
}

func resolveJavIdols(c *gin.Context) {
	ids := parseInt64CSV(c.Query("ids"))
	items, err := dbpkg.ResolveJavIdols(c.Request.Context(), ids)
	if err != nil {
		logging.Error("resolve jav idols: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if items == nil {
		items = []dbpkg.JavIdolSummary{}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func getJavIdol(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	directoryIDs := parseDirectoryIDs(c.Query("directory_ids"))
	item, err := dbpkg.GetJavIdolSummary(c.Request.Context(), id, directoryIDs)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "idol not found"})
			return
		}
		logging.Error("get jav idol id=%d: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	items := []dbpkg.JavIdolSummary{*item}
	enrichJavIdolSummaries(c.Request.Context(), items, directoryIDs)
	c.JSON(http.StatusOK, items[0])
}

func enrichJavIdolSummaries(ctx context.Context, items []dbpkg.JavIdolSummary, directoryIDs []int64) {
	cfg := common.AppConfig
	coverDir := ""
	if cfg != nil {
		coverDir = cfg.JavCoverDir
	}
	for i := range items {
		enrichJavIdolSummary(ctx, &items[i], coverDir, directoryIDs)
	}
}

func enrichJavIdolSummary(ctx context.Context, item *dbpkg.JavIdolSummary, coverDir string, directoryIDs []int64) {
	item.Name = strings.TrimSpace(item.Name)
	item.RomanName = strings.TrimSpace(item.RomanName)
	item.JapaneseName = strings.TrimSpace(item.JapaneseName)
	item.ChineseName = strings.TrimSpace(item.ChineseName)

	if coverDir == "" {
		return
	}
	if _, ok := manager.FindCoverPath(coverDir, item.SampleCode); ok {
		return
	}
	codes, err := dbpkg.ListIdolCoverCodes(ctx, item.ID, directoryIDs)
	if err != nil {
		logging.Error("list idol cover codes id=%d: %v", item.ID, err)
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
