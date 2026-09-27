package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"javboss/internal/jav"
)

func TestProviderConnectivityRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router)
	list := httptest.NewRecorder()
	router.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/jav/providers", nil))
	var providers []jav.ConnectivityProvider
	if err := json.Unmarshal(list.Body.Bytes(), &providers); err != nil || list.Code != http.StatusOK || len(providers) != 10 {
		t.Fatalf("provider list: status=%d body=%s err=%v", list.Code, list.Body, err)
	}
	for _, id := range []string{"0", "3", "11", "999", "invalid"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/jav/providers/"+id+"/connectivity", nil))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("invalid provider %s: status=%d", id, recorder.Code)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/jav/providers/1/connectivity", nil).WithContext(ctx))
	var result jav.ConnectivityResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil || recorder.Code != http.StatusOK || result.Status != "canceled" {
		t.Fatalf("canceled probe: status=%d body=%s err=%v", recorder.Code, recorder.Body, err)
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("connectivity responses must not be cached")
	}
}

func TestProviderConnectivityRequiresAuthentication(t *testing.T) {
	router := NewRouter("", testAuthService(t))
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/jav/providers"},
		{http.MethodPost, "/jav/providers/1/connectivity"},
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(route.method, route.path, nil))
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: status=%d", route.method, route.path, recorder.Code)
		}
	}
}
