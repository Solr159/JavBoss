package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"javboss/internal/db"
	"javboss/internal/jav"
	"javboss/internal/models"
	"javboss/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type createJavDiscoverySubscriptionRequest struct {
	Name          string `json:"name"`
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
	request.Name = strings.Join(strings.Fields(strings.TrimSpace(request.Name)), " ")
	request.ReferenceCode = strings.ToUpper(strings.TrimSpace(request.ReferenceCode))
	if request.Name == "" || request.ReferenceCode == "" {
		respondLocalizedError(c, http.StatusBadRequest, "请输入女优名和一个单体作品番号", "Enter an idol name and one solo work code")
		return
	}

	validated, err := jav.ValidateJavBusActressSubscription(c.Request.Context(), request.Name, request.ReferenceCode)
	if err != nil {
		if errors.Is(err, jav.ResourceNotFonud) {
			respondLocalizedError(
				c,
				http.StatusUnprocessableEntity,
				"JavBus 校验失败：番号必须是该女优的单体作品，且女优名需与页面一致",
				"JavBus validation failed: the code must be a solo work for the exact idol name",
			)
			return
		}
		respondLocalizedError(c, http.StatusBadGateway, "暂时无法连接 JavBus，请稍后重试", "JavBus is temporarily unavailable; try again later")
		return
	}

	subscription := models.JavDiscoverySubscription{
		Kind:          "idol",
		Name:          validated.Name,
		ReferenceCode: request.ReferenceCode,
		ProviderKey:   validated.ProviderKey,
	}
	if err := db.CreateJavDiscoverySubscription(c.Request.Context(), &subscription); err != nil {
		if errors.Is(err, db.ErrJavDiscoverySubscriptionExists) {
			respondLocalizedError(c, http.StatusConflict, "该女优已经订阅", "This idol is already subscribed")
			return
		}
		respondLocalizedError(c, http.StatusInternalServerError, "添加订阅失败", "Failed to add subscription")
		return
	}
	service.TriggerJavDiscoverySync()
	c.JSON(http.StatusCreated, subscription)
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
	items, total, err := db.ListJavDiscoveryItems(c.Request.Context(), wantedOnly, limit, offset)
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

func triggerJavDiscoverySync(c *gin.Context) {
	service.TriggerJavDiscoverySync()
	c.JSON(http.StatusAccepted, gin.H{"status": "scheduled"})
}
