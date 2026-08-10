package server

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"javboss/internal/common/logging"
	dbpkg "javboss/internal/db"
	"javboss/internal/western"
)

const thePornDBTokenEnv = "JAVBOSS_THEPORNDB_TOKEN"

func searchVideoWesternMetadata(c *gin.Context) {
	id, ok := parsePositiveVideoID(c)
	if !ok {
		return
	}
	video, err := dbpkg.GetVideo(c.Request.Context(), id)
	if err != nil {
		logging.Error("load video for western scrape search: %v", err)
		respondLocalizedError(c, http.StatusInternalServerError, "读取视频失败", "Failed to load video")
		return
	}
	if video == nil {
		respondLocalizedError(c, http.StatusNotFound, "视频不存在", "Video does not exist")
		return
	}
	query := strings.TrimSpace(c.Query("q"))
	if query == "" {
		query = filepath.Base(filepath.FromSlash(video.Filename))
	}
	token := strings.TrimSpace(os.Getenv(thePornDBTokenEnv))
	if token == "" {
		respondLocalizedError(
			c,
			http.StatusPreconditionFailed,
			"请先设置 JAVBOSS_THEPORNDB_TOKEN 环境变量",
			"Set the JAVBOSS_THEPORNDB_TOKEN environment variable first",
		)
		return
	}
	var hash string
	if location, locationErr := dbpkg.GetPrimaryVideoLocation(c.Request.Context(), id); locationErr == nil && location != nil {
		videoPath := filepath.Join(location.DirectoryRef.Path, filepath.FromSlash(location.RelativePath))
		hash, _ = western.OpenSubtitlesHash(videoPath)
	}
	items, err := western.SearchThePornDBWithOptions(c.Request.Context(), token, western.SearchOptions{
		Query: query,
		Hash:  hash,
	})
	if err != nil {
		if errors.Is(err, western.ErrThePornDBUnauthorized) {
			respondLocalizedError(c, http.StatusPreconditionFailed, "ThePornDB Token 无效，请重新生成", "ThePornDB token is invalid; generate a new token")
			return
		}
		logging.Error("search ThePornDB western metadata query=%q: %v", query, err)
		respondLocalizedError(c, http.StatusBadGateway, "ThePornDB 搜索失败", "ThePornDB search failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "query": query})
}

func saveVideoWesternMetadata(c *gin.Context) {
	id, ok := parsePositiveVideoID(c)
	if !ok {
		return
	}
	var req western.Metadata
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "欧美元数据无效", "Invalid Western metadata")
		return
	}
	req.Source = "theporndb"
	metadata, err := dbpkg.SaveWesternMetadata(c.Request.Context(), id, req)
	if err != nil {
		logging.Error("save western metadata video=%d: %v", id, err)
		respondLocalizedError(c, http.StatusBadRequest, "保存欧美元数据失败", "Failed to save Western metadata")
		return
	}
	if location, locationErr := dbpkg.GetPrimaryVideoLocation(c.Request.Context(), id); locationErr == nil && location != nil {
		videoPath := filepath.Join(location.DirectoryRef.Path, filepath.FromSlash(location.RelativePath))
		if nfoErr := western.WriteNFO(videoPath, req); nfoErr != nil {
			logging.Error("write western NFO video=%d: %v", id, nfoErr)
		}
	}
	c.JSON(http.StatusOK, metadata)
}

func deleteVideoWesternMetadata(c *gin.Context) {
	id, ok := parsePositiveVideoID(c)
	if !ok {
		return
	}
	if err := dbpkg.DeleteWesternMetadata(c.Request.Context(), id); err != nil {
		logging.Error("delete western metadata video=%d: %v", id, err)
		respondLocalizedError(c, http.StatusInternalServerError, "删除欧美元数据失败", "Failed to delete Western metadata")
		return
	}
	c.Status(http.StatusNoContent)
}
