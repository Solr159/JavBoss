package api

import (
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"javboss/internal/common/logging"
	dbpkg "javboss/internal/db"
)

func parseVideoDirectoryFilter(c *gin.Context) ([]int64, string, bool) {
	rawID := c.Query("directory_id")
	subdirectory := c.Query("subdirectory")
	if rawID == "" && subdirectory == "" {
		return nil, "", true
	}
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "目录 ID 无效", "Invalid directory ID")
		return nil, "", false
	}
	if subdirectory != "" {
		subdirectory = path.Clean(subdirectory)
		if path.IsAbs(subdirectory) || subdirectory == ".." || strings.HasPrefix(subdirectory, "../") || strings.ContainsRune(subdirectory, '\x00') {
			respondLocalizedError(c, http.StatusBadRequest, "子目录路径无效", "Invalid subdirectory path")
			return nil, "", false
		}
	}
	return []int64{id}, subdirectory, true
}

// GET /videos/directories returns indexed directories and their ancestors for
// the video filter, grouped by enabled library root (directory_id).
// directory_paths maps each root ID to its absolute path once; items reference it
// through directory_id and retain their relative path in path.
// Each item includes video_count (direct files only) and videos (up to twenty
// filenames, ordered by name). Ancestors without direct files have an empty list.
func listVideoDirectories(c *gin.Context) {
	items, err := dbpkg.Default().ListVideoDirectories(c.Request.Context())
	if err != nil {
		logging.Error("list video directories error: %v", err)
		respondLocalizedError(c, http.StatusInternalServerError, "加载子目录失败", "Failed to load subdirectories")
		return
	}
	directoryPaths := make(map[int64]string)
	for _, item := range items {
		directoryPaths[item.DirectoryID] = item.DirectoryPath
	}
	c.JSON(http.StatusOK, gin.H{"directory_paths": directoryPaths, "items": items})
}
