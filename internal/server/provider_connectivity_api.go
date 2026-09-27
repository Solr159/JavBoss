package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"javboss/internal/jav"
)

func listConnectivityProviders(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, jav.ConnectivityProviders())
}

func checkProviderConnectivity(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("provider"))
	if err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "数据源无效", "Invalid provider")
		return
	}
	result, err := jav.CheckConnectivity(c.Request.Context(), jav.ParseProvider(id))
	if err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "该数据源不支持连通性检测", "Connectivity checks are not supported for this provider")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, result)
}
