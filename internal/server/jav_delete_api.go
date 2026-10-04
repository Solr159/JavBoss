package server

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"javboss/internal/common"
	"javboss/internal/common/logging"
	dbpkg "javboss/internal/db"
	"javboss/internal/models"
	"javboss/internal/util"
)

var errJavDeleteFilesUnavailable = errors.New("jav files are unavailable or unsafe")

// DELETE /jav/items/:id/videos removes video records, files, sidecars and screenshots.
// The JAV record, metadata associations and JAV cover are preserved.
// No query parameters; applies to all directories, including hidden locations.
func deleteJavVideos(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "JAV ID 无效", "Invalid JAV ID")
		return
	}
	videoIDs, err := dbpkg.DeleteJavVideos(c.Request.Context(), id, deleteJavVideoFiles)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			respondLocalizedError(c, http.StatusNotFound, "JAV 不存在", "JAV does not exist")
		case errors.Is(err, dbpkg.ErrJavVideoShared):
			respondLocalizedError(c, http.StatusConflict, "关联视频同时属于其他 JAV，请先修正关联后再删除", "A video is also linked to another JAV; correct its links before deleting")
		case errors.Is(err, errJavDeleteFilesUnavailable):
			respondLocalizedError(c, http.StatusConflict, "目录不可访问或文件路径不安全，未删除视频，请检查后重试", "A directory is unavailable or a file path is unsafe; check it and retry")
		default:
			logging.Error("delete jav videos %d: %v", id, err)
			respondLocalizedError(c, http.StatusInternalServerError, "删除失败，部分文件可能已删除；视频记录已保留，请检查后重试", "Deletion failed; some files may have been removed. The video records were kept; check and retry")
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "video_ids": videoIDs})
}

func deleteJavVideoFiles(locations []models.VideoLocation, videoIDs []int64) error {
	files := make(map[string]struct{})
	videoPaths := make(map[string]bool, len(locations))
	for _, loc := range locations {
		videoPaths[filepath.Join(loc.DirectoryRef.Path, filepath.FromSlash(loc.RelativePath))] = true
	}
	var cacheDirs []string
	// Build and validate the entire plan before removing the first file.
	for _, loc := range locations {
		root := filepath.Clean(loc.DirectoryRef.Path)
		info, err := os.Stat(root)
		if loc.DirectoryRef.Missing || !filepath.IsAbs(root) || err != nil || !info.IsDir() {
			return fmt.Errorf("%w: directory %s", errJavDeleteFilesUnavailable, root)
		}
		path, err := javDeletePath(root, filepath.FromSlash(loc.RelativePath))
		if err != nil {
			return err
		}
		files[path] = struct{}{}
		entries, err := os.ReadDir(filepath.Dir(path))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		var otherStems []string
		for _, sibling := range entries {
			siblingPath := filepath.Join(filepath.Dir(path), sibling.Name())
			if sibling.Type().IsRegular() && !videoPaths[siblingPath] && util.IsVideoCandidate(siblingPath) {
				otherStems = append(otherStems, strings.TrimSuffix(sibling.Name(), filepath.Ext(sibling.Name())))
			}
		}
		for _, entry := range entries {
			if entry.IsDir() || !isJavDeleteSidecar(stem, entry.Name()) {
				continue
			}
			// Preserve attachments also owned by a video outside this deletion.
			shared := false
			for _, other := range otherStems {
				if isJavDeleteSidecar(other, entry.Name()) {
					shared = true
					break
				}
			}
			if shared {
				continue
			}
			candidate, err := javDeletePath(root, filepath.Join(filepath.Dir(loc.RelativePath), entry.Name()))
			if err != nil {
				return err
			}
			files[candidate] = struct{}{}
		}
	}
	if cfg := common.AppConfig; cfg != nil {
		if cfg.DatabasePath != "" {
			for _, id := range videoIDs {
				path, err := javDeletePath(filepath.Dir(cfg.DatabasePath), filepath.Join("video", strconv.FormatInt(id, 10)))
				if err != nil {
					return err
				}
				cacheDirs = append(cacheDirs, path)
			}
		}
	}
	for path := range files {
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%w: %s", errJavDeleteFilesUnavailable, path)
		}
	}
	for path := range files {
		err := util.MoveFileToTrash(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", path, err)
		}
	}
	for _, path := range cacheDirs {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("remove %s: %w", path, err)
		}
	}
	return nil
}

func isJavDeleteSidecar(stem, name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	base := strings.TrimSuffix(name, filepath.Ext(name))
	switch ext {
	case ".srt", ".ass", ".ssa", ".vtt", ".sub", ".idx", ".sup", ".smi":
		return base == stem || strings.HasPrefix(base, stem+".")
	case ".nfo":
		return base == stem
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp", ".avif":
		return base == stem || base == stem+"-poster" || base == stem+"-fanart" || base == stem+"-thumb" || base == stem+"-landscape"
	default:
		return false
	}
}

// Reject traversal and symlinks, including parent directories. Missing files are
// allowed so cleanup can be retried after a partial filesystem failure.
func javDeletePath(root, relative string) (string, error) {
	root = filepath.Clean(root)
	if !filepath.IsAbs(root) || relative == "" || filepath.IsAbs(relative) {
		return "", errJavDeleteFilesUnavailable
	}
	path := filepath.Join(root, relative)
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", errJavDeleteFilesUnavailable
	}
	for current := path; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("%w: symlink %s", errJavDeleteFilesUnavailable, current)
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	return path, nil
}
