package server

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"javboss/internal/common/logging"
	dbpkg "javboss/internal/db"
)

func listJavFavoriteGroupsFor(entityType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		groups, err := dbpkg.ListJavFavoriteGroups(c.Request.Context(), entityType, parseDirectoryIDs(c.Query("directory_ids")))
		if err != nil {
			logging.Error("list jav favorite groups type=%s: %v", entityType, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		if groups == nil {
			groups = []dbpkg.JavFavoriteGroupSummary{}
		}
		c.JSON(http.StatusOK, gin.H{"items": groups})
	}
}

func createJavFavoriteGroupFor(entityType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name string `json:"name"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return
		}

		group, err := dbpkg.CreateJavFavoriteGroup(c.Request.Context(), entityType, req.Name)
		if err != nil {
			logging.Error("create jav favorite group type=%s: %v", entityType, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, dbpkg.JavFavoriteGroupSummary{
			ID:         group.ID,
			EntityType: group.EntityType,
			Name:       group.Name,
			SortOrder:  group.SortOrder,
			Count:      0,
		})
	}
}

func renameJavFavoriteGroupFor(entityType string) gin.HandlerFunc {
	return func(c *gin.Context) {
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
		if err := dbpkg.RenameJavFavoriteGroup(c.Request.Context(), entityType, id, req.Name); err != nil {
			logging.Error("rename jav favorite group type=%s id=%d: %v", entityType, id, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func deleteJavFavoriteGroupFor(entityType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		if err := dbpkg.DeleteJavFavoriteGroup(c.Request.Context(), entityType, id); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "favorite group not found"})
				return
			}
			logging.Error("delete jav favorite group type=%s id=%d: %v", entityType, id, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func reorderJavFavoriteGroupsFor(entityType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			GroupIDs []int64 `json:"group_ids"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return
		}
		if err := dbpkg.ReorderJavFavoriteGroups(c.Request.Context(), entityType, req.GroupIDs); err != nil {
			logging.Error("reorder jav favorite groups type=%s: %v", entityType, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func listJavFavoriteGroupItemsFor(entityType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		items, err := dbpkg.ListJavFavoriteGroupItems(
			c.Request.Context(),
			entityType,
			id,
			parseDirectoryIDs(c.Query("directory_ids")),
		)
		if err != nil {
			logging.Error("list jav favorite group items type=%s id=%d: %v", entityType, id, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		if items == nil {
			items = []dbpkg.JavFavoriteItemSummary{}
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	}
}

func reorderJavFavoriteGroupItemsFor(entityType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		var req struct {
			EntityIDs []int64 `json:"entity_ids"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return
		}
		if err := dbpkg.ReorderJavFavoriteGroupItems(c.Request.Context(), entityType, id, req.EntityIDs); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "favorite group not found"})
				return
			}
			logging.Error("reorder jav favorite group items type=%s id=%d: %v", entityType, id, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func removeJavFavoriteGroupItemsFor(entityType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		var req struct {
			EntityIDs []int64 `json:"entity_ids"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return
		}
		if err := dbpkg.RemoveJavFavoriteGroupItems(c.Request.Context(), entityType, id, req.EntityIDs); err != nil {
			logging.Error("remove jav favorite group items type=%s id=%d: %v", entityType, id, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func listJavFavoriteGroupIDsFor(entityType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		ids, err := dbpkg.ListJavFavoriteGroupIDs(c.Request.Context(), entityType, id)
		if err != nil {
			logging.Error("list jav favorite group ids type=%s id=%d: %v", entityType, id, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		if ids == nil {
			ids = []int64{}
		}
		c.JSON(http.StatusOK, gin.H{"selected_group_ids": ids})
	}
}

func replaceJavFavoriteGroupsFor(entityType string) gin.HandlerFunc {
	return func(c *gin.Context) {
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

		if err := dbpkg.ReplaceJavFavoriteGroups(c.Request.Context(), entityType, id, req.GroupIDs); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "entity not found"})
				return
			}
			logging.Error("replace jav favorite groups type=%s id=%d: %v", entityType, id, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}
