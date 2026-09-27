package server

import (
	"bytes"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	dbpkg "javboss/internal/db"
	"javboss/internal/jav/javdb"
	"javboss/internal/util"
)

// getJavSampleImage serves a stored sample image, decoding JavDB's image format.
// index is zero-based; variant is thumbnail or detail. URLs come only from the
// item's stored sample list, not from a caller-supplied URL.
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
	u, err := url.Parse(source)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		c.Status(http.StatusBadGateway)
		return
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, source, nil)
	if err != nil {
		c.Status(http.StatusBadGateway)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := util.DoRequest(req)
	if err != nil {
		c.Status(http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.Status(http.StatusBadGateway)
		return
	}
	body, encoded := javdb.DecodeImageBody(resp.Body)
	const maxImageBytes = 16 << 20
	data, err := io.ReadAll(io.LimitReader(body, maxImageBytes+1))
	contentType := http.DetectContentType(data)
	if err != nil || len(data) > maxImageBytes || !strings.HasPrefix(contentType, "image/") {
		c.Status(http.StatusBadGateway)
		return
	}
	if encoded {
		if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
			c.Status(http.StatusBadGateway)
			return
		}
	}
	c.Header("Cache-Control", "private, max-age=86400")
	c.Data(http.StatusOK, contentType, data)
}
