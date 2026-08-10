package server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	dbpkg "javboss/internal/db"
)

func listWesternEntities(c *gin.Context) {
	kind := strings.TrimSpace(c.Param("kind"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "25"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	items, total, err := dbpkg.ListWesternEntities(c.Request.Context(), kind, c.Query("search"), limit, offset)
	if err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "Western 数据类型无效", "Invalid Western entity kind")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}
