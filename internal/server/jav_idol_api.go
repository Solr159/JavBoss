package server

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"pornboss/internal/common"
	"pornboss/internal/common/logging"
	dbpkg "pornboss/internal/db"
	"pornboss/internal/jav"
	"pornboss/internal/manager"
)

func listJavIdols(c *gin.Context) {
	limit := queryInt(c, "limit", 100)
	offset := queryInt(c, "offset", 0)
	search := strings.TrimSpace(c.Query("search"))
	sort := strings.TrimSpace(c.Query("sort"))
	directoryIDs := parseDirectoryIDs(c.Query("directory_ids"))
	favoriteGroupID := int64(0)
	if favoriteGroupParam := strings.TrimSpace(c.Query("favorite_group_id")); favoriteGroupParam != "" {
		parsed, err := strconv.ParseInt(favoriteGroupParam, 10, 64)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid favorite_group_id"})
			return
		}
		favoriteGroupID = parsed
	}

	items, total, err := dbpkg.ListJavIdols(c.Request.Context(), search, sort, limit, offset, directoryIDs, favoriteGroupID)
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

func listJavIdolFavoriteGroups(c *gin.Context) {
	groups, err := dbpkg.ListJavIdolFavoriteGroups(c.Request.Context(), parseDirectoryIDs(c.Query("directory_ids")))
	if err != nil {
		logging.Error("list jav idol favorite groups: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if groups == nil {
		groups = []dbpkg.JavIdolFavoriteGroupSummary{}
	}
	c.JSON(http.StatusOK, gin.H{"items": groups})
}

func createJavIdolFavoriteGroup(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	group, err := dbpkg.CreateJavIdolFavoriteGroup(c.Request.Context(), req.Name)
	if err != nil {
		logging.Error("create jav idol favorite group: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, dbpkg.JavIdolFavoriteGroupSummary{
		ID:        group.ID,
		Name:      group.Name,
		SortOrder: group.SortOrder,
		Count:     0,
	})
}

func renameJavIdolFavoriteGroup(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if err := dbpkg.RenameJavIdolFavoriteGroup(c.Request.Context(), id, req.Name); err != nil {
		logging.Error("rename jav idol favorite group id=%d: %v", id, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func deleteJavIdolFavoriteGroup(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := dbpkg.DeleteJavIdolFavoriteGroup(c.Request.Context(), id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "favorite group not found"})
			return
		}
		logging.Error("delete jav idol favorite group id=%d: %v", id, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func reorderJavIdolFavoriteGroups(c *gin.Context) {
	var req struct {
		GroupIDs []int64 `json:"group_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if err := dbpkg.ReorderJavIdolFavoriteGroups(c.Request.Context(), req.GroupIDs); err != nil {
		logging.Error("reorder jav idol favorite groups: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func listJavIdolFavoriteGroupIdols(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	items, err := dbpkg.ListJavIdolFavoriteGroupIdols(
		c.Request.Context(),
		id,
		parseDirectoryIDs(c.Query("directory_ids")),
	)
	if err != nil {
		logging.Error("list jav idol favorite group idols id=%d: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	enrichJavIdolSummaries(c.Request.Context(), items, parseDirectoryIDs(c.Query("directory_ids")))
	if items == nil {
		items = []dbpkg.JavIdolSummary{}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func reorderJavIdolFavoriteGroupIdols(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		IdolIDs []int64 `json:"idol_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if err := dbpkg.ReorderJavIdolFavoriteGroupIdols(c.Request.Context(), id, req.IdolIDs); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "favorite group not found"})
			return
		}
		logging.Error("reorder jav idol favorite group idols id=%d: %v", id, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func removeJavIdolFavoriteGroupIdols(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		IdolIDs []int64 `json:"idol_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if err := dbpkg.RemoveJavIdolFavoriteGroupIdols(c.Request.Context(), id, req.IdolIDs); err != nil {
		logging.Error("remove jav idol favorite group idols id=%d: %v", id, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func listJavIdolFavoriteGroupIDs(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	ids, err := dbpkg.ListJavIdolFavoriteGroupIDs(c.Request.Context(), id)
	if err != nil {
		logging.Error("list jav idol favorite group ids id=%d: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if ids == nil {
		ids = []int64{}
	}
	c.JSON(http.StatusOK, gin.H{"selected_group_ids": ids})
}

func replaceJavIdolFavoriteGroups(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		GroupIDs []int64 `json:"group_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if err := dbpkg.ReplaceJavIdolFavoriteGroups(c.Request.Context(), id, req.GroupIDs); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "idol not found"})
			return
		}
		logging.Error("replace jav idol favorite groups id=%d: %v", id, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
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

func listJavIdolCoverOptions(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	options, err := dbpkg.ListIdolCoverOptions(
		c.Request.Context(),
		id,
		parseDirectoryIDs(c.Query("directory_ids")),
	)
	if err != nil {
		logging.Error("list jav idol cover options id=%d: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if options == nil {
		options = []dbpkg.JavIdolCoverOption{}
	}
	c.JSON(http.StatusOK, gin.H{"items": options})
}

func updateJavIdolCover(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req struct {
		JavID    int64   `json:"jav_id"`
		CropLeft float64 `json:"crop_left"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if math.IsNaN(req.CropLeft) || math.IsInf(req.CropLeft, 0) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid crop_left"})
		return
	}

	item, err := dbpkg.UpdateJavIdolCoverSelection(
		c.Request.Context(),
		id,
		req.JavID,
		req.CropLeft,
		parseDirectoryIDs(c.Query("directory_ids")),
	)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "idol not found"})
			return
		}
		logging.Error("update jav idol cover id=%d: %v", id, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	items := []dbpkg.JavIdolSummary{*item}
	enrichJavIdolSummaries(c.Request.Context(), items, parseDirectoryIDs(c.Query("directory_ids")))
	c.JSON(http.StatusOK, items[0])
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

func getJavIdolJavDBURL(c *gin.Context) {
	code := strings.TrimSpace(c.Query("code"))
	name := strings.TrimSpace(c.Query("name"))
	if code == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code and name are required"})
		return
	}

	profileURL, err := jav.LookupActressURLByCodeAndName(code, name, jav.ProviderJavDB)
	if err != nil {
		if errors.Is(err, jav.ResourceNotFonud) {
			c.JSON(http.StatusNotFound, gin.H{"error": "javdb actress url not found"})
			return
		}
		logging.Error("lookup javdb actress url code=%s name=%s: %v", code, name, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": profileURL})
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
	item.CoverCode = strings.TrimSpace(item.CoverCode)

	if coverDir == "" {
		return
	}
	if item.CoverJavID != nil && item.CoverCode != "" {
		if common.CoverManager != nil && !common.CoverManager.Exists(item.CoverCode) {
			common.CoverManager.Enqueue(item.CoverCode)
		}
		return
	}
	if item.CoverCode != "" {
		if _, ok := manager.FindCoverPath(coverDir, item.CoverCode); ok {
			return
		}
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
		item.CoverCode = chosen
		if common.CoverManager != nil && !common.CoverManager.Exists(chosen) {
			common.CoverManager.Enqueue(chosen)
		}
	}
}
