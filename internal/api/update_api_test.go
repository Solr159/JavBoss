package api

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestUpdateEndpointsRequireSessionAndSameOrigin(t *testing.T) {
	deps := newTestDependencies()
	gin.SetMode(gin.TestMode)
	router := NewRouter("", testAuthService(deps, t))
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/system/update"},
		{http.MethodPost, "/system/update/check"},
		{http.MethodPost, "/system/update/install"},
	} {
		response := performRequest(router, tc.method, tc.path, nil, "", nil)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status=%d", tc.method, tc.path, response.Code)
		}
	}
	login := performRequest(router, http.MethodPost, "/auth/login", []byte(`{"password":"admin"}`), "http://example.com", nil)
	if login.Code != http.StatusOK {
		t.Fatalf("login=%d %s", login.Code, login.Body.String())
	}
	cookie := login.Result().Cookies()[0]
	response := performRequest(router, http.MethodPost, "/system/update/install", nil, "http://attacker.example", cookie)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin update status=%d", response.Code)
	}
}
