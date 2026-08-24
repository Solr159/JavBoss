package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"javboss/internal/common"
	dbpkg "javboss/internal/db"
	"javboss/internal/jav"
	"javboss/internal/models"

	"github.com/gin-gonic/gin"
)

func TestCreateJavEditOptionsReturnsPersistentIDs(t *testing.T) {
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

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/jav/idols", createJavIdol)
	router.POST("/jav/tags/scraped", createJavScrapedTag)
	router.GET("/jav/tags", listJavTags)

	requestJSON := func(method, path, body string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)
		return recorder
	}

	idolResponse := requestJSON(http.MethodPost, "/jav/idols", `{"name":"即時建立女优"}`)
	if idolResponse.Code != http.StatusCreated {
		t.Fatalf("create idol status = %d body=%s", idolResponse.Code, idolResponse.Body.String())
	}
	var idol dbpkg.JavIdolSummary
	if err := json.Unmarshal(idolResponse.Body.Bytes(), &idol); err != nil {
		t.Fatalf("decode idol response: %v", err)
	}
	if idol.ID <= 0 || idol.Name != "即時建立女优" {
		t.Fatalf("created idol = %#v", idol)
	}

	duplicateResponse := requestJSON(http.MethodPost, "/jav/idols", `{"name":"即時建立女优"}`)
	var duplicate dbpkg.JavIdolSummary
	if err := json.Unmarshal(duplicateResponse.Body.Bytes(), &duplicate); err != nil {
		t.Fatalf("decode duplicate idol response: %v", err)
	}
	if duplicateResponse.Code != http.StatusCreated || duplicate.ID != idol.ID {
		t.Fatalf("duplicate idol = %#v status=%d, want id %d", duplicate, duplicateResponse.Code, idol.ID)
	}

	tagResponse := requestJSON(http.MethodPost, "/jav/tags/scraped", `{"name":"无码"}`)
	if tagResponse.Code != http.StatusCreated {
		t.Fatalf("create scraped tag status = %d body=%s", tagResponse.Code, tagResponse.Body.String())
	}
	var tag dbpkg.JavTagCount
	if err := json.Unmarshal(tagResponse.Body.Bytes(), &tag); err != nil {
		t.Fatalf("decode scraped tag response: %v", err)
	}
	if tag.ID <= 0 || tag.Name != "無碼" || tag.Provider != int(jav.ProviderManualScrape) {
		t.Fatalf("created scraped tag = %#v", tag)
	}

	listResponse := requestJSON(http.MethodGet, "/jav/tags", "")
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list tags status = %d body=%s", listResponse.Code, listResponse.Body.String())
	}
	var tags []dbpkg.JavTagCount
	if err := json.Unmarshal(listResponse.Body.Bytes(), &tags); err != nil {
		t.Fatalf("decode tags response: %v", err)
	}
	if len(tags) != 1 || tags[0].ID != tag.ID || tags[0].Count != 0 {
		t.Fatalf("listed tags = %#v, want zero-count created tag %#v", tags, tag)
	}
}

func acceptJavSampleImageURL(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func TestLookupJavSampleImagesByProviderFallsBackFromJavMenuToJavBus(t *testing.T) {
	var calls []jav.Provider
	images, err := lookupJavSampleImagesByProvider(context.Background(), "IPX-228", func(code string, provider jav.Provider) (*jav.JavInfo, error) {
		if code != "IPX-228" {
			t.Fatalf("unexpected code: %q", code)
		}
		calls = append(calls, provider)
		switch provider {
		case jav.ProviderJavMenu:
			return &jav.JavInfo{Code: code, Provider: provider}, nil
		case jav.ProviderJavBus:
			return &jav.JavInfo{
				Code:     code,
				Provider: provider,
				SampleImages: []jav.SampleImage{
					{
						ThumbnailURL: "https://example.com/thumb.jpg",
						DetailURL:    "https://example.com/detail.jpg",
					},
				},
			}, nil
		default:
			t.Fatalf("unexpected provider: %s", provider.String())
			return nil, nil
		}
	}, acceptJavSampleImageURL)
	if err != nil {
		t.Fatalf("lookup sample images: %v", err)
	}
	if !reflect.DeepEqual(calls, []jav.Provider{jav.ProviderJavMenu, jav.ProviderJavBus}) {
		t.Fatalf("provider calls = %#v", calls)
	}
	want := models.JavSampleImages{
		{
			ThumbnailURL: "https://example.com/thumb.jpg",
			DetailURL:    "https://example.com/detail.jpg",
		},
	}
	if !reflect.DeepEqual(images, want) {
		t.Fatalf("sample images = %#v, want %#v", images, want)
	}
}

func TestLookupJavSampleImagesByProviderStopsAfterJavMenuSuccess(t *testing.T) {
	var calls []jav.Provider
	images, err := lookupJavSampleImagesByProvider(context.Background(), "IPX-228", func(_ string, provider jav.Provider) (*jav.JavInfo, error) {
		calls = append(calls, provider)
		if provider != jav.ProviderJavMenu {
			return nil, errors.New("JavBus must not be called after JavMenu succeeds")
		}
		return &jav.JavInfo{
			SampleImages: []jav.SampleImage{
				{ThumbnailURL: "thumbnail", DetailURL: "detail"},
			},
		}, nil
	}, acceptJavSampleImageURL)
	if err != nil {
		t.Fatalf("lookup sample images: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("sample image count = %d, want 1", len(images))
	}
	if !reflect.DeepEqual(calls, []jav.Provider{jav.ProviderJavMenu}) {
		t.Fatalf("provider calls = %#v", calls)
	}
}

func TestLookupJavSampleImagesByProviderPreservesTemporaryErrors(t *testing.T) {
	temporaryErr := errors.New("network timeout")
	images, err := lookupJavSampleImagesByProvider(context.Background(), "IPX-228", func(_ string, provider jav.Provider) (*jav.JavInfo, error) {
		switch provider {
		case jav.ProviderJavMenu:
			return nil, temporaryErr
		case jav.ProviderJavBus:
			return nil, jav.ResourceNotFonud
		default:
			t.Fatalf("unexpected provider: %s", provider.String())
			return nil, nil
		}
	}, acceptJavSampleImageURL)
	if len(images) != 0 {
		t.Fatalf("sample image count = %d, want 0", len(images))
	}
	if !errors.Is(err, temporaryErr) {
		t.Fatalf("lookup error = %v, want network timeout", err)
	}
}

func TestLookupJavSampleImagesByProviderTreatsConfirmedMissAsNotFound(t *testing.T) {
	images, err := lookupJavSampleImagesByProvider(context.Background(), "IPX-228", func(_ string, _ jav.Provider) (*jav.JavInfo, error) {
		return nil, jav.ResourceNotFonud
	}, acceptJavSampleImageURL)
	if err != nil {
		t.Fatalf("confirmed miss returned error: %v", err)
	}
	if len(images) != 0 {
		t.Fatalf("sample image count = %d, want 0", len(images))
	}
}

func TestLookupJavSampleImagesByProviderValidatesLastDetailURLAndFallsBack(t *testing.T) {
	var calls []jav.Provider
	var validated []string
	images, err := lookupJavSampleImagesByProvider(
		context.Background(),
		"IPX-228",
		func(_ string, provider jav.Provider) (*jav.JavInfo, error) {
			calls = append(calls, provider)
			return &jav.JavInfo{SampleImages: []jav.SampleImage{
				{ThumbnailURL: "thumb-1", DetailURL: provider.String() + "-detail-1"},
				{ThumbnailURL: "thumb-2", DetailURL: provider.String() + "-detail-10"},
			}}, nil
		},
		func(_ context.Context, detailURL string) (bool, error) {
			validated = append(validated, detailURL)
			return strings.HasPrefix(detailURL, "javbus-"), nil
		},
	)
	if err != nil {
		t.Fatalf("lookup sample images: %v", err)
	}
	if !reflect.DeepEqual(calls, []jav.Provider{jav.ProviderJavMenu, jav.ProviderJavBus}) {
		t.Fatalf("provider calls = %#v", calls)
	}
	if !reflect.DeepEqual(validated, []string{"javmenu-detail-10", "javbus-detail-10"}) {
		t.Fatalf("validated URLs = %#v", validated)
	}
	if len(images) != 2 || images[1].DetailURL != "javbus-detail-10" {
		t.Fatalf("sample images = %#v", images)
	}
}

func TestValidateJavSampleImageDetailURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/valid.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00})
		case "/invalid.jpg":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("not an image"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	tests := []struct {
		name string
		url  string
		want bool
	}{
		{name: "valid image", url: server.URL + "/valid.jpg", want: true},
		{name: "HTML response", url: server.URL + "/invalid.jpg", want: false},
		{name: "missing image", url: server.URL + "/missing.jpg", want: false},
		{name: "invalid URL", url: "javascript:alert(1)", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := validateJavSampleImageDetailURL(context.Background(), test.url)
			if err != nil {
				t.Fatalf("validate detail URL: %v", err)
			}
			if got != test.want {
				t.Fatalf("valid = %v, want %v", got, test.want)
			}
		})
	}
}
