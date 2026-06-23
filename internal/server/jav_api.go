package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"javboss/internal/common"
	"javboss/internal/common/logging"
	dbpkg "javboss/internal/db"
	"javboss/internal/jav"
	"javboss/internal/manager"
)

func searchJav(c *gin.Context) {
	limit := queryInt(c, "limit", 100)
	offset := queryInt(c, "offset", 0)
	idolIDs := parseInt64CSV(c.Query("idol_ids"))
	tagIDs := parseInt64CSV(c.Query("tag_ids"))
	directoryIDs := parseDirectoryIDs(c.Query("directory_ids"))
	studioID := int64(0)
	if studioParam := strings.TrimSpace(c.Query("studio_id")); studioParam != "" {
		parsed, err := strconv.ParseInt(studioParam, 10, 64)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid studio_id"})
			return
		}
		studioID = parsed
	}
	seriesID := int64(0)
	if seriesParam := strings.TrimSpace(c.Query("series_id")); seriesParam != "" {
		parsed, err := strconv.ParseInt(seriesParam, 10, 64)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid series_id"})
			return
		}
		seriesID = parsed
	}
	search := strings.TrimSpace(c.Query("search"))
	sort := strings.TrimSpace(c.Query("sort"))
	soloOnly := queryBool(c, "solo", false)
	favoriteGroupID := int64(0)
	if favoriteGroupParam := strings.TrimSpace(c.Query("favorite_group_id")); favoriteGroupParam != "" {
		parsed, err := strconv.ParseInt(favoriteGroupParam, 10, 64)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid favorite_group_id"})
			return
		}
		favoriteGroupID = parsed
	}
	seedParam := strings.TrimSpace(c.Query("seed"))
	var seed *int64
	if seedParam != "" {
		parsed, err := strconv.ParseInt(seedParam, 10, 64)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid seed"})
			return
		}
		seed = &parsed
	}

	items, total, err := dbpkg.SearchJav(c.Request.Context(), idolIDs, tagIDs, search, sort, limit, offset, seed, directoryIDs, studioID, seriesID, boolInt64(soloOnly), favoriteGroupID)
	if err != nil {
		logging.Error("SearchJav: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
	})
}

func boolInt64(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

func getJavJavDBURL(c *gin.Context) {
	code := strings.TrimSpace(c.Query("code"))
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code is required"})
		return
	}

	javdbURL, err := jav.LookupJavDBURLByCode(code)
	if err != nil {
		if errors.Is(err, jav.ResourceNotFonud) {
			c.JSON(http.StatusNotFound, gin.H{"error": "javdb url not found"})
			return
		}
		logging.Error("lookup javdb url code=%s: %v", code, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": javdbURL})
}

func listJavTags(c *gin.Context) {
	tags, err := dbpkg.ListJavTags(c.Request.Context(), parseDirectoryIDs(c.Query("directory_ids")))
	if err != nil {
		logging.Error("list jav tags error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if tags == nil {
		tags = []dbpkg.JavTagCount{}
	}
	c.JSON(http.StatusOK, tags)
}

type javItemUpdateRequest struct {
	Title       *string  `json:"title"`
	CoverURL    *string  `json:"cover_url"`
	TagIDs      *[]int64 `json:"tag_ids"`
	IdolIDs     *[]int64 `json:"idol_ids"`
	StudioID    *int64   `json:"studio_id"`
	SeriesID    *int64   `json:"series_id"`
	ReleaseDate *string  `json:"release_date"`
	DurationMin *int     `json:"duration_min"`
}

func updateJavItem(c *gin.Context) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req javItemUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	var releaseUnix *int64
	if req.ReleaseDate != nil {
		parsed, err := parseJavEditReleaseUnix(*req.ReleaseDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		releaseUnix = &parsed
	}

	if req.CoverURL != nil {
		coverURL := strings.TrimSpace(*req.CoverURL)
		if coverURL != "" {
			cfg := common.AppConfig
			if cfg == nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "config not loaded"})
				return
			}
			item, err := dbpkg.GetJav(c.Request.Context(), id, parseDirectoryIDs(c.Query("directory_ids")))
			if err != nil {
				logging.Error("get jav for cover update error: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
			defer cancel()
			if err := manager.DownloadCoverFromURL(ctx, cfg.JavCoverDir, item.Code, coverURL); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
	}

	updated, err := dbpkg.UpdateJav(c.Request.Context(), id, dbpkg.JavUpdateInput{
		Title:       req.Title,
		StudioID:    req.StudioID,
		SeriesID:    req.SeriesID,
		IdolIDs:     req.IdolIDs,
		UserTagIDs:  req.TagIDs,
		ReleaseUnix: releaseUnix,
		DurationMin: req.DurationMin,
	}, parseDirectoryIDs(c.Query("directory_ids")))
	if err != nil {
		logging.Error("update jav item error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func parseJavEditReleaseUnix(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return 0, errors.New("release_date must be YYYY-MM-DD")
	}
	return t.Unix(), nil
}

func createJavTag(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	tag, err := dbpkg.CreateJavTag(c.Request.Context(), req.Name)
	if err != nil {
		logging.Error("create jav tag error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, dbpkg.JavTagCount{
		ID:       tag.ID,
		Name:     tag.Name,
		Provider: tag.Provider,
		Count:    0,
	})
}

func renameJavTag(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
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

	if err := dbpkg.RenameJavTag(c.Request.Context(), id, req.Name); err != nil {
		logging.Error("rename jav tag error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func deleteJavTag(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := dbpkg.DeleteJavTag(c.Request.Context(), id); err != nil {
		logging.Error("delete jav tag error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

type javTagRequest struct {
	JavIDs []int64 `json:"jav_ids"`
	TagID  int64   `json:"tag_id"`
}

type javTagsReplaceRequest struct {
	JavIDs []int64 `json:"jav_ids"`
	TagIDs []int64 `json:"tag_ids"`
}

type javTagsBatchDeleteRequest struct {
	TagIDs []int64 `json:"tag_ids"`
}

func addJavTagsToItems(c *gin.Context) {
	var req javTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if req.TagID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag_id must be positive"})
		return
	}
	if err := dbpkg.AddJavTagToJavs(c.Request.Context(), req.TagID, req.JavIDs); err != nil {
		logging.Error("add jav tag error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func removeJavTagsFromItems(c *gin.Context) {
	var req javTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if req.TagID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag_id must be positive"})
		return
	}
	if err := dbpkg.RemoveJavTagFromJavs(c.Request.Context(), req.TagID, req.JavIDs); err != nil {
		logging.Error("remove jav tag error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func replaceJavTagsForItems(c *gin.Context) {
	var req javTagsReplaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if len(req.JavIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "jav_ids required"})
		return
	}
	if err := dbpkg.ReplaceJavUserTags(c.Request.Context(), req.JavIDs, req.TagIDs); err != nil {
		logging.Error("replace jav tags error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func deleteJavTagsBatch(c *gin.Context) {
	var req javTagsBatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if len(req.TagIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag_ids required"})
		return
	}
	if err := dbpkg.DeleteJavTags(c.Request.Context(), req.TagIDs); err != nil {
		logging.Error("delete jav tags error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
