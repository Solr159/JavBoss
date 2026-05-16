package server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"pornboss/internal/common/logging"
	dbpkg "pornboss/internal/db"
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

	items, total, err := dbpkg.SearchJav(c.Request.Context(), idolIDs, tagIDs, search, sort, limit, offset, seed, directoryIDs, studioID, seriesID)
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
