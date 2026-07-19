package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"javboss/internal/common/logging"
	dbpkg "javboss/internal/db"
	"javboss/internal/models"
	"javboss/internal/runtimeconfig"
	"javboss/internal/service"
	"javboss/internal/util/dirpicker"
)

func listDirectories(c *gin.Context) {
	dirs, err := dbpkg.ListDirectories(c.Request.Context())
	if err != nil {
		logging.Error("list directories error: %v", err)
		respondLocalizedError(c, http.StatusInternalServerError, "加载目录列表失败", "Failed to load directories")
		return
	}
	c.JSON(http.StatusOK, dirs)
}

func createDirectory(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "添加目录请求无效", "Invalid add-directory request")
		return
	}
	if strings.TrimSpace(req.Path) == "" {
		respondLocalizedError(c, http.StatusBadRequest, "目录路径不能为空", "Directory path is required")
		return
	}

	dir, err := dbpkg.CreateDirectory(c.Request.Context(), req.Path)
	if err != nil {
		logging.Error("create directory error: %v", err)
		respondLocalizedError(c, http.StatusBadRequest, "添加目录失败，请检查路径是否有效或已存在", "Failed to add directory; check whether the path is valid or already exists")
		return
	}
	go func(created models.Directory) {
		ctx := context.Background()
		if _, err := service.SyncDirectory(ctx, created); err != nil {
			if errors.Is(err, service.ErrDirectoryScanInProgress) {
				return
			}
			logging.Error("scan after create failed id=%d path=%s err=%v", created.ID, created.Path, err)
		}
	}(*dir)
	c.JSON(http.StatusCreated, dir)
}

func pickDirectory(c *gin.Context) {
	if runtimeconfig.DisableDirectoryPicker() {
		respondLocalizedError(c, http.StatusNotImplemented, "当前部署模式已禁用目录选择器", "The directory picker is disabled in this deployment")
		return
	}
	if err := http.NewResponseController(c.Writer).SetWriteDeadline(time.Now().Add(10 * time.Minute)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		logging.Error("set directory picker write deadline failed: %v", err)
	}
	path, err := dirpicker.PickDirectory(c.Request.Context())
	if err != nil {
		if errors.Is(err, dirpicker.ErrDirPickerCanceled) {
			respondLocalizedError(c, http.StatusBadRequest, "已取消选择目录", "Directory selection was canceled")
			return
		}
		logging.Error("pick directory error: %v", err)
		respondLocalizedError(c, http.StatusInternalServerError, "打开目录选择器失败", "Failed to open the directory picker")
		return
	}
	c.JSON(http.StatusOK, gin.H{"path": path})
}

func updateDirectory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "目录 ID 无效", "Invalid directory ID")
		return
	}

	var req struct {
		Path     *string `json:"path"`
		IsDelete *bool   `json:"is_delete"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "修改目录请求无效", "Invalid directory update request")
		return
	}
	if req.Path != nil && strings.TrimSpace(*req.Path) == "" {
		respondLocalizedError(c, http.StatusBadRequest, "目录路径不能为空", "Directory path is required")
		return
	}

	var releaseScanReservation func()
	if req.Path != nil {
		reserveCtx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		release, err := service.CancelAndReserveDirectoryScan(reserveCtx, id)
		cancel()
		if err != nil {
			logging.Error("cancel directory scan before update failed id=%d err=%v", id, err)
			respondLocalizedError(c, http.StatusConflict, "目录扫描正在停止，请稍后重试", "The directory scan is stopping; please try again shortly")
			return
		}
		releaseScanReservation = release
		defer func() {
			if releaseScanReservation != nil {
				releaseScanReservation()
			}
		}()
	}

	dir, err := dbpkg.UpdateDirectory(c.Request.Context(), id, req.Path, req.IsDelete)
	if err != nil {
		logging.Error("update directory error: %v", err)
		respondLocalizedError(c, http.StatusBadRequest, "修改目录失败，请检查路径是否有效或已存在", "Failed to update directory; check whether the path is valid or already exists")
		return
	}
	if dir == nil {
		respondLocalizedError(c, http.StatusNotFound, "目录不存在", "Directory does not exist")
		return
	}
	if releaseScanReservation != nil {
		releaseScanReservation()
		releaseScanReservation = nil
	}
	go func(updated models.Directory) {
		if updated.IsDelete {
			return
		}
		ctx := context.Background()
		if _, err := service.SyncDirectory(ctx, updated); err != nil {
			if errors.Is(err, service.ErrDirectoryScanInProgress) {
				return
			}
			logging.Error("scan after update failed id=%d path=%s err=%v", updated.ID, updated.Path, err)
		}
	}(*dir)
	c.JSON(http.StatusOK, dir)
}
