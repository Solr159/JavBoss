package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"pornboss/internal/common/logging"
	dbpkg "pornboss/internal/db"
)

type videoTagRequest struct {
	VideoIDs []int64 `json:"video_ids"`
	TagID    int64   `json:"tag_id"`
}

type videoTagsReplaceRequest struct {
	VideoIDs []int64 `json:"video_ids"`
	TagIDs   []int64 `json:"tag_ids"`
}

type tagsBatchDeleteRequest struct {
	TagIDs []int64 `json:"tag_ids"`
}

func listTags(c *gin.Context) {
	tags, err := dbpkg.ListTags(
		c.Request.Context(),
		parseDirectoryIDs(c.Query("directory_ids")),
		queryBool(c, "hide_jav", true),
	)
	if err != nil {
		logging.Error("list tags error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if tags == nil {
		tags = []dbpkg.TagCount{}
	}
	c.JSON(http.StatusOK, tags)
}

func createTag(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	tag, err := dbpkg.CreateTag(c.Request.Context(), req.Name)
	if err != nil {
		logging.Error("create tag error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, dbpkg.TagCount{ID: tag.ID, Name: tag.Name, Count: 0})
}

func renameTag(c *gin.Context) {
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

	if err := dbpkg.RenameTag(c.Request.Context(), id, req.Name); err != nil {
		logging.Error("rename tag error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func deleteTag(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := dbpkg.DeleteTag(c.Request.Context(), id); err != nil {
		logging.Error("delete tag error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func addTagsToVideos(c *gin.Context) {
	var req videoTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if req.TagID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag_id must be positive"})
		return
	}

	if err := dbpkg.AddTagToVideos(c.Request.Context(), req.TagID, req.VideoIDs); err != nil {
		logging.Error("add tag error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func removeTagsFromVideos(c *gin.Context) {
	var req videoTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if req.TagID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag_id must be positive"})
		return
	}

	if err := dbpkg.RemoveTagFromVideos(c.Request.Context(), req.TagID, req.VideoIDs); err != nil {
		logging.Error("remove tag error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func replaceTagsForVideos(c *gin.Context) {
	var req videoTagsReplaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if len(req.VideoIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "video_ids required"})
		return
	}
	if err := dbpkg.ReplaceTagsForVideos(c.Request.Context(), req.VideoIDs, req.TagIDs); err != nil {
		logging.Error("replace tags error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func deleteTagsBatch(c *gin.Context) {
	var req tagsBatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if len(req.TagIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag_ids required"})
		return
	}
	if err := dbpkg.DeleteTags(c.Request.Context(), req.TagIDs); err != nil {
		logging.Error("delete tags error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
