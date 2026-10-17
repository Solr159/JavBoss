package server

import (
	"encoding/json"
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

var titleTranslationClient = translation.NewClient()

// Serialize paid requests and recheck the stored title after waiting, including requests from other tabs.
var titleTranslationMu sync.Mutex

// POST /jav/title-translation/models discovers models or checks an unsaved model's capabilities.
func listTitleTranslationModels(c *gin.Context) {
	var req struct {
		Provider string          `json:"provider"`
		Profile  json.RawMessage `json:"profile"`
		APIKey   *string         `json:"api_key"`
		Discover *bool           `json:"discover"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 65536)
	if err := c.ShouldBindJSON(&req); err != nil {
		respondLocalizedError(c, 400, "模型列表请求无效", "Invalid model list request")
		return
	}
	cfg, err := dbpkg.ListConfig(c.Request.Context())
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
	if req.APIKey != nil {
		profile.APIKey = strings.TrimSpace(*req.APIKey)
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
	models, err := titleTranslationClient.Models(c.Request.Context(), provider, profile)
	if err != nil {
		respondTitleTranslationError(c, err)
		return
	}
	c.JSON(200, gin.H{"models": models, "capabilities": caps})
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
	if strings.TrimSpace(sourceTitle) == "" {
		respondLocalizedError(c, 400, "影片标题不能为空", "The title must not be empty")
		return
	}
	if req.Refresh && req.Thinking == nil {
		fast := false
		req.Thinking = &fast
	}
	title, err := titleTranslationClient.Translate(c.Request.Context(), translation.ActiveProvider(cfg), translation.ActiveProfile(cfg), cfg["jav_title_translation_prompt"], sourceTitle, req.Thinking)
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
	status, zh, en := titleTranslationErrorDetails(err)
	respondLocalizedError(c, status, zh, en)
}

func titleTranslationErrorDetails(err error) (int, string, string) {
	var configError *translation.ConfigError
	if errors.As(err, &configError) {
		parts := strings.SplitN(configError.Message, " / ", 2)
		en := parts[0]
		if len(parts) > 1 {
			en = parts[1]
		}
		return http.StatusBadRequest, parts[0], en
	}
	status := http.StatusBadGateway
	zh, en := "服务商请求失败或未返回完整译文，请稍后重试", "The provider failed or returned an incomplete translation; retry later"
	var apiError *translation.APIError
	if !errors.As(err, &apiError) {
		return status, zh, en
	}
	switch apiError.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		// Provider authentication failures must not expire the application's session.
		status = http.StatusBadRequest
		zh, en = "API Key 无效或无访问权限", "The API key is invalid or lacks permission"
	case http.StatusPaymentRequired:
		status = http.StatusBadRequest
		zh, en = "服务商账户余额不足", "The provider account balance is insufficient"
	case http.StatusTooManyRequests:
		status = http.StatusTooManyRequests
		zh, en = "服务商请求受到限制，请检查额度或稍后重试", "The provider restricted the request; check your quota or retry later"
	}
	// A specific provider code takes precedence over a broad error type.
	for _, reason := range []string{apiError.Code, apiError.Type} {
		matched := true
		switch reason {
		case "credit_balance_exhausted":
			zh, en = "服务商账户余额已耗尽，请充值后重试", "The provider account credit balance is exhausted; add credits and retry"
		case "organization_spend_limit_exceeded":
			zh, en = "服务商组织支出已达上限，请检查组织额度设置", "The provider organization spending limit has been reached; check its limits"
		case "project_spend_limit_exceeded":
			zh, en = "服务商项目支出已达上限，请检查项目额度设置", "The provider project spending limit has been reached; check its limits"
		case "organization_usage_limit_exceeded":
			zh, en = "服务商组织用量已达上限，请检查用量额度", "The provider organization usage limit has been reached; check its quota"
		case "insufficient_quota":
			zh, en = "服务商可用额度不足，请检查账户余额和额度限制", "The provider quota is insufficient; check your balance and limits"
		case "slow_down", "rate_limit_exceeded", "rate_limit_error":
			zh, en = "服务商请求过于频繁，请稍后重试", "Provider rate limit reached; retry later"
		default:
			matched = false
		}
		if matched {
			break
		}
	}
	diagnostic := "HTTP " + strconv.Itoa(apiError.StatusCode)
	if apiError.Code != "" {
		diagnostic += " / " + apiError.Code
	}
	if apiError.Type != "" && apiError.Type != apiError.Code {
		diagnostic += " / " + apiError.Type
	}
	diagnostic = " (" + diagnostic + ")"
	if apiError.Message != "" {
		diagnostic += ": " + apiError.Message
	}
	return status, zh + diagnostic, en + diagnostic
}
