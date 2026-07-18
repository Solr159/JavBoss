package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"javboss/internal/common/logging"

	"github.com/gin-gonic/gin"
)

func registerAuthRoutes(router *gin.Engine, auth *AuthService) {
	router.GET("/auth/status", func(c *gin.Context) {
		token := requestSessionToken(c, auth.cookieName)
		authenticated, cookieTTL, renewed, err := auth.authenticateRequest(c.Request.Context(), token)
		if err != nil {
			logging.Error("renew auth session error: %v", err)
		}
		if authenticated && renewed {
			setAuthCookie(c, auth.cookieName, token, cookieTTL)
		}
		c.JSON(http.StatusOK, gin.H{"authenticated": authenticated})
	})
	router.POST("/auth/login", func(c *gin.Context) {
		var req struct {
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Password == "" || len(req.Password) > 72 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid password"})
			return
		}
		token, retryAfter, err := auth.Login(c.Request.Context(), requestClient(c.Request), req.Password)
		if err != nil {
			if errors.Is(err, errLoginLocked) {
				seconds := int(retryAfter.Seconds())
				if seconds < 1 {
					seconds = 1
				}
				c.Header("Retry-After", strconv.Itoa(seconds))
				c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many login attempts"})
				return
			}
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid password"})
			return
		}
		setAuthCookie(c, auth.cookieName, token, authSessionTTL)
		c.JSON(http.StatusOK, gin.H{"authenticated": true})
	})
	router.POST("/auth/logout", func(c *gin.Context) {
		if !requestOriginAllowed(c.Request) {
			c.JSON(http.StatusForbidden, gin.H{"error": "invalid request origin"})
			return
		}
		token := requestSessionToken(c, auth.cookieName)
		if err := auth.Logout(c.Request.Context(), token); err != nil {
			logging.Error("logout error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		clearAuthCookie(c, auth.cookieName)
		c.Status(http.StatusNoContent)
	})
}

func registerProtectedAuthRoutes(router gin.IRoutes, auth *AuthService) {
	router.PUT("/auth/password", func(c *gin.Context) {
		var req struct {
			CurrentPassword string `json:"current_password"`
			NewPassword     string `json:"new_password"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return
		}
		if len(req.CurrentPassword) == 0 || len(req.CurrentPassword) > 72 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid current password"})
			return
		}
		if !validNewPassword(req.NewPassword) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "new password must be 6-20 characters without surrounding spaces"})
			return
		}
		token := requestSessionToken(c, auth.cookieName)
		newToken, err := auth.ChangePassword(c.Request.Context(), token, req.CurrentPassword, req.NewPassword)
		if err != nil {
			if errors.Is(err, errInvalidCredentials) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid current password"})
				return
			}
			if errors.Is(err, errInvalidSession) {
				clearAuthCookie(c, auth.cookieName)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
				return
			}
			logging.Error("change password error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		setAuthCookie(c, auth.cookieName, newToken, authSessionTTL)
		c.Status(http.StatusNoContent)
	})
}

func validNewPassword(password string) bool {
	passwordLength := utf8.RuneCountInString(password)
	return passwordLength >= 6 && passwordLength <= 20 && len(password) <= 72 && strings.TrimSpace(password) == password
}
