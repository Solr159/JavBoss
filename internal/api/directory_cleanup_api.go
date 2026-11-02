package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"javboss/internal/common/logging"
	dbpkg "javboss/internal/db"
	"javboss/internal/directory"
	"javboss/internal/models"
)

func cleanupDirectory(c *gin.Context) *models.Directory {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "目录 ID 无效", "Invalid directory ID")
		return nil
	}
	dir, err := dbpkg.Default().GetDirectory(c.Request.Context(), id)
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "读取目录失败", "Failed to load directory")
		return nil
	}
	if dir == nil || dir.IsDelete {
		respondLocalizedError(c, http.StatusNotFound, "目录不存在", "Directory does not exist")
		return nil
	}
	if dir.Missing {
		respondLocalizedError(c, http.StatusConflict, "目录当前不可用", "The directory is currently unavailable")
		return nil
	}
	return dir
}

// POST /directories/:id/cleanup/preview accepts CleanupOptions category flags.
// include_system controls system junk. Paths in the response are relative to root_path.
func previewDirectoryCleanup(c *gin.Context) {
	var req directory.CleanupOptions
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "清理扫描请求无效", "Invalid cleanup preview request")
		return
	}
	dir := cleanupDirectory(c)
	if dir == nil {
		return
	}
	preview, err := directory.PreviewDirectoryCleanup(c.Request.Context(), *dir, req)
	if err != nil {
		respondCleanupError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, preview)
}

// POST /directories/:id/cleanup accepts root_path and selected files from preview.
// Only supported junk files whose size and modification time still match are deleted.
func cleanDirectoryFiles(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8<<20)
	var req struct {
		RootPath string                  `json:"root_path"`
		Files    []directory.CleanupFile `json:"files"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "清理请求无效或过大", "Invalid or oversized cleanup request")
		return
	}
	dir := cleanupDirectory(c)
	if dir == nil {
		return
	}
	result, err := directory.CleanDirectoryFiles(c.Request.Context(), *dir, req.RootPath, req.Files)
	if err != nil {
		respondCleanupError(c, err)
		return
	}
	logging.Info("directory cleanup complete id=%d deleted=%d bytes=%d failed=%d", dir.ID, len(result.Deleted), result.DeletedBytes, len(result.Issues))
	c.JSON(http.StatusOK, result)
}

func respondCleanupError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, directory.ErrInvalidCleanupRequest):
		respondLocalizedError(c, http.StatusBadRequest, "清理列表无效或目录已变化，请重新扫描", "Invalid cleanup selection or directory changed; scan again")
	case errors.Is(err, directory.ErrDirectoryWorkInProgress):
		respondLocalizedError(c, http.StatusConflict, "目录正在执行其他任务，请稍后重试", "The directory is busy; please try again later")
	default:
		logging.Error("directory cleanup failed: %v", err)
		respondLocalizedError(c, http.StatusInternalServerError, "清理失败，请检查目录是否可访问后重试", "Cleanup failed; check that the directory is accessible and try again")
	}
}
