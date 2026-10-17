package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// PATCH /jav/series/:id saves a Chinese name; name is the source used by the editor.
func updateJavSeriesTranslation(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, 400, "系列 ID 无效", "Invalid series ID")
		return
	}
	var req struct {
		Name   *string `json:"name"`
		ZhName *string `json:"zh_name"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 65536)
	if err := c.ShouldBindJSON(&req); err != nil || req.ZhName == nil || len(*req.ZhName) > 8192 {
		respondLocalizedError(c, 400, "系列译名无效或过长", "Invalid series translation or name too long")
		return
	}
	item, err := javSeriesDomain.Get(c.Request.Context(), id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respondLocalizedError(c, 404, "系列不存在", "Series was not found")
		return
	}
	if err != nil {
		respondLocalizedError(c, 500, "读取系列失败", "Failed to load series")
		return
	}
	if req.Name != nil && *req.Name != item.Source {
		respondLocalizedError(c, 409, "系列名称已更改，请重新打开编辑窗口", "The series name changed; reopen the editor")
		return
	}
	text := strings.TrimSpace(*req.ZhName)
	saved, err := javSeriesDomain.Save(c.Request.Context(), item, text)
	if err != nil {
		respondLocalizedError(c, 500, "保存系列译名失败", "Failed to save series translation")
		return
	}
	if !saved {
		respondLocalizedError(c, 409, "系列已更改，请重新打开编辑窗口", "The series changed; reopen the editor")
		return
	}
	item.Translation = text
	c.JSON(http.StatusOK, javSeriesDomain.response(item))
}
