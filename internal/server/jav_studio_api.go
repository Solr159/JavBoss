package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"javboss/internal/common"
	"javboss/internal/common/logging"
	dbpkg "javboss/internal/db"
	"javboss/internal/jav"
	"javboss/internal/manager"
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

func getJavStudio(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	directoryIDs := parseDirectoryIDs(c.Query("directory_ids"))
	item, err := dbpkg.GetJavStudioSummary(c.Request.Context(), id, directoryIDs)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "studio not found"})
			return
		}
		logging.Error("get jav studio id=%d: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	enrichJavStudioSummary(c.Request.Context(), item, javCoverDir(), directoryIDs)
	c.JSON(http.StatusOK, item)
}

func listJavSeries(c *gin.Context) {
	limit := queryInt(c, "limit", 100)
	offset := queryInt(c, "offset", 0)
	search := strings.TrimSpace(c.Query("search"))
	directoryIDs := parseDirectoryIDs(c.Query("directory_ids"))

	items, total, err := dbpkg.ListJavSeries(c.Request.Context(), search, limit, offset, directoryIDs)
	if err != nil {
		logging.Error("list jav series: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	enrichJavSeriesSummaries(c.Request.Context(), items, directoryIDs)

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
	})
}

func getJavSeries(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	directoryIDs := parseDirectoryIDs(c.Query("directory_ids"))
	item, err := dbpkg.GetJavSeriesSummary(c.Request.Context(), id, directoryIDs)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "series not found"})
			return
		}
		logging.Error("get jav series id=%d: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	enrichJavSeriesSummary(c.Request.Context(), item, javCoverDir(), directoryIDs)
	c.JSON(http.StatusOK, item)
}

func getJavSeriesJavDBURL(c *gin.Context) {
	seriesID, err := strconv.ParseInt(strings.TrimSpace(c.Query("series_id")), 10, 64)
	if err != nil || seriesID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "series_id is required"})
		return
	}

	codes, err := dbpkg.ListSeriesCoverCodes(c.Request.Context(), seriesID, nil)
	if err != nil {
		logging.Error("list series codes id=%d: %v", seriesID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	var lastErr error
	for i, code := range codes {
		if i >= 3 {
			break
		}
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		seriesURL, err := jav.LookupSeriesURLByCode(code, jav.ProviderJavDB)
		if err == nil && strings.TrimSpace(seriesURL) != "" {
			c.JSON(http.StatusOK, gin.H{"url": seriesURL})
			return
		}
		if err != nil && !errors.Is(err, jav.ResourceNotFonud) {
			lastErr = err
			logging.Error("lookup javdb series url series_id=%d code=%s: %v", seriesID, code, err)
		}
	}
	if lastErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "javdb series url not found"})
}

func getJavStudioJavDBURL(c *gin.Context) {
	studioID, err := strconv.ParseInt(strings.TrimSpace(c.Query("studio_id")), 10, 64)
	if err != nil || studioID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "studio_id is required"})
		return
	}

	codes, err := dbpkg.ListStudioCoverCodes(c.Request.Context(), studioID, nil)
	if err != nil {
		logging.Error("list studio codes id=%d: %v", studioID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	var lastErr error
	for i, code := range codes {
		if i >= 3 {
			break
		}
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		studioURL, err := jav.LookupStudioURLByCode(code, jav.ProviderJavDB)
		if err == nil && strings.TrimSpace(studioURL) != "" {
			c.JSON(http.StatusOK, gin.H{"url": studioURL})
			return
		}
		if err != nil && !errors.Is(err, jav.ResourceNotFonud) {
			lastErr = err
			logging.Error("lookup javdb studio url studio_id=%d code=%s: %v", studioID, code, err)
		}
	}
	if lastErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "javdb studio url not found"})
}

func enrichJavStudioSummaries(ctx context.Context, items []dbpkg.JavStudioSummary, directoryIDs []int64) {
	coverDir := javCoverDir()
	for i := range items {
		enrichJavStudioSummary(ctx, &items[i], coverDir, directoryIDs)
	}
}

func enrichJavSeriesSummaries(ctx context.Context, items []dbpkg.JavSeriesSummary, directoryIDs []int64) {
	coverDir := javCoverDir()
	for i := range items {
		enrichJavSeriesSummary(ctx, &items[i], coverDir, directoryIDs)
	}
}

func javCoverDir() string {
	cfg := common.AppConfig
	if cfg != nil {
		return cfg.JavCoverDir
	}
	return ""
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

func enrichJavSeriesSummary(ctx context.Context, item *dbpkg.JavSeriesSummary, coverDir string, directoryIDs []int64) {
	item.Name = strings.TrimSpace(item.Name)
	item.SampleCode = strings.TrimSpace(item.SampleCode)

	if coverDir == "" {
		return
	}
	if _, ok := manager.FindCoverPath(coverDir, item.SampleCode); ok {
		return
	}
	codes, err := dbpkg.ListSeriesCoverCodes(ctx, item.ID, directoryIDs)
	if err != nil {
		logging.Error("list series cover codes id=%d: %v", item.ID, err)
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
