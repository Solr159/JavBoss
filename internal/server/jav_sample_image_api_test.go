package server

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	"javboss/internal/common"
	dbpkg "javboss/internal/db"
	"javboss/internal/models"
)

func TestFC2SampleImagesRetryAndDisplay(t *testing.T) {
	var plain bytes.Buffer
	if err := jpeg.Encode(&plain, image.NewRGBA(image.Rect(0, 0, 32, 32)), nil); err != nil {
		t.Fatal(err)
	}
	encoded := make([]byte, plain.Len()+1)
	encoded[0] = 0xa8
	for i, b := range plain.Bytes() {
		encoded[i+1] = b ^ encoded[0]
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/encoded.jpg":
			w.Header().Set("Content-Type", "binary/octet-stream")
			_, _ = w.Write(encoded)
		case "/plain.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(plain.Bytes())
		case "/invalid.jpg":
			_, _ = w.Write([]byte("not an image"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	valid, err := validateJavSampleImageDetailURL(context.Background(), upstream.URL+"/encoded.jpg")
	if err != nil || !valid {
		t.Fatalf("encoded sample rejected: valid=%v err=%v", valid, err)
	}

	database, err := dbpkg.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	previousDB := common.DB
	common.DB = database
	t.Cleanup(func() {
		common.DB = previousDB
		if sqlDB, err := database.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	item := models.Jav{Code: "FC2-PPV-1234567", SampleImages: models.NewJavSampleImagesNotFound()}
	if err := database.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	want := models.JavSampleImages{
		{ThumbnailURL: upstream.URL + "/encoded.jpg", DetailURL: upstream.URL + "/plain.jpg"},
		{ThumbnailURL: upstream.URL + "/invalid.jpg", DetailURL: upstream.URL + "/missing.jpg"},
	}
	stored, err := dbpkg.SetJavSampleImagesIfEmpty(context.Background(), item.ID, want)
	if err != nil || !reflect.DeepEqual(stored, want) {
		t.Fatalf("FC2 miss was not replaced: %v, %v", stored, err)
	}
	stored, err = dbpkg.SetJavSampleImagesIfEmpty(context.Background(), item.ID, want[1:])
	if err != nil || !reflect.DeepEqual(stored, want) {
		t.Fatalf("existing images were replaced: %v, %v", stored, err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/jav/items/:id/sample-images/:index/:variant", getJavSampleImage)
	base := fmt.Sprintf("/jav/items/%d/sample-images/", item.ID)
	for _, tc := range []struct {
		suffix string
		status int
	}{
		{"0/thumbnail", http.StatusOK},
		{"0/detail", http.StatusOK},
		{"1/thumbnail", http.StatusBadGateway},
		{"1/detail", http.StatusBadGateway},
		{"10/detail", http.StatusNotFound},
		{"-1/detail", http.StatusBadRequest},
		{"0/unknown", http.StatusBadRequest},
	} {
		t.Run(tc.suffix, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, base+tc.suffix, nil))
			if recorder.Code != tc.status {
				t.Fatalf("status = %d, want %d", recorder.Code, tc.status)
			}
			if tc.status != http.StatusOK {
				return
			}
			if recorder.Header().Get("Content-Type") != "image/jpeg" || !bytes.Equal(recorder.Body.Bytes(), plain.Bytes()) {
				t.Fatal("response is not the decoded image")
			}
			if recorder.Header().Get("Cache-Control") != "private, max-age=86400" {
				t.Fatal("missing browser cache header")
			}
		})
	}
}
