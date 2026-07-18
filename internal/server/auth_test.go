package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"javboss/internal/common"
	dbpkg "javboss/internal/db"
	"javboss/internal/models"

	"github.com/gin-gonic/gin"
)

func TestAuthLoginProtectsRoutesAndLogoutRevokesSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := testAuthService(t)
	router := gin.New()
	registerAuthRoutes(router, auth)
	protected := router.Group("/")
	protected.Use(auth.requireAuth())
	protected.GET("/private", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	protected.POST("/private", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	unauthorized := performRequest(router, http.MethodGet, "/private", nil, "", nil)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want %d", unauthorized.Code, http.StatusUnauthorized)
	}

	login := performRequest(router, http.MethodPost, "/auth/login", []byte(`{"password":"admin"}`), "http://frontend.example:5173", nil)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d body=%s", login.Code, login.Body.String())
	}
	cookie := responseCookie(t, login, authSessionCookie)
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.MaxAge <= 0 {
		t.Fatalf("unexpected auth cookie: %#v", cookie)
	}

	allowed := performRequest(router, http.MethodGet, "/private", nil, "", cookie)
	if allowed.Code != http.StatusNoContent {
		t.Fatalf("authenticated status = %d, want %d", allowed.Code, http.StatusNoContent)
	}
	crossOrigin := performRequest(router, http.MethodPost, "/private", nil, "http://attacker.example", cookie)
	if crossOrigin.Code != http.StatusForbidden {
		t.Fatalf("cross-origin status = %d, want %d", crossOrigin.Code, http.StatusForbidden)
	}

	logout := performRequest(router, http.MethodPost, "/auth/logout", nil, "http://example.com", cookie)
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want %d", logout.Code, http.StatusNoContent)
	}
	revoked := performRequest(router, http.MethodGet, "/private", nil, "", cookie)
	if revoked.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session status = %d, want %d", revoked.Code, http.StatusUnauthorized)
	}
	restarted, err := NewAuthService(context.Background())
	if err != nil {
		t.Fatalf("restart auth service after logout: %v", err)
	}
	if restarted.Authenticated(cookie.Value) {
		t.Fatal("logged-out session was restored after restart")
	}
}

func TestNewRouterAuthenticationBoundary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter("", testAuthService(t))

	health := performRequest(router, http.MethodGet, "/healthz", nil, "", nil)
	if health.Code != http.StatusOK {
		t.Fatalf("health status = %d, want %d", health.Code, http.StatusOK)
	}
	status := performRequest(router, http.MethodGet, "/auth/status", nil, "", nil)
	if status.Code != http.StatusOK {
		t.Fatalf("auth status endpoint = %d, want %d", status.Code, http.StatusOK)
	}
	videos := performRequest(router, http.MethodGet, "/videos", nil, "", nil)
	if videos.Code != http.StatusUnauthorized {
		t.Fatalf("videos status = %d, want %d", videos.Code, http.StatusUnauthorized)
	}
}

func TestAuthCookieNameIsStableAndInstallationSpecific(t *testing.T) {
	root := t.TempDir()
	firstDir := filepath.Join(root, "first")
	secondDir := filepath.Join(root, "second")
	for _, dir := range []string{firstDir, secondDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create instance directory: %v", err)
		}
	}

	first, err := authCookieNameForInstance(firstDir)
	if err != nil {
		t.Fatalf("first cookie name: %v", err)
	}
	firstAgain, err := authCookieNameForInstance(filepath.Join(firstDir, "."))
	if err != nil {
		t.Fatalf("stable cookie name: %v", err)
	}
	second, err := authCookieNameForInstance(secondDir)
	if err != nil {
		t.Fatalf("second cookie name: %v", err)
	}
	if first != firstAgain {
		t.Fatalf("same installation produced different cookie names: %q != %q", first, firstAgain)
	}
	if first == second {
		t.Fatalf("different installations produced the same cookie name: %q", first)
	}
	if !strings.HasPrefix(first, authSessionCookie+"_") || strings.Contains(first, root) {
		t.Fatalf("cookie name does not use a private hash suffix: %q", first)
	}
}

func TestAuthenticatedRequestRenewsSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := testAuthService(t)
	now := time.Now().UTC().Truncate(time.Second)
	auth.now = func() time.Time { return now }
	token, _, err := auth.Login(context.Background(), "renewal", "admin")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	key, ok := sessionKey(token)
	if !ok {
		t.Fatal("generated an invalid session token")
	}
	initialExpiry := auth.sessions[key].expiresAt

	router := gin.New()
	registerAuthRoutes(router, auth)
	cookie := &http.Cookie{Name: auth.cookieName, Value: token, Path: "/"}

	now = now.Add(authRenewInterval - time.Hour)
	beforeThreshold := performRequest(router, http.MethodGet, "/auth/status", nil, "", cookie)
	beforeThresholdCookie := responseCookie(t, beforeThreshold, auth.cookieName)
	if beforeThresholdCookie.MaxAge != int(authSessionTTL.Seconds()) {
		t.Fatalf("sliding cookie MaxAge = %d, want %d", beforeThresholdCookie.MaxAge, int(authSessionTTL.Seconds()))
	}
	if !auth.sessions[key].expiresAt.Equal(now.Add(authSessionTTL)) {
		t.Fatal("in-memory session expiry was not renewed on request")
	}
	var beforePersist models.AuthSession
	if err := common.DB.First(&beforePersist, "token_hash = ?", sessionHash(key)).Error; err != nil {
		t.Fatalf("load session before persisted renewal: %v", err)
	}
	if !beforePersist.ExpiresAt.Equal(initialExpiry) {
		t.Fatal("session was persisted before the renewal interval")
	}

	now = now.Add(2 * time.Hour)
	renewedResponse := performRequest(router, http.MethodGet, "/auth/status", nil, "", cookie)
	renewedCookie := responseCookie(t, renewedResponse, auth.cookieName)
	if renewedCookie.MaxAge != int(authSessionTTL.Seconds()) {
		t.Fatalf("renewed cookie MaxAge = %d, want %d", renewedCookie.MaxAge, int(authSessionTTL.Seconds()))
	}
	wantExpiry := now.Add(authSessionTTL)
	if !auth.sessions[key].expiresAt.Equal(wantExpiry) {
		t.Fatalf("in-memory expiry = %s, want %s", auth.sessions[key].expiresAt, wantExpiry)
	}

	var persisted models.AuthSession
	if err := common.DB.First(&persisted, "token_hash = ?", sessionHash(key)).Error; err != nil {
		t.Fatalf("load renewed session: %v", err)
	}
	if !persisted.ExpiresAt.Equal(wantExpiry) {
		t.Fatalf("persisted expiry = %s, want %s", persisted.ExpiresAt, wantExpiry)
	}
}

func TestDefaultPasswordAndPasswordChangePersist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database, err := dbpkg.Open(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("database handle: %v", err)
	}
	defer sqlDB.Close()
	previousDB := common.DB
	common.DB = database
	defer func() { common.DB = previousDB }()

	auth, err := NewAuthService(context.Background())
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}
	token, _, err := auth.Login(context.Background(), "test", "admin")
	if err != nil {
		t.Fatalf("default password login: %v", err)
	}
	var persistedHash string
	if err := database.Table("auth_session").Select("token_hash").Scan(&persistedHash).Error; err != nil {
		t.Fatalf("load persisted session hash: %v", err)
	}
	if len(persistedHash) != 64 || persistedHash == token {
		t.Fatalf("session was not stored as a SHA-256 hash: %q", persistedHash)
	}
	restarted, err := NewAuthService(context.Background())
	if err != nil {
		t.Fatalf("restart auth service: %v", err)
	}
	if !restarted.Authenticated(token) {
		t.Fatal("session was not restored after restart")
	}
	newToken, err := restarted.ChangePassword(context.Background(), token, "admin", "new-password")
	if err != nil {
		t.Fatalf("change password: %v", err)
	}
	if restarted.Authenticated(token) {
		t.Fatal("old session remained authenticated after password change")
	}
	if !restarted.Authenticated(newToken) {
		t.Fatal("replacement session is not authenticated")
	}

	reloaded, err := NewAuthService(context.Background())
	if err != nil {
		t.Fatalf("reload auth service: %v", err)
	}
	if reloaded.Authenticated(token) {
		t.Fatal("revoked session was restored after password change")
	}
	if !reloaded.Authenticated(newToken) {
		t.Fatal("replacement session was not restored after restart")
	}
	if _, _, err := reloaded.Login(context.Background(), "old", "admin"); err == nil {
		t.Fatal("old password still works")
	}
	if _, _, err := reloaded.Login(context.Background(), "new", "new-password"); err != nil {
		t.Fatalf("new password login: %v", err)
	}
}

func TestAuthLoginRateLimit(t *testing.T) {
	auth := testAuthService(t)
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	auth.now = func() time.Time { return now }
	for attempt := 1; attempt < maxLoginFailures; attempt++ {
		if _, _, err := auth.Login(context.Background(), "client", "wrong"); err != errInvalidCredentials {
			t.Fatalf("attempt %d error = %v, want invalid credentials", attempt, err)
		}
	}
	if _, retryAfter, err := auth.Login(context.Background(), "client", "wrong"); err != errLoginLocked || retryAfter != loginLockout {
		t.Fatalf("lockout = (%s, %v), want (%s, %v)", retryAfter, err, loginLockout, errLoginLocked)
	}
	if _, _, err := auth.Login(context.Background(), "client", "admin"); err != errLoginLocked {
		t.Fatalf("correct password during lockout error = %v, want %v", err, errLoginLocked)
	}
	now = now.Add(loginLockout)
	if _, _, err := auth.Login(context.Background(), "client", "admin"); err != nil {
		t.Fatalf("login after lockout: %v", err)
	}
}

func TestRequestOriginAllowedBehindReverseProxy(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://backend.internal/auth/login", nil)
	req.Host = "backend.internal"
	req.Header.Set("Origin", "https://media.example")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "media.example")
	if !requestOriginAllowed(req) {
		t.Fatal("matching forwarded origin was rejected")
	}
	req.Header.Set("Origin", "https://attacker.example")
	if requestOriginAllowed(req) {
		t.Fatal("cross-origin request was allowed")
	}
}

func TestRequestOriginAllowedUsesFetchMetadata(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://backend.internal/private", nil)
	req.Host = "backend.internal"
	req.Header.Set("Origin", "http://frontend.local:5173")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	if !requestOriginAllowed(req) {
		t.Fatal("same-origin fetch metadata was rejected after proxy host rewrite")
	}
	req.Header.Set("Origin", "http://backend.internal")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	if requestOriginAllowed(req) {
		t.Fatal("cross-site fetch metadata was allowed")
	}
}

func TestValidNewPasswordLength(t *testing.T) {
	tests := []struct {
		name     string
		password string
		want     bool
	}{
		{name: "five characters", password: "abcde", want: false},
		{name: "six characters", password: "abcdef", want: true},
		{name: "twenty characters", password: "12345678901234567890", want: true},
		{name: "twenty one characters", password: "123456789012345678901", want: false},
		{name: "unicode characters", password: "密码安全测试", want: true},
		{name: "surrounding space", password: " abcdef", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validNewPassword(tt.password); got != tt.want {
				t.Fatalf("validNewPassword(%q) = %v, want %v", tt.password, got, tt.want)
			}
		})
	}
}

func testAuthService(t *testing.T) *AuthService {
	t.Helper()
	database, err := dbpkg.Open(filepath.Join(t.TempDir(), "auth-service.db"))
	if err != nil {
		t.Fatalf("open auth service database: %v", err)
	}
	previousDB := common.DB
	common.DB = database
	t.Cleanup(func() {
		common.DB = previousDB
		if sqlDB, dbErr := database.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	auth, err := NewAuthService(context.Background())
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}
	return auth
}

func performRequest(handler http.Handler, method, path string, body []byte, origin string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Host = "example.com"
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func responseCookie(t *testing.T, recorder *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("cookie %q not found", name)
	return nil
}
