package server

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"javboss/internal/common/logging"
)

const maxScreenshotUploadBytes int64 = 25 << 20

var (
	errScreenshotUploadTooLarge = errors.New("screenshot upload is too large")
	errScreenshotUploadType     = errors.New("screenshot upload type is invalid")
)

func uploadVideoScreenshot(c *gin.Context) {
	id, screenshotDir, ok := resolveVideoScreenshotDir(c)
	if !ok {
		return
	}
	name := filepath.Base(strings.TrimSpace(c.Param("name")))
	if !isScreenshotImageName(name) || name != strings.TrimSpace(c.Param("name")) {
		respondLocalizedError(c, http.StatusBadRequest, "截图文件名无效", "Invalid screenshot filename")
		return
	}
	if c.Request.ContentLength > maxScreenshotUploadBytes {
		respondLocalizedError(c, http.StatusRequestEntityTooLarge, "截图文件过大", "The screenshot file is too large")
		return
	}

	info, err := saveUploadedScreenshot(screenshotDir, name, c.Request.Body)
	if err != nil {
		switch {
		case errors.Is(err, errScreenshotUploadTooLarge):
			respondLocalizedError(c, http.StatusRequestEntityTooLarge, "截图文件过大", "The screenshot file is too large")
		case errors.Is(err, errScreenshotUploadType):
			respondLocalizedError(c, http.StatusUnsupportedMediaType, "截图图片格式无效", "The screenshot image format is invalid")
		default:
			logging.Error("save uploaded video screenshot error: %v", err)
			respondLocalizedError(c, http.StatusInternalServerError, "保存上传截图失败", "Failed to save the uploaded screenshot")
		}
		return
	}
	imageURL := "/videos/" + strconv.FormatInt(id, 10) + "/screenshots/" + url.PathEscape(name)
	imageURL += "?mtime=" + strconv.FormatInt(info.ModTime().UnixNano(), 10)
	c.JSON(http.StatusCreated, videoScreenshotInfo{
		VideoID:    id,
		Name:       name,
		URL:        imageURL,
		Size:       info.Size(),
		ModifiedAt: info.ModTime(),
	})
}

func saveUploadedScreenshot(dir, name string, input io.Reader) (os.FileInfo, error) {
	return saveUploadedScreenshotWithLimit(dir, name, input, maxScreenshotUploadBytes)
}

func saveUploadedScreenshotWithLimit(dir, name string, input io.Reader, maxBytes int64) (os.FileInfo, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create screenshot upload directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".javboss-screenshot-upload-*")
	if err != nil {
		return nil, fmt.Errorf("create screenshot upload temp file: %w", err)
	}
	tempPath := temp.Name()
	cleanup := func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}

	written, err := io.Copy(temp, io.LimitReader(input, maxBytes+1))
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("write screenshot upload: %w", err)
	}
	if written <= 0 {
		cleanup()
		return nil, errScreenshotUploadType
	}
	if written > maxBytes {
		cleanup()
		return nil, errScreenshotUploadTooLarge
	}
	if _, err := temp.Seek(0, io.SeekStart); err != nil {
		cleanup()
		return nil, fmt.Errorf("inspect screenshot upload: %w", err)
	}
	header := make([]byte, 512)
	n, err := temp.Read(header)
	if err != nil && !errors.Is(err, io.EOF) {
		cleanup()
		return nil, fmt.Errorf("read screenshot upload header: %w", err)
	}
	if !screenshotUploadTypeMatches(name, header[:n]) {
		cleanup()
		return nil, errScreenshotUploadType
	}
	if err := temp.Chmod(0o644); err != nil {
		cleanup()
		return nil, fmt.Errorf("set screenshot upload permissions: %w", err)
	}
	if err := temp.Sync(); err != nil {
		cleanup()
		return nil, fmt.Errorf("sync screenshot upload: %w", err)
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return nil, fmt.Errorf("close screenshot upload: %w", err)
	}

	target := filepath.Join(dir, name)
	if err := replaceUploadedScreenshot(tempPath, target); err != nil {
		_ = os.Remove(tempPath)
		return nil, fmt.Errorf("install screenshot upload: %w", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		return nil, fmt.Errorf("inspect saved screenshot upload: %w", err)
	}
	return info, nil
}

func screenshotUploadTypeMatches(name string, header []byte) bool {
	detected := http.DetectContentType(header)
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg":
		return detected == "image/jpeg"
	case ".png":
		return detected == "image/png"
	case ".webp":
		return detected == "image/webp"
	default:
		return false
	}
}

func replaceUploadedScreenshot(source, target string) error {
	if err := os.Rename(source, target); err == nil {
		return nil
	} else if _, statErr := os.Stat(target); statErr != nil {
		return err
	}
	if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(source, target)
}
