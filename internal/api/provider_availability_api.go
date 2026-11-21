package api

import (
	"net/http"
	"net/url"
	"strconv"

	"javboss/internal/common/logging"
	dbpkg "javboss/internal/db"
	"javboss/internal/jav/scrape"

	"github.com/gin-gonic/gin"
)

func listAvailabilityProviders(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, scrape.AvailabilityProviders())
}

func checkProviderAvailability(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("provider"))
	if err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "数据源无效", "Invalid provider")
		return
	}
	result, err := scrape.CheckAvailability(c.Request.Context(), scrape.ParseProvider(id))
	if err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "该数据源不支持可用性检测", "Availability checks are not supported for this provider")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, result)
}

type providerBaseURLRequest struct {
	BaseURL *string `json:"base_url"`
}

type providerBaseURLResponse struct {
	Provider       scrape.Provider `json:"provider"`
	BaseURL        string          `json:"base_url"`
	DefaultBaseURL string          `json:"default_base_url"`
	Domain         string          `json:"domain"`
}

// updateProviderBaseURL stores and applies a custom origin for one provider so
// users can scrape a mirror instead of the built-in site.
func updateProviderBaseURL(c *gin.Context) {
	provider := scrape.ParseProvider(0)
	if id, err := strconv.Atoi(c.Param("provider")); err == nil {
		provider = scrape.ParseProvider(id)
	}
	if scrape.ProviderBaseURLConfigKey(provider) == "" {
		respondLocalizedError(c, http.StatusBadRequest, "该数据源不支持自定义地址", "This provider does not support a custom URL")
		return
	}

	var req providerBaseURLRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.BaseURL == nil {
		respondLocalizedError(c, http.StatusBadRequest, "请求无效", "Invalid request")
		return
	}
	normalized, err := scrape.NormalizeProviderBaseURL(provider, *req.BaseURL)
	if err != nil {
		respondLocalizedError(
			c,
			http.StatusBadRequest,
			"数据源地址无效：仅支持 http(s) 站点地址，不能包含路径、参数或账号信息",
			"Invalid provider URL: use an http(s) origin without a path, query, or credentials",
		)
		return
	}

	key := scrape.ProviderBaseURLConfigKey(provider)
	if err := dbpkg.Default().UpsertConfig(c.Request.Context(), map[string]string{key: normalized}); err != nil {
		logging.Error("save provider base url error: %v", err)
		respondLocalizedError(c, http.StatusInternalServerError, "保存数据源地址失败", "Failed to save the provider URL")
		return
	}
	if err := scrape.SetProviderBaseURL(provider, normalized); err != nil {
		logging.Error("apply provider base url error: %v", err)
		respondLocalizedError(c, http.StatusInternalServerError, "应用数据源地址失败", "Failed to apply the provider URL")
		return
	}

	response := providerBaseURLResponse{
		Provider:       provider,
		BaseURL:        normalized,
		DefaultBaseURL: scrape.DefaultProviderBaseURL(provider),
	}
	effective := normalized
	if effective == "" {
		effective = response.DefaultBaseURL
	}
	if parsed, err := url.Parse(effective); err == nil {
		response.Domain = parsed.Hostname()
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, response)
}
