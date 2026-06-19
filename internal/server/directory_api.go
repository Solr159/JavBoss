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
	"javboss/internal/service"
	"javboss/internal/util/dirpicker"
)

func listDirectories(c *gin.Context) {
	dirs, err := dbpkg.ListDirectories(c.Request.Context())
	if err != nil {
		logging.Error("list directories error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, dirs)
}

func createDirectory(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if strings.TrimSpace(req.Path) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}

	dir, err := dbpkg.CreateDirectory(c.Request.Context(), req.Path)
	if err != nil {
		logging.Error("create directory error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
	if err := http.NewResponseController(c.Writer).SetWriteDeadline(time.Now().Add(10 * time.Minute)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		logging.Error("set directory picker write deadline failed: %v", err)
	}
	path, err := dirpicker.PickDirectory(c.Request.Context())
	if err != nil {
		if errors.Is(err, dirpicker.ErrDirPickerCanceled) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "directory selection canceled"})
			return
		}
		logging.Error("pick directory error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "pick directory failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"path": path})
}

func updateDirectory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req struct {
		Path     *string `json:"path"`
		IsDelete *bool   `json:"is_delete"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if req.Path != nil && strings.TrimSpace(*req.Path) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}

	var releaseScanReservation func()
	if req.Path != nil {
		reserveCtx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		release, err := service.CancelAndReserveDirectoryScan(reserveCtx, id)
		cancel()
		if err != nil {
			logging.Error("cancel directory scan before update failed id=%d err=%v", id, err)
			c.JSON(http.StatusConflict, gin.H{"error": "directory scan is stopping; try again shortly"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if dir == nil {
		c.Status(http.StatusNotFound)
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
