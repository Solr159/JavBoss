package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"javboss/internal/db"
	"javboss/internal/jav"
	"javboss/internal/manager"
	"javboss/internal/models"
	"javboss/internal/service"
	"javboss/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const maxJavDiscoveryCoverBytes = 15 * 1024 * 1024

const javDiscoveryDetailsTTL = 24 * time.Hour

var (
	javDiscoveryCoverHTTPDo          = doJavDiscoveryCoverRequest
	fetchJavBusDetails               = jav.FetchJavBusDiscoveryItemDetails
	resolveJavBusActressSubscription = jav.ResolveJavBusActressSubscription
	loadMoreJavDiscoveryHistory      = service.LoadMoreJavDiscoveryHistory
)

type createJavDiscoverySubscriptionRequest struct {
	ReferenceCode string `json:"reference_code"`
}

type updateJavDiscoveryWantedRequest struct {
	Wanted *bool `json:"wanted"`
}

func listJavDiscoverySubscriptions(c *gin.Context) {
	subscriptions, err := db.ListJavDiscoverySubscriptions(c.Request.Context())
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "读取发现订阅失败", "Failed to list discovery subscriptions")
		return
	}
	c.JSON(http.StatusOK, subscriptions)
}

func createJavDiscoverySubscription(c *gin.Context) {
	var request createJavDiscoverySubscriptionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "订阅参数格式不正确", "Invalid subscription payload")
		return
	}
	request.ReferenceCode = strings.ToUpper(strings.TrimSpace(request.ReferenceCode))
	if request.ReferenceCode == "" {
		respondLocalizedError(c, http.StatusBadRequest, "请输入一个单体作品番号", "Enter a solo work code")
		return
	}

	validated, err := resolveJavBusActressSubscription(c.Request.Context(), request.ReferenceCode)
	if err != nil {
		if errors.Is(err, jav.ResourceNotFonud) {
			respondLocalizedError(
				c,
				http.StatusUnprocessableEntity,
				"JavBus 校验失败：番号必须准确对应一部单体女优作品",
				"JavBus validation failed: the code must exactly identify a solo actress work",
			)
			return
		}
		respondLocalizedError(c, http.StatusBadGateway, "暂时无法连接 JavBus，请稍后重试", "JavBus is temporarily unavailable; try again later")
		return
	}

	subscription := models.JavDiscoverySubscription{
		Kind:            "idol",
		Name:            validated.Name,
		ReferenceCode:   request.ReferenceCode,
		ProviderLocator: validated.ProviderLocator,
	}
	if err := db.CreateJavDiscoverySubscription(c.Request.Context(), &subscription); err != nil {
		if errors.Is(err, db.ErrJavDiscoverySubscriptionExists) {
			respondLocalizedError(c, http.StatusConflict, "该女优已经订阅", "This idol is already subscribed")
			return
		}
		respondLocalizedError(c, http.StatusInternalServerError, "添加订阅失败", "Failed to add subscription")
		return
	}
	c.JSON(http.StatusCreated, subscription)
}

func loadMoreJavDiscoverySubscriptionHistory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "订阅 ID 不正确", "Invalid subscription ID")
		return
	}
	loaded, err := loadMoreJavDiscoveryHistory(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondLocalizedError(c, http.StatusNotFound, "订阅不存在", "Subscription not found")
			return
		}
		respondLocalizedError(c, http.StatusBadGateway, "加载历史作品失败", "Failed to load historical works")
		return
	}
	c.JSON(http.StatusOK, gin.H{"loaded": loaded})
}

func deleteJavDiscoverySubscription(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "订阅 ID 不正确", "Invalid subscription ID")
		return
	}
	if err := db.DeleteJavDiscoverySubscription(c.Request.Context(), id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondLocalizedError(c, http.StatusNotFound, "订阅不存在", "Subscription not found")
			return
		}
		respondLocalizedError(c, http.StatusInternalServerError, "删除订阅失败", "Failed to delete subscription")
		return
	}
	c.Status(http.StatusNoContent)
}

func listJavDiscoveryItems(c *gin.Context) {
	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)
	wantedOnly := queryBool(c, "wanted", false)
	subscriptionID := int64(queryInt(c, "subscription_id", 0))
	items, total, err := db.ListJavDiscoveryItems(c.Request.Context(), db.ListJavDiscoveryItemsOptions{
		WantedOnly:     wantedOnly,
		IncludeOwned:   queryBool(c, "include_owned", false),
		SubscriptionID: subscriptionID,
		Limit:          limit,
		Offset:         offset,
	})
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "读取发现作品失败", "Failed to list discovery items")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

func updateJavDiscoveryItemWanted(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "作品 ID 不正确", "Invalid item ID")
		return
	}
	var request updateJavDiscoveryWantedRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.Wanted == nil {
		respondLocalizedError(c, http.StatusBadRequest, "请提供 wanted 状态", "Provide a wanted state")
		return
	}
	if err := db.SetJavDiscoveryItemWanted(c.Request.Context(), id, *request.Wanted); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondLocalizedError(c, http.StatusNotFound, "发现作品不存在", "Discovery item not found")
			return
		}
		respondLocalizedError(c, http.StatusInternalServerError, "更新想要状态失败", "Failed to update wanted state")
		return
	}
	c.Status(http.StatusNoContent)
}

func resolveJavDiscoveryItemDetails(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "作品 ID 不正确", "Invalid item ID")
		return
	}
	record, err := db.GetJavDiscoveryItem(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondLocalizedError(c, http.StatusNotFound, "发现作品不存在", "Discovery item not found")
			return
		}
		respondLocalizedError(c, http.StatusInternalServerError, "读取发现作品失败", "Failed to read discovery item")
		return
	}
	var cached jav.JavBusDiscoveryItem
	if json.Unmarshal([]byte(record.MetadataJSON), &cached) == nil &&
		cached.DetailsFetchedAt != nil &&
		cached.DetailsFetchedAt.After(time.Now().Add(-javDiscoveryDetailsTTL)) &&
		javDiscoveryMagnetLinksFetched(record.MagnetLinksJSON) {
		respondJavDiscoveryDetails(c, record)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	details, err := fetchJavBusDetails(ctx, record.Code)
	if err != nil {
		if errors.Is(err, jav.ResourceNotFonud) {
			respondLocalizedError(c, http.StatusNotFound, "JavBus 上未找到该作品详情", "JavBus details were not found")
			return
		}
		respondLocalizedError(c, http.StatusBadGateway, "暂时无法从 JavBus 加载详情", "Unable to load details from JavBus")
		return
	}
	if details == nil {
		respondLocalizedError(c, http.StatusBadGateway, "JavBus 返回了空详情", "JavBus returned empty details")
		return
	}
	record, err = db.UpdateJavDiscoveryItemDetails(c.Request.Context(), id, *details)
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "保存发现作品详情失败", "Failed to save discovery item details")
		return
	}
	respondJavDiscoveryDetails(c, record)
}

func respondJavDiscoveryDetails(c *gin.Context, record *models.JavDiscoveryItem) {
	metadata := json.RawMessage(strings.TrimSpace(record.MetadataJSON))
	if !json.Valid(metadata) {
		metadata = json.RawMessage(`{}`)
	}
	magnetLinks := json.RawMessage(strings.TrimSpace(record.MagnetLinksJSON))
	if !json.Valid(magnetLinks) || string(magnetLinks) == "null" {
		magnetLinks = json.RawMessage(`[]`)
	}
	c.JSON(http.StatusOK, gin.H{
		"metadata":     metadata,
		"magnet_links": magnetLinks,
		"release_unix": record.ReleaseUnix,
		"updated_at":   record.UpdatedAt,
	})
}

func javDiscoveryMagnetLinksFetched(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || value == "null" {
		return false
	}
	var links []jav.JavBusMagnetLink
	return json.Unmarshal([]byte(value), &links) == nil
}

func getJavDiscoveryItemCover(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "作品 ID 不正确", "Invalid item ID")
		return
	}
	coverURL, err := db.GetJavDiscoveryItemCoverURL(c.Request.Context(), id)
	proxyJavDiscoveryItemImage(c, coverURL, err)
}

func getJavDiscoveryItemThumbnail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "作品 ID 不正确", "Invalid item ID")
		return
	}
	coverURL, err := db.GetJavDiscoveryItemThumbnailURL(c.Request.Context(), id)
	proxyJavDiscoveryItemImage(c, coverURL, err)
}

func proxyJavDiscoveryItemImage(c *gin.Context, coverURL string, err error) {
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondLocalizedError(c, http.StatusNotFound, "发现作品封面不存在", "Discovery item cover not found")
			return
		}
		respondLocalizedError(c, http.StatusInternalServerError, "读取发现作品封面失败", "Failed to read discovery item cover")
		return
	}
	parsed, err := url.Parse(coverURL)
	if err != nil ||
		!strings.EqualFold(parsed.Scheme, "https") ||
		!isAllowedJavDiscoveryCoverHost(parsed.Hostname()) {
		respondLocalizedError(c, http.StatusUnprocessableEntity, "发现作品封面地址不受支持", "Unsupported discovery cover URL")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "创建封面请求失败", "Failed to build cover request")
		return
	}
	manager.SetCoverDownloadHeaders(request)
	response, err := javDiscoveryCoverHTTPDo(request)
	if err != nil {
		respondLocalizedError(c, http.StatusBadGateway, "下载发现作品封面失败", "Failed to download discovery item cover")
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		respondLocalizedError(c, http.StatusBadGateway, "封面图片源返回异常", "The cover image source returned an error")
		return
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxJavDiscoveryCoverBytes+1))
	if err != nil {
		respondLocalizedError(c, http.StatusBadGateway, "读取封面图片失败", "Failed to read the cover image")
		return
	}
	if len(data) == 0 || len(data) > maxJavDiscoveryCoverBytes {
		respondLocalizedError(c, http.StatusBadGateway, "封面图片大小无效", "Invalid cover image size")
		return
	}
	contentType, ok := discoveryCoverContentType(response.Header.Get("Content-Type"), data)
	if !ok {
		respondLocalizedError(c, http.StatusBadGateway, "封面地址未返回有效图片", "The cover URL did not return a valid image")
		return
	}

	c.Header("Cache-Control", "private, max-age=86400")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, contentType, data)
}

func isAllowedJavDiscoveryCoverHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	for _, domain := range []string{"javbus.com", "dmm.co.jp"} {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

func doJavDiscoveryCoverRequest(request *http.Request) (*http.Response, error) {
	client := *util.DefaultHTTPClient()
	client.CheckRedirect = func(next *http.Request, _ []*http.Request) error {
		if next.URL == nil ||
			!strings.EqualFold(next.URL.Scheme, "https") ||
			!isAllowedJavDiscoveryCoverHost(next.URL.Hostname()) {
			return errors.New("discovery cover redirect is not allowed")
		}
		manager.SetCoverDownloadHeaders(next)
		return nil
	}
	return client.Do(request)
}

func discoveryCoverContentType(declared string, data []byte) (string, bool) {
	declared = strings.ToLower(strings.TrimSpace(strings.SplitN(declared, ";", 2)[0]))
	detected := strings.ToLower(http.DetectContentType(data))
	for _, contentType := range []string{declared, detected} {
		if contentType == "image/svg+xml" {
			continue
		}
		if strings.HasPrefix(contentType, "image/") {
			return contentType, true
		}
	}
	return "", false
}

func triggerJavDiscoverySync(c *gin.Context) {
	service.TriggerJavDiscoverySync()
	c.JSON(http.StatusAccepted, gin.H{"status": "scheduled"})
}
