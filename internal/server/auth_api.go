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
			respondLocalizedError(c, http.StatusBadRequest, "密码格式无效", "Invalid password format")
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
				respondLocalizedError(c, http.StatusTooManyRequests, "登录尝试次数过多，请稍后再试", "Too many login attempts; please try again later")
				return
			}
			respondLocalizedError(c, http.StatusUnauthorized, "密码错误", "Incorrect password")
			return
		}
		setAuthCookie(c, auth.cookieName, token, authSessionTTL)
		c.JSON(http.StatusOK, gin.H{"authenticated": true})
	})
	router.POST("/auth/logout", func(c *gin.Context) {
		if !requestOriginAllowed(c.Request) {
			respondLocalizedError(c, http.StatusForbidden, "请求来源无效", "Invalid request origin")
			return
		}
		token := requestSessionToken(c, auth.cookieName)
		if err := auth.Logout(c.Request.Context(), token); err != nil {
			logging.Error("logout error: %v", err)
			respondLocalizedError(c, http.StatusInternalServerError, "退出登录失败", "Failed to sign out")
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
			respondLocalizedError(c, http.StatusBadRequest, "修改密码请求无效", "Invalid password change request")
			return
		}
		if len(req.CurrentPassword) == 0 || len(req.CurrentPassword) > 72 {
			respondLocalizedError(c, http.StatusBadRequest, "当前密码格式无效", "Invalid current password format")
			return
		}
		if !validNewPassword(req.NewPassword) {
			respondLocalizedError(c, http.StatusUnprocessableEntity, "新密码需为 6-20 个字符，且首尾不能有空格", "New password must be 6-20 characters without surrounding spaces")
			return
		}
		token := requestSessionToken(c, auth.cookieName)
		newToken, err := auth.ChangePassword(c.Request.Context(), token, req.CurrentPassword, req.NewPassword)
		if err != nil {
			if errors.Is(err, errInvalidCredentials) {
				respondLocalizedError(c, http.StatusBadRequest, "当前密码错误", "Current password is incorrect")
				return
			}
			if errors.Is(err, errInvalidSession) {
				clearAuthCookie(c, auth.cookieName)
				respondLocalizedError(c, http.StatusUnauthorized, "登录状态已失效，请重新登录", "Your session has expired; please sign in again")
				return
			}
			logging.Error("change password error: %v", err)
			respondLocalizedError(c, http.StatusInternalServerError, "修改密码失败", "Failed to change password")
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
