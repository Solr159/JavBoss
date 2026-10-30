package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAutoSubtitleSweepRoutesAreMounted(t *testing.T) {
	deps := newTestDependencies()
	auth := testAuthService(deps, t)
	gin.SetMode(gin.TestMode)
	router := NewRouter("", auth)

	unauthenticated := httptest.NewRecorder()
	router.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodGet, "/autosub/auto/status", nil))
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want %d", unauthenticated.Code, http.StatusUnauthorized)
	}

	login := performRequest(router, http.MethodPost, "/auth/login", []byte(`{"password":"admin"}`), "http://example.com", nil)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d body = %s", login.Code, login.Body)
	}
	cookie := responseCookie(t, login, authSessionCookie)

	status := performRequest(router, http.MethodGet, "/autosub/auto/status", nil, "", cookie)
	if status.Code != http.StatusOK {
		t.Fatalf("status route returned %d: %s", status.Code, status.Body)
	}
	if body := status.Body.String(); !strings.Contains(body, `"available"`) || !strings.Contains(body, `"running"`) {
		t.Fatalf("status body = %s, want sweep status fields", body)
	}

	run := performRequest(router, http.MethodPost, "/autosub/auto/run", nil, "", cookie)
	if run.Code != http.StatusServiceUnavailable {
		t.Fatalf("run route returned %d, want %d before the sweep starts: %s", run.Code, http.StatusServiceUnavailable, run.Body)
	}
}
