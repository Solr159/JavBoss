package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const apiTokenHeader = "X-Pornboss-Token"

func apiTokenMiddleware(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token == "" || requestMethodIsReadOnly(c.Request.Method) {
			c.Next()
			return
		}
		if c.GetHeader(apiTokenHeader) != token {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid api token"})
			return
		}
		c.Next()
	}
}

func requestMethodIsReadOnly(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}
