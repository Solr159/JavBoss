package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"javboss/internal/common"
	dbpkg "javboss/internal/db"
)

// getJavSampleImage serves a cached sample image, downloading JavDB's image
// format on a cache miss. index is zero-based; variant is thumbnail or detail.
// URLs come only from the item's stored sample list, not from a caller-supplied URL.
func getJavSampleImage(c *gin.Context) {
	id, idErr := strconv.ParseInt(c.Param("id"), 10, 64)
	index, indexErr := strconv.Atoi(c.Param("index"))
	variant := c.Param("variant")
	if idErr != nil || id <= 0 || indexErr != nil || index < 0 || (variant != "thumbnail" && variant != "detail") {
		c.Status(http.StatusBadRequest)
		return
	}
	item, err := dbpkg.GetJav(c.Request.Context(), id, nil)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.Status(http.StatusNotFound)
		} else {
			c.Status(http.StatusInternalServerError)
		}
		return
	}
	if index >= len(item.SampleImages) || item.SampleImages.IsNotFound() {
		c.Status(http.StatusNotFound)
		return
	}
	sample := item.SampleImages[index]
	source := sample.DetailURL
	if variant == "thumbnail" {
		source = sample.ThumbnailURL
	}
	if source == "" {
		source = sample.DetailURL
		if source == "" {
			source = sample.ThumbnailURL
		}
	}
	source = strings.TrimSpace(source)

	manager := common.SampleImageManager
	if manager == nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	path, err := manager.Get(c.Request.Context(), source)
	if err != nil {
		c.Status(http.StatusBadGateway)
		return
	}
	// Warm the full-size sibling while grid thumbnails load so opening the
	// preview does not wait on the remote host.
	if variant == "thumbnail" {
		if detail := strings.TrimSpace(sample.DetailURL); detail != "" && detail != source {
			manager.Warm(detail)
		}
	}
	c.Header("Cache-Control", "private, max-age=86400")
	c.File(path)
}
