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
	"time"

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
		Kind:            "idol",
		Name:            "Test Idol",
		ReferenceCode:   "ABC-001",
		ProviderLocator: "/star/test-idol",
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

	type discoveryListPayload struct {
		Items []struct {
			ID    int64 `json:"id"`
			Owned bool  `json:"owned"`
		} `json:"items"`
		Total int64 `json:"total"`
	}
	hidden := performDiscoveryRequest(t, router, http.MethodGet, "/jav/discovery/items", nil)
	if hidden.Code != http.StatusOK {
		t.Fatalf("default list status = %d body=%s", hidden.Code, hidden.Body.String())
	}
	var hiddenList discoveryListPayload
	if err := json.Unmarshal(hidden.Body.Bytes(), &hiddenList); err != nil {
		t.Fatalf("decode default list: %v", err)
	}
	if hiddenList.Total != 0 || len(hiddenList.Items) != 0 {
		t.Fatalf("owned item should be hidden by default: %+v", hiddenList)
	}

	all := performDiscoveryRequest(t, router, http.MethodGet, "/jav/discovery/items?include_owned=1", nil)
	if all.Code != http.StatusOK {
		t.Fatalf("included list status = %d body=%s", all.Code, all.Body.String())
	}
	var listed discoveryListPayload
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
	requestedCoverURLs := make([]string, 0, 3)
	javDiscoveryCoverHTTPDo = func(request *http.Request) (*http.Response, error) {
		requestedCoverURLs = append(requestedCoverURLs, request.URL.String())
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
	if len(requestedCoverURLs) != 1 || requestedCoverURLs[0] != "https://pics.javbus.com/cover.jpg" {
		t.Fatalf("initial cover request URLs = %#v", requestedCoverURLs)
	}

	detailsFetchedAt := time.Now().UTC()
	detailCalls := 0
	previousDetailsFetch := fetchJavBusDetails
	fetchJavBusDetails = func(_ context.Context, code string) (*jav.JavBusDiscoveryItem, error) {
		detailCalls++
		if code != "ABC-001" {
			t.Errorf("detail code = %q", code)
		}
		return &jav.JavBusDiscoveryItem{
			Code:             code,
			Title:            "Full JavBus title",
			ReleaseUnix:      200,
			DurationMin:      123,
			CoverURL:         "https://pics.javbus.com/full-cover.jpg",
			DetailURL:        "https://www.javbus.com/ABC-001",
			Actresses:        []string{"Test Idol"},
			Studio:           "Test Studio",
			Series:           "Test Series",
			Tags:             []string{"Tag A", "Tag B"},
			Source:           "javbus",
			DetailsFetchedAt: &detailsFetchedAt,
			MagnetLinks: []jav.JavBusMagnetLink{{
				Name: "ABC-001-HD", URL: "magnet:?xt=urn:btih:ABC123&dn=ABC-001-HD", Size: "4.2GB",
			}},
		}, nil
	}
	t.Cleanup(func() { fetchJavBusDetails = previousDetailsFetch })
	for attempt := 0; attempt < 2; attempt++ {
		details := performDiscoveryRequest(
			t,
			router,
			http.MethodPost,
			"/jav/discovery/items/"+strconv.FormatInt(itemID, 10)+"/details",
			nil,
		)
		if details.Code != http.StatusOK {
			t.Fatalf("details status = %d body=%s", details.Code, details.Body.String())
		}
		var payload struct {
			Metadata struct {
				Title  string   `json:"title"`
				Studio string   `json:"studio"`
				Series string   `json:"series"`
				Tags   []string `json:"tags"`
			} `json:"metadata"`
			MagnetLinks []jav.JavBusMagnetLink `json:"magnet_links"`
		}
		if err := json.Unmarshal(details.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode details: %v", err)
		}
		if payload.Metadata.Title != "Full JavBus title" ||
			payload.Metadata.Studio != "Test Studio" ||
			payload.Metadata.Series != "Test Series" ||
			len(payload.Metadata.Tags) != 2 {
			t.Fatalf("unexpected details payload: %+v", payload)
		}
		if len(payload.MagnetLinks) != 1 || payload.MagnetLinks[0].Name != "ABC-001-HD" {
			t.Fatalf("unexpected magnet links payload: %+v", payload.MagnetLinks)
		}
	}
	if detailCalls != 1 {
		t.Fatalf("detail fetch calls = %d, want cached second response", detailCalls)
	}
	for _, path := range []string{"thumbnail", "cover"} {
		image := performDiscoveryRequest(
			t,
			router,
			http.MethodGet,
			"/jav/discovery/items/"+strconv.FormatInt(itemID, 10)+"/"+path,
			nil,
		)
		if image.Code != http.StatusOK {
			t.Fatalf("%s status = %d body=%s", path, image.Code, image.Body.String())
		}
	}
	wantCoverURLs := []string{
		"https://pics.javbus.com/cover.jpg",
		"https://pics.javbus.com/cover.jpg",
		"https://pics.javbus.com/full-cover.jpg",
	}
	if len(requestedCoverURLs) != len(wantCoverURLs) {
		t.Fatalf("cover request URLs = %#v, want %#v", requestedCoverURLs, wantCoverURLs)
	}
	for index := range wantCoverURLs {
		if requestedCoverURLs[index] != wantCoverURLs[index] {
			t.Fatalf("cover request URLs = %#v, want %#v", requestedCoverURLs, wantCoverURLs)
		}
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

	wanted := performDiscoveryRequest(t, router, http.MethodGet, "/jav/discovery/items?wanted=1&include_owned=1", nil)
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

func TestCreateJavDiscoverySubscriptionRequiresOnlyReferenceCode(t *testing.T) {
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

	previousResolver := resolveJavBusActressSubscription
	resolveJavBusActressSubscription = func(_ context.Context, code string) (*jav.JavBusActressSubscription, error) {
		if code != "ABC-001" {
			t.Errorf("reference code = %q", code)
		}
		return &jav.JavBusActressSubscription{Name: "葵つかさ", ProviderLocator: "/uncensored/star/abc123"}, nil
	}
	t.Cleanup(func() {
		resolveJavBusActressSubscription = previousResolver
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router)
	response := performDiscoveryRequest(
		t,
		router,
		http.MethodPost,
		"/jav/discovery/subscriptions",
		[]byte(`{"reference_code":"abc-001"}`),
	)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", response.Code, response.Body.String())
	}
	var subscription models.JavDiscoverySubscription
	if err := json.Unmarshal(response.Body.Bytes(), &subscription); err != nil {
		t.Fatalf("decode subscription: %v", err)
	}
	if subscription.Name != "葵つかさ" ||
		subscription.ReferenceCode != "ABC-001" ||
		subscription.ProviderLocator != "" {
		t.Fatalf("created subscription = %#v", subscription)
	}
	stored, err := dbpkg.ListJavDiscoverySubscriptions(context.Background())
	if err != nil {
		t.Fatalf("list subscriptions: %v", err)
	}
	if len(stored) != 1 || stored[0].Name != "葵つかさ" || stored[0].ProviderLocator != "/uncensored/star/abc123" {
		t.Fatalf("stored subscriptions = %#v", stored)
	}
}

func TestLoadMoreJavDiscoverySubscriptionHistory(t *testing.T) {
	previousLoader := loadMoreJavDiscoveryHistory
	loadMoreJavDiscoveryHistory = func(_ context.Context, subscriptionID int64) (int, error) {
		if subscriptionID != 42 {
			t.Errorf("subscription id = %d, want 42", subscriptionID)
		}
		return 10, nil
	}
	t.Cleanup(func() { loadMoreJavDiscoveryHistory = previousLoader })

	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router)
	response := performDiscoveryRequest(
		t,
		router,
		http.MethodPost,
		"/jav/discovery/subscriptions/42/history",
		nil,
	)
	if response.Code != http.StatusOK {
		t.Fatalf("load history status = %d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Loaded int `json:"loaded"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode load history response: %v", err)
	}
	if payload.Loaded != 10 {
		t.Fatalf("loaded = %d, want 10", payload.Loaded)
	}
}

func TestJavDiscoveryItemsAPIFiltersBySubscription(t *testing.T) {
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

	first := models.JavDiscoverySubscription{
		Kind: "idol", Name: "葵つかさ", ReferenceCode: "ABP-001", ProviderLocator: "/star/star-a",
	}
	second := models.JavDiscoverySubscription{
		Kind: "idol", Name: "相沢みなみ", ReferenceCode: "IPX-001", ProviderLocator: "/star/star-b",
	}
	for name, subscription := range map[string]*models.JavDiscoverySubscription{
		"first": &first, "second": &second,
	} {
		if err := dbpkg.CreateJavDiscoverySubscription(context.Background(), subscription); err != nil {
			t.Fatalf("create %s subscription: %v", name, err)
		}
	}
	if err := dbpkg.UpsertJavDiscoveryItems(context.Background(), first.ID, []jav.JavBusDiscoveryItem{{
		Code: "ABP-001", Title: "First work", Source: "javbus",
	}}); err != nil {
		t.Fatalf("upsert first item: %v", err)
	}
	if err := dbpkg.UpsertJavDiscoveryItems(context.Background(), second.ID, []jav.JavBusDiscoveryItem{{
		Code: "IPX-001", Title: "Second work", Source: "javbus",
	}}); err != nil {
		t.Fatalf("upsert second item: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router)
	response := performDiscoveryRequest(
		t,
		router,
		http.MethodGet,
		"/jav/discovery/items?subscription_id="+strconv.FormatInt(first.ID, 10),
		nil,
	)
	if response.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Items []struct {
			Code string `json:"code"`
		} `json:"items"`
		Total int64 `json:"total"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode filtered items: %v", err)
	}
	if payload.Total != 1 || len(payload.Items) != 1 || payload.Items[0].Code != "ABP-001" {
		t.Fatalf("filtered payload = %#v", payload)
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
