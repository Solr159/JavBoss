package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"javboss/internal/common/logging"
	dbpkg "javboss/internal/db"
	"javboss/internal/jav/translation"
)

// POST /jav/series/:id/name-translation saves a Chinese series name while preserving the original.
func translateJavSeriesName(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "系列 ID 无效", "Invalid series ID")
		return
	}
	// Reasoning can outlast the server's default write timeout.
	if err := http.NewResponseController(c.Writer).SetWriteDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
		logging.Error("disable series name translation write deadline: %v", err)
	}
	// Share the paid-request lock with JAV title translation.
	titleTranslationMu.Lock()
	defer titleTranslationMu.Unlock()
	cfg, err := dbpkg.ListConfig(c.Request.Context())
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "读取翻译配置失败", "Failed to load translation settings")
		return
	}
	enabled, _ := strconv.ParseBool(cfg["jav_title_translation_enabled"])
	if !enabled {
		respondLocalizedError(c, http.StatusBadRequest, "标题翻译未开启", "Title translation is disabled")
		return
	}
	item, err := dbpkg.GetJavSeriesRecord(c.Request.Context(), id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respondLocalizedError(c, http.StatusNotFound, "系列不存在", "Series was not found")
		return
	}
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "读取系列失败", "Failed to load the series")
		return
	}
	if strings.TrimSpace(item.ZhName) != "" {
		c.JSON(http.StatusOK, gin.H{"id": item.ID, "name": item.Name, "zh_name": item.ZhName})
		return
	}
	key, model := cfg["jav_title_translation_api_key"], cfg["jav_title_translation_model"]
	if key == "" || model == "" || strings.TrimSpace(item.Name) == "" {
		respondLocalizedError(c, http.StatusBadRequest, "请先填写 API Key 并选择模型，系列名称不能为空", "Configure an API key and model; the series name must not be empty")
		return
	}
	name, err := deepSeekClient.Translate(c.Request.Context(), key, model, translation.DefaultSeriesNamePrompt, item.Name, cfg["jav_title_translation_thinking"] == "true")
	if err != nil {
		respondTitleTranslationError(c, err)
		return
	}
	if _, err := dbpkg.SaveJavSeriesNameTranslation(c.Request.Context(), item, name); err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "保存系列译名失败", "Failed to save the translated series name")
		return
	}
	item, err = dbpkg.GetJavSeriesRecord(c.Request.Context(), id)
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "读取系列译名失败", "Failed to load the translated series name")
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": item.ID, "name": item.Name, "zh_name": item.ZhName})
}
