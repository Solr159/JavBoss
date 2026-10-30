package api

import (
	"encoding/json"
	"errors"
	"io"
	"javboss/internal/common/logging"
	dbpkg "javboss/internal/db"
	"javboss/internal/jav/translation"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// POST /jav/title-translation/models discovers models or checks an unsaved model's capabilities.
func listTitleTranslationModels(c *gin.Context) {
	var req struct {
		Provider string          `json:"provider"`
		Profile  json.RawMessage `json:"profile"`
		Discover *bool           `json:"discover"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 65536)
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, 400, "模型列表请求无效", "Invalid model list request")
		return
	}
	cfg, err := dbpkg.Default().ListConfig(c.Request.Context())
	if err != nil {
		respondLocalizedError(c, 500, "读取翻译配置失败", "Failed to load translation settings")
		return
	}
	provider := req.Provider
	if provider == "" {
		provider = translation.ActiveProvider(cfg)
	}
	profile := translation.LoadProfiles(cfg)[provider]
	if len(req.Profile) > 0 {
		if err := json.Unmarshal(req.Profile, &profile); err != nil {
			respondLocalizedError(c, 400, "翻译配置无效", "Invalid translation settings")
			return
		}
	}
	if err := profile.Validate(provider); err != nil {
		respondLocalizedError(c, 400, "翻译配置无效", "Invalid translation settings")
		return
	}
	caps := translation.ModelCapabilities(provider, profile)
	if req.Discover != nil && !*req.Discover {
		c.JSON(200, gin.H{"capabilities": caps})
		return
	}
	if profile.APIKey == "" && provider != "custom" {
		respondLocalizedError(c, 400, "请填写 API Key", "Enter an API key")
		return
	}
	models, err := translation.Models(c.Request.Context(), provider, profile)
	if err != nil {
		respondTitleTranslationError(c, err)
		return
	}
	c.JSON(200, gin.H{"models": models, "capabilities": caps})
}

// POST /jav/title-translation/test translates a sample with draft settings; nothing is saved.
func testTitleTranslation(c *gin.Context) {
	var req struct {
		Provider string          `json:"provider"`
		Profile  json.RawMessage `json:"profile"`
		Prompt   string          `json:"prompt"`
		Title    string          `json:"title"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 65536)
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Prompt) > 8192 || len(req.Title) > 8192 || strings.TrimSpace(req.Title) == "" {
		respondLocalizedError(c, 400, "测试标题或配置无效", "Invalid sample title or settings")
		return
	}
	cfg, err := dbpkg.Default().ListConfig(c.Request.Context())
	if err != nil {
		respondLocalizedError(c, 500, "读取翻译配置失败", "Failed to load translation settings")
		return
	}
	provider := req.Provider
	if provider == "" {
		provider = translation.ActiveProvider(cfg)
	}
	// Merge exactly as saving does, retaining an omitted API key and replacing request objects.
	profiles, err := translation.MergeProfiles(cfg, map[string]json.RawMessage{provider: req.Profile})
	if err != nil {
		respondLocalizedError(c, 400, "翻译配置无效", "Invalid translation settings")
		return
	}
	cfg[translation.ProfilesKey] = profiles
	profile := translation.LoadProfiles(cfg)[provider]
	if err := http.NewResponseController(c.Writer).SetWriteDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
		logging.Error("disable test translation write deadline: %v", err)
	}
	release, err := translation.Acquire(c.Request.Context(), 0)
	if err != nil {
		return
	}
	defer release()
	title, err := translation.Translate(c.Request.Context(), provider, profile, req.Prompt, strings.TrimSpace(req.Title), nil)
	if err != nil {
		respondTitleTranslationError(c, err)
		return
	}
	key := "zh_title"
	if translation.TargetLanguage(cfg) == "en" {
		key = "en_title"
	}
	c.JSON(http.StatusOK, gin.H{key: title})
}

// POST /jav/items/:id/title-translation previews a Chinese title; nothing is saved.
// Optional title and thinking fields override the saved source and reasoning setting.
func translateJavTitle(c *gin.Context) {
	translateText(c, javTitleDomain)
}

func translateText(c *gin.Context, domain textTranslationDomain) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		domain.error(c, http.StatusBadRequest, "影片 ID 无效", "Invalid JAV ID")
		return
	}
	var req struct {
		Title    *string `json:"title"`
		Name     *string `json:"name"`
		Thinking *bool   `json:"thinking"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 65536)
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		domain.error(c, http.StatusBadRequest, "标题翻译请求无效", "Invalid title translation request")
		return
	}
	if domain.Series {
		req.Title = req.Name
	}
	if req.Title != nil && len(*req.Title) > 8192 {
		domain.error(c, http.StatusBadRequest, "标题翻译请求无效或标题过长", "Invalid title translation request or title too long")
		return
	}
	// Reasoning and queued translations can outlast the server's default write timeout.
	if err := http.NewResponseController(c.Writer).SetWriteDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
		logging.Error("disable title translation write deadline: %v", err)
	}
	release, err := domain.Acquire(c.Request.Context(), id)
	if err != nil {
		return
	}
	defer release()
	cfg, err := dbpkg.Default().ListConfig(c.Request.Context())
	if err != nil {
		domain.error(c, http.StatusInternalServerError, "读取翻译配置失败", "Failed to load translation settings")
		return
	}
	domain = domain.withConfig(cfg)
	if !domain.Enabled(cfg) {
		domain.error(c, 400, "标题翻译未开启", "Title translation is disabled")
		return
	}
	item, err := domain.Get(c.Request.Context(), id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		domain.error(c, http.StatusNotFound, "影片不存在", "JAV item was not found")
		return
	}
	if err != nil {
		domain.error(c, http.StatusInternalServerError, "读取影片失败", "Failed to load JAV item")
		return
	}
	sourceTitle := item.Source
	if req.Title != nil {
		sourceTitle = strings.TrimSpace(*req.Title)
	}
	if strings.TrimSpace(sourceTitle) == "" {
		domain.error(c, 400, "影片标题不能为空", "The title must not be empty")
		return
	}
	title, err := translation.Translate(c.Request.Context(), translation.ActiveProvider(cfg), translation.ActiveProfile(cfg), translation.PromptFor(cfg), sourceTitle, req.Thinking)
	if err != nil {
		respondTitleTranslationError(c, err)
		return
	}
	item.Source, item.Translation = sourceTitle, title
	c.JSON(http.StatusOK, domain.response(item))
}

func respondTitleTranslationError(c *gin.Context, err error) {
	status, zh, en := translation.ErrorDetails(err)
	respondLocalizedError(c, status, zh, en)
}
