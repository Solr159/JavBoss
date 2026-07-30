package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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
		CoverURL:    "https://pics.javbus.com/cover.jpg",
		Source:      "javbus",
	}}); err != nil {
		t.Fatalf("create discovery item: %v", err)
	}
	mainJav := models.Jav{Code: "ABC-001", Title: "Owned title"}
	directory := models.Directory{Path: "/media/owned"}
	video := models.Video{Fingerprint: "owned-discovery-video"}
	for name, value := range map[string]any{
		"main JAV":  &mainJav,
		"directory": &directory,
		"video":     &video,
	} {
		if err := database.Create(value).Error; err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}
	if err := database.Create(&models.VideoLocation{
		VideoID:      video.ID,
		DirectoryID:  directory.ID,
		RelativePath: "ABC-001.mp4",
		Filename:     "ABC-001.mp4",
		JavID:        &mainJav.ID,
	}).Error; err != nil {
		t.Fatalf("create owned video location: %v", err)
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
			ID    int64 `json:"id"`
			Owned bool  `json:"owned"`
		} `json:"items"`
		Total int64 `json:"total"`
	}
	if err := json.Unmarshal(all.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if listed.Total != 1 || len(listed.Items) != 1 {
		t.Fatalf("unexpected list response: %+v", listed)
	}
	if !listed.Items[0].Owned {
		t.Fatalf("discovery item should be marked owned: %+v", listed.Items[0])
	}

	itemID := listed.Items[0].ID
	previousHTTPDo := javDiscoveryCoverHTTPDo
	javDiscoveryCoverHTTPDo = func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://pics.javbus.com/cover.jpg" {
			t.Errorf("cover request URL = %q", request.URL.String())
		}
		if request.Header.Get("Referer") != "https://www.javbus.com/" {
			t.Errorf("cover request referer = %q", request.Header.Get("Referer"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/jpeg"}},
			Body:       io.NopCloser(bytes.NewReader([]byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10})),
		}, nil
	}
	t.Cleanup(func() { javDiscoveryCoverHTTPDo = previousHTTPDo })
	cover := performDiscoveryRequest(
		t,
		router,
		http.MethodGet,
		"/jav/discovery/items/"+strconv.FormatInt(itemID, 10)+"/cover",
		nil,
	)
	if cover.Code != http.StatusOK {
		t.Fatalf("cover status = %d body=%s", cover.Code, cover.Body.String())
	}
	if contentType := cover.Header().Get("Content-Type"); contentType != "image/jpeg" {
		t.Fatalf("cover content type = %q", contentType)
	}
	if cacheControl := cover.Header().Get("Cache-Control"); cacheControl != "private, max-age=86400" {
		t.Fatalf("cover cache control = %q", cacheControl)
	}

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

func TestIsAllowedJavDiscoveryCoverHost(t *testing.T) {
	for _, host := range []string{"www.javbus.com", "pics.javbus.com", "pics.dmm.co.jp"} {
		if !isAllowedJavDiscoveryCoverHost(host) {
			t.Fatalf("expected host %q to be allowed", host)
		}
	}
	for _, host := range []string{"javbus.com.example.org", "localhost", "127.0.0.1"} {
		if isAllowedJavDiscoveryCoverHost(host) {
			t.Fatalf("expected host %q to be rejected", host)
		}
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
