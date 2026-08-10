package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"javboss/internal/common/logging"
	dbpkg "javboss/internal/db"
	"javboss/internal/models"
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
		queryBool(c, "hide_jav", false),
	)
	if err != nil {
		logging.Error("list tags error: %v", err)
		respondLocalizedError(c, http.StatusInternalServerError, "加载视频标签失败", "Failed to load video tags")
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
		respondLocalizedError(c, http.StatusBadRequest, "创建标签请求无效", "Invalid tag creation request")
		return
	}

	tag, err := dbpkg.CreateTag(c.Request.Context(), req.Name)
	if err != nil {
		logging.Error("create tag error: %v", err)
		respondLocalizedError(c, http.StatusBadRequest, "创建标签失败，标签名称可能为空或已存在", "Failed to create tag; the name may be empty or already exist")
		return
	}
	c.JSON(http.StatusCreated, dbpkg.TagCount{ID: tag.ID, Name: tag.Name, Count: 0})
}

func listTagCategories(c *gin.Context) {
	categories, err := dbpkg.ListTagCategories(c.Request.Context())
	if err != nil {
		logging.Error("list tag categories error: %v", err)
		respondLocalizedError(c, http.StatusInternalServerError, "加载视频标签分类失败", "Failed to load video tag categories")
		return
	}
	if categories == nil {
		categories = []models.TagCategory{}
	}
	c.JSON(http.StatusOK, categories)
}

func createTagCategory(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "创建标签分类请求无效", "Invalid tag category creation request")
		return
	}
	category, err := dbpkg.CreateTagCategory(c.Request.Context(), req.Name)
	if err != nil {
		logging.Error("create tag category error: %v", err)
		respondLocalizedError(c, http.StatusBadRequest, "创建标签分类失败，名称可能为空或已存在", "Failed to create tag category; the name may be empty or already exist")
		return
	}
	c.JSON(http.StatusCreated, category)
}

func reorderTagCategories(c *gin.Context) {
	var req struct {
		CategoryIDs []int64 `json:"category_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "调整标签分类顺序请求无效", "Invalid tag category reorder request")
		return
	}
	if err := dbpkg.ReorderTagCategories(c.Request.Context(), req.CategoryIDs); err != nil {
		logging.Error("reorder tag categories error: %v", err)
		respondLocalizedError(c, http.StatusBadRequest, "调整标签分类顺序失败", "Failed to reorder tag categories")
		return
	}
	c.Status(http.StatusNoContent)
}

func renameTagCategory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "标签分类 ID 无效", "Invalid tag category ID")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "修改标签分类请求无效", "Invalid tag category update request")
		return
	}
	if err := dbpkg.RenameTagCategory(c.Request.Context(), id, req.Name); err != nil {
		logging.Error("rename tag category error: %v", err)
		respondLocalizedError(c, http.StatusBadRequest, "修改标签分类失败，名称可能为空或已存在", "Failed to rename tag category; the name may be empty or already exist")
		return
	}
	c.Status(http.StatusNoContent)
}

func deleteTagCategory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "标签分类 ID 无效", "Invalid tag category ID")
		return
	}
	if err := dbpkg.DeleteTagCategory(c.Request.Context(), id); err != nil {
		logging.Error("delete tag category error: %v", err)
		respondLocalizedError(c, http.StatusBadRequest, "删除标签分类失败", "Failed to delete tag category")
		return
	}
	c.Status(http.StatusNoContent)
}

func assignTagsCategory(c *gin.Context) {
	var req struct {
		TagIDs     []int64 `json:"tag_ids"`
		CategoryID *int64  `json:"category_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "批量调整标签分类请求无效", "Invalid batch tag category request")
		return
	}
	if err := dbpkg.AssignTagsCategory(c.Request.Context(), req.TagIDs, req.CategoryID); err != nil {
		logging.Error("assign tag category error: %v", err)
		respondLocalizedError(c, http.StatusBadRequest, "批量调整标签分类失败", "Failed to assign tag categories")
		return
	}
	c.Status(http.StatusNoContent)
}

func renameTag(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "标签 ID 无效", "Invalid tag ID")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "重命名标签请求无效", "Invalid tag rename request")
		return
	}

	if err := dbpkg.RenameTag(c.Request.Context(), id, req.Name); err != nil {
		logging.Error("rename tag error: %v", err)
		respondLocalizedError(c, http.StatusBadRequest, "重命名标签失败，标签名称可能为空或已存在", "Failed to rename tag; the name may be empty or already exist")
		return
	}
	c.Status(http.StatusNoContent)
}

func deleteTag(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "标签 ID 无效", "Invalid tag ID")
		return
	}

	if err := dbpkg.DeleteTag(c.Request.Context(), id); err != nil {
		logging.Error("delete tag error: %v", err)
		respondLocalizedError(c, http.StatusBadRequest, "删除标签失败", "Failed to delete tag")
		return
	}
	c.Status(http.StatusNoContent)
}

func addTagsToVideos(c *gin.Context) {
	var req videoTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "添加视频标签请求无效", "Invalid add-video-tags request")
		return
	}

	if req.TagID <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "标签 ID 无效", "Invalid tag ID")
		return
	}

	if err := dbpkg.AddTagToVideos(c.Request.Context(), req.TagID, req.VideoIDs); err != nil {
		logging.Error("add tag error: %v", err)
		respondLocalizedError(c, http.StatusBadRequest, "添加视频标签失败", "Failed to add video tags")
		return
	}
	c.Status(http.StatusNoContent)
}

func removeTagsFromVideos(c *gin.Context) {
	var req videoTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "移除视频标签请求无效", "Invalid remove-video-tags request")
		return
	}

	if req.TagID <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "标签 ID 无效", "Invalid tag ID")
		return
	}

	if err := dbpkg.RemoveTagFromVideos(c.Request.Context(), req.TagID, req.VideoIDs); err != nil {
		logging.Error("remove tag error: %v", err)
		respondLocalizedError(c, http.StatusBadRequest, "移除视频标签失败", "Failed to remove video tags")
		return
	}
	c.Status(http.StatusNoContent)
}

func replaceTagsForVideos(c *gin.Context) {
	var req videoTagsReplaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "更新视频标签请求无效", "Invalid update-video-tags request")
		return
	}
	if len(req.VideoIDs) == 0 {
		respondLocalizedError(c, http.StatusBadRequest, "视频 ID 不能为空", "Video IDs are required")
		return
	}
	if err := dbpkg.ReplaceTagsForVideos(c.Request.Context(), req.VideoIDs, req.TagIDs); err != nil {
		logging.Error("replace tags error: %v", err)
		respondLocalizedError(c, http.StatusBadRequest, "更新视频标签失败", "Failed to update video tags")
		return
	}
	c.Status(http.StatusNoContent)
}

func deleteTagsBatch(c *gin.Context) {
	var req tagsBatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "批量删除标签请求无效", "Invalid batch tag deletion request")
		return
	}
	if len(req.TagIDs) == 0 {
		respondLocalizedError(c, http.StatusBadRequest, "标签 ID 不能为空", "Tag IDs are required")
		return
	}
	if err := dbpkg.DeleteTags(c.Request.Context(), req.TagIDs); err != nil {
		logging.Error("delete tags error: %v", err)
		respondLocalizedError(c, http.StatusBadRequest, "批量删除标签失败", "Failed to delete tags")
		return
	}
	c.Status(http.StatusNoContent)
}
