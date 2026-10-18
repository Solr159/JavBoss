package api

import (
	"net/http"
	"strconv"

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
