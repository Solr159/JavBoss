package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"javboss/internal/common"
	dbpkg "javboss/internal/db"
	"javboss/internal/jav"
	"javboss/internal/models"

	"github.com/gin-gonic/gin"
)

func TestJavDiscoveryItemsAPIKeepsWantedInsideDiscoveredSet(t *testing.T) {
	database, err := dbpkg.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	previousDB := common.DB
	common.DB = database
	t.Cleanup(func() {
		common.DB = previousDB
		if sqlDB, dbErr := database.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	subscription := models.JavDiscoverySubscription{
		Kind:          "idol",
		Name:          "Test Idol",
		ReferenceCode: "ABC-001",
		ProviderKey:   "test-idol",
	}
	if err := dbpkg.CreateJavDiscoverySubscription(context.Background(), &subscription); err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	if err := dbpkg.UpsertJavDiscoveryItems(context.Background(), subscription.ID, []jav.JavBusDiscoveryItem{{
		Code:        "ABC-001",
		Title:       "Discovered title",
		ReleaseUnix: 100,
		Source:      "javbus",
	}}); err != nil {
		t.Fatalf("create discovery item: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router)

	all := performDiscoveryRequest(t, router, http.MethodGet, "/jav/discovery/items", nil)
	if all.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", all.Code, all.Body.String())
	}
	var listed struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
		Total int64 `json:"total"`
	}
	if err := json.Unmarshal(all.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if listed.Total != 1 || len(listed.Items) != 1 {
		t.Fatalf("unexpected list response: %+v", listed)
	}

	itemID := listed.Items[0].ID
	update := performDiscoveryRequest(
		t,
		router,
		http.MethodPatch,
		"/jav/discovery/items/"+strconv.FormatInt(itemID, 10)+"/wanted",
		[]byte(`{"wanted":true}`),
	)
	if update.Code != http.StatusNoContent {
		t.Fatalf("update status = %d body=%s", update.Code, update.Body.String())
	}

	wanted := performDiscoveryRequest(t, router, http.MethodGet, "/jav/discovery/items?wanted=1", nil)
	if wanted.Code != http.StatusOK {
		t.Fatalf("wanted status = %d body=%s", wanted.Code, wanted.Body.String())
	}
	var wantedList struct {
		Items []json.RawMessage `json:"items"`
		Total int64             `json:"total"`
	}
	if err := json.Unmarshal(wanted.Body.Bytes(), &wantedList); err != nil {
		t.Fatalf("decode wanted list: %v", err)
	}
	if wantedList.Total != 1 || len(wantedList.Items) != 1 {
		t.Fatalf("wanted list is not a subset: %+v", wantedList)
	}
}

func performDiscoveryRequest(t *testing.T, router http.Handler, method, target string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, bytes.NewReader(body))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	router.ServeHTTP(recorder, request)
	return recorder
}
