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
	favoriteGroupID := int64(0)
	if favoriteGroupParam := strings.TrimSpace(c.Query("favorite_group_id")); favoriteGroupParam != "" {
		parsed, err := strconv.ParseInt(favoriteGroupParam, 10, 64)
		if err != nil || parsed <= 0 {
			respondLocalizedError(c, http.StatusBadRequest, "收藏夹 ID 无效", "Invalid favorite group ID")
			return
		}
		favoriteGroupID = parsed
	}

	items, total, err := dbpkg.ListJavStudios(c.Request.Context(), search, limit, offset, directoryIDs, favoriteGroupID)
	if err != nil {
		logging.Error("list jav studios: %v", err)
		respondLocalizedError(c, http.StatusInternalServerError, "加载片商列表失败", "Failed to load studios")
		return
	}

	enrichJavStudioSummaries(c.Request.Context(), items, directoryIDs)

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
	})
}

func listJavStudioOptions(c *gin.Context) {
	limit := queryInt(c, "limit", 100)
	offset := queryInt(c, "offset", 0)
	search := strings.TrimSpace(c.Query("search"))

	items, total, err := dbpkg.ListJavStudioOptions(c.Request.Context(), search, limit, offset)
	if err != nil {
		logging.Error("list jav studio options: %v", err)
		respondLocalizedError(c, http.StatusInternalServerError, "加载片商选项失败", "Failed to load studio options")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
	})
}

func mergeJavStudios(c *gin.Context) {
	var req struct {
		CanonicalID int64   `json:"canonical_id"`
		MergeIDs    []int64 `json:"merge_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "合并片商请求无效", "Invalid studio merge request")
		return
	}
	if req.CanonicalID <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "主片商 ID 不能为空", "Canonical studio ID is required")
		return
	}
	if len(req.MergeIDs) == 0 {
		respondLocalizedError(c, http.StatusBadRequest, "待合并片商 ID 不能为空", "Studio IDs to merge are required")
		return
	}

	directoryIDs := parseDirectoryIDs(c.Query("directory_ids"))
	item, err := dbpkg.MergeJavStudios(c.Request.Context(), req.CanonicalID, req.MergeIDs, directoryIDs)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondLocalizedError(c, http.StatusNotFound, "片商不存在", "Studio was not found")
			return
		}
		logging.Error("merge jav studios canonical=%d merge=%v: %v", req.CanonicalID, req.MergeIDs, err)
		respondLocalizedError(c, http.StatusBadRequest, "合并片商失败，请检查所选片商是否有效", "Failed to merge studios; check the selected studios")
		return
	}
	enrichJavStudioSummary(c.Request.Context(), item, javCoverDir(), directoryIDs)
	c.JSON(http.StatusOK, item)
}

func updateJavStudio(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "片商 ID 无效", "Invalid studio ID")
		return
	}
	var req struct {
		Name    string   `json:"name"`
		Aliases []string `json:"aliases"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "修改片商信息请求无效", "Invalid studio update request")
		return
	}

	directoryIDs := parseDirectoryIDs(c.Query("directory_ids"))
	item, err := dbpkg.UpdateJavStudioProfile(c.Request.Context(), id, dbpkg.JavStudioUpdateInput{
		Name:    req.Name,
		Aliases: req.Aliases,
	}, directoryIDs)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondLocalizedError(c, http.StatusNotFound, "片商不存在", "Studio was not found")
			return
		}
		logging.Error("update jav studio id=%d: %v", id, err)
		respondLocalizedError(c, http.StatusBadRequest, "保存片商信息失败", "Failed to save studio information")
		return
	}
	enrichJavStudioSummary(c.Request.Context(), item, javCoverDir(), directoryIDs)
	c.JSON(http.StatusOK, item)
}

func getJavStudio(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "片商 ID 无效", "Invalid studio ID")
		return
	}

	directoryIDs := parseDirectoryIDs(c.Query("directory_ids"))
	item, err := dbpkg.GetJavStudioSummary(c.Request.Context(), id, directoryIDs)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondLocalizedError(c, http.StatusNotFound, "片商不存在", "Studio was not found")
			return
		}
		logging.Error("get jav studio id=%d: %v", id, err)
		respondLocalizedError(c, http.StatusInternalServerError, "加载片商信息失败", "Failed to load studio information")
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
	favoriteGroupID := int64(0)
	if favoriteGroupParam := strings.TrimSpace(c.Query("favorite_group_id")); favoriteGroupParam != "" {
		parsed, err := strconv.ParseInt(favoriteGroupParam, 10, 64)
		if err != nil || parsed <= 0 {
			respondLocalizedError(c, http.StatusBadRequest, "收藏夹 ID 无效", "Invalid favorite group ID")
			return
		}
		favoriteGroupID = parsed
	}

	items, total, err := dbpkg.ListJavSeries(c.Request.Context(), search, limit, offset, directoryIDs, favoriteGroupID)
	if err != nil {
		logging.Error("list jav series: %v", err)
		respondLocalizedError(c, http.StatusInternalServerError, "加载系列列表失败", "Failed to load series")
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
		respondLocalizedError(c, http.StatusBadRequest, "系列 ID 无效", "Invalid series ID")
		return
	}

	directoryIDs := parseDirectoryIDs(c.Query("directory_ids"))
	item, err := dbpkg.GetJavSeriesSummary(c.Request.Context(), id, directoryIDs)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondLocalizedError(c, http.StatusNotFound, "系列不存在", "Series was not found")
			return
		}
		logging.Error("get jav series id=%d: %v", id, err)
		respondLocalizedError(c, http.StatusInternalServerError, "加载系列信息失败", "Failed to load series information")
		return
	}

	enrichJavSeriesSummary(c.Request.Context(), item, javCoverDir(), directoryIDs)
	c.JSON(http.StatusOK, item)
}

func getJavSeriesJavDBURL(c *gin.Context) {
	seriesID, err := strconv.ParseInt(strings.TrimSpace(c.Query("series_id")), 10, 64)
	if err != nil || seriesID <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "系列 ID 不能为空", "Series ID is required")
		return
	}

	codes, err := dbpkg.ListSeriesCoverCodes(c.Request.Context(), seriesID, nil)
	if err != nil {
		logging.Error("list series codes id=%d: %v", seriesID, err)
		respondLocalizedError(c, http.StatusInternalServerError, "读取系列作品番号失败", "Failed to load series JAV codes")
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
		respondLocalizedError(c, http.StatusInternalServerError, "查询 JavDB 系列页面失败", "Failed to look up the JavDB series page")
		return
	}
	respondLocalizedError(c, http.StatusNotFound, "未找到对应的 JavDB 系列页面", "JavDB series page was not found")
}

func getJavStudioJavDBURL(c *gin.Context) {
	studioID, err := strconv.ParseInt(strings.TrimSpace(c.Query("studio_id")), 10, 64)
	if err != nil || studioID <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "片商 ID 不能为空", "Studio ID is required")
		return
	}

	codes, err := dbpkg.ListStudioCoverCodes(c.Request.Context(), studioID, nil)
	if err != nil {
		logging.Error("list studio codes id=%d: %v", studioID, err)
		respondLocalizedError(c, http.StatusInternalServerError, "读取片商作品番号失败", "Failed to load studio JAV codes")
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
		respondLocalizedError(c, http.StatusInternalServerError, "查询 JavDB 片商页面失败", "Failed to look up the JavDB studio page")
		return
	}
	respondLocalizedError(c, http.StatusNotFound, "未找到对应的 JavDB 片商页面", "JavDB studio page was not found")
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
