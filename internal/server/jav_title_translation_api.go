package server

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"javboss/internal/common/logging"
	dbpkg "javboss/internal/db"
	"javboss/internal/jav/translation"
)

var deepSeekClient = translation.NewClient()

// Serialize paid requests and recheck the stored title after waiting, including requests from other tabs.
var titleTranslationMu sync.Mutex

// POST /jav/title-translation/models accepts an optional api_key for an unsaved settings draft.
func listTitleTranslationModels(c *gin.Context) {
	var req struct {
		APIKey *string `json:"api_key"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "模型列表请求无效", "Invalid model list request")
		return
	}
	cfg, err := dbpkg.ListConfig(c.Request.Context())
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "读取翻译配置失败", "Failed to load translation settings")
		return
	}
	key := cfg["jav_title_translation_api_key"]
	if req.APIKey != nil {
		key = strings.TrimSpace(*req.APIKey)
	}
	if key == "" || len(key) > 512 {
		respondLocalizedError(c, http.StatusBadRequest, "请填写 DeepSeek API Key", "Enter a DeepSeek API key")
		return
	}
	models, err := deepSeekClient.Models(c.Request.Context(), key)
	if err != nil {
		respondTitleTranslationError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"models": models})
}

// POST /jav/items/:id/title-translation saves a Chinese title while preserving the original.
// With refresh=true, optional title and thinking override this preview only; nothing is saved.
func translateJavTitle(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "影片 ID 无效", "Invalid JAV ID")
		return
	}
	var req struct {
		Refresh  bool    `json:"refresh"`
		Title    *string `json:"title"`
		Thinking *bool   `json:"thinking"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 65536)
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		respondLocalizedError(c, http.StatusBadRequest, "标题翻译请求无效", "Invalid title translation request")
		return
	}
	if (req.Title != nil && (!req.Refresh || len(*req.Title) > 8192)) || (req.Thinking != nil && !req.Refresh) {
		respondLocalizedError(c, http.StatusBadRequest, "标题翻译请求无效或标题过长", "Invalid title translation request or title too long")
		return
	}
	// Reasoning and queued translations can outlast the server's default write timeout.
	if err := http.NewResponseController(c.Writer).SetWriteDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
		logging.Error("disable title translation write deadline: %v", err)
	}
	titleTranslationMu.Lock()
	defer titleTranslationMu.Unlock()
	cfg, err := dbpkg.ListConfig(c.Request.Context())
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "读取翻译配置失败", "Failed to load translation settings")
		return
	}
	enabled, _ := strconv.ParseBool(cfg["jav_title_translation_enabled"])
	if !enabled && !req.Refresh {
		respondLocalizedError(c, http.StatusBadRequest, "标题翻译未开启", "Title translation is disabled")
		return
	}
	item, err := dbpkg.GetJav(c.Request.Context(), id, nil)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respondLocalizedError(c, http.StatusNotFound, "影片不存在", "JAV item was not found")
		return
	}
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "读取影片失败", "Failed to load JAV item")
		return
	}
	if !req.Refresh && strings.TrimSpace(item.ZhTitle) != "" {
		c.JSON(http.StatusOK, gin.H{"id": item.ID, "title": item.Title, "zh_title": item.ZhTitle})
		return
	}
	sourceTitle := item.Title
	if req.Title != nil {
		sourceTitle = strings.TrimSpace(*req.Title)
	}
	key, model := cfg["jav_title_translation_api_key"], cfg["jav_title_translation_model"]
	if key == "" || model == "" || strings.TrimSpace(sourceTitle) == "" {
		respondLocalizedError(c, http.StatusBadRequest, "请先填写 API Key 并选择模型，影片标题不能为空", "Configure an API key and model; the title must not be empty")
		return
	}
	thinking := cfg["jav_title_translation_thinking"] == "true"
	if req.Thinking != nil {
		thinking = *req.Thinking
	}
	title, err := deepSeekClient.Translate(c.Request.Context(), key, model, cfg["jav_title_translation_prompt"], sourceTitle, thinking)
	if err != nil {
		respondTitleTranslationError(c, err)
		return
	}
	if req.Refresh {
		c.JSON(http.StatusOK, gin.H{"id": item.ID, "title": sourceTitle, "zh_title": title})
		return
	}
	if _, err := dbpkg.SaveJavTitleTranslation(c.Request.Context(), item, title); err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "保存翻译标题失败", "Failed to save translated title")
		return
	}
	item, err = dbpkg.GetJav(c.Request.Context(), id, nil)
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "读取翻译标题失败", "Failed to load translated title")
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": item.ID, "title": item.Title, "zh_title": item.ZhTitle})
}

func respondTitleTranslationError(c *gin.Context, err error) {
	var apiError *translation.APIError
	if errors.As(err, &apiError) {
		switch apiError.StatusCode {
		case http.StatusUnauthorized:
			respondLocalizedError(c, http.StatusBadRequest, "DeepSeek API Key 无效", "The DeepSeek API key is invalid")
			return
		case http.StatusPaymentRequired:
			respondLocalizedError(c, http.StatusBadRequest, "DeepSeek 账户余额不足", "The DeepSeek account balance is insufficient")
			return
		case http.StatusTooManyRequests:
			respondLocalizedError(c, http.StatusTooManyRequests, "DeepSeek 请求过于频繁，请稍后重试", "DeepSeek rate limit reached; retry later")
			return
		}
	}
	respondLocalizedError(c, http.StatusBadGateway, "DeepSeek 请求失败或未返回完整译文，请稍后重试", "DeepSeek failed or returned an incomplete translation; retry later")
}
