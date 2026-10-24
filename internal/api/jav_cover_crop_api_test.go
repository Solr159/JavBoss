package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	dbpkg "javboss/internal/db"
	"javboss/internal/models"
)

func TestUpdateJavCoverCrop(t *testing.T) {
	deps := newTestDependencies()
	database, err := dbpkg.Open(filepath.Join(t.TempDir(), "crop.db"))
	if err != nil {
		t.Fatal(err)
	}
	deps.database = database
	deps.bindDatabase(t)
	t.Cleanup(func() {
		if sqlDB, err := database.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	item := models.Jav{Code: "CROP-API", Title: "Original", DurationMin: 120}
	if err := database.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/jav/items/:id", updateJavItem)
	for _, tc := range []struct {
		body     string
		status   int
		position float64
	}{
		{`{"cover_crop_position":0.25}`, http.StatusOK, 0.25},
		{`{"cover_crop_position":0}`, http.StatusOK, 0},
		{`{"cover_crop_position":-0.1}`, http.StatusBadRequest, 0},
		{`{"cover_crop_position":1.1}`, http.StatusBadRequest, 0},
		{`{}`, http.StatusOK, 0},
		{`{"cover_crop_position":1}`, http.StatusOK, 1},
	} {
		t.Run(tc.body, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/jav/items/%d", item.ID), strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)
			if recorder.Code != tc.status {
				t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
			}
			if tc.status == http.StatusOK {
				var response models.Jav
				if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				if response.CoverCropPosition != tc.position {
					t.Fatalf("response position = %v", response.CoverCropPosition)
				}
			}
			var stored models.Jav
			if err := database.First(&stored, item.ID).Error; err != nil {
				t.Fatal(err)
			}
			if stored.CoverCropPosition != tc.position || stored.Title != item.Title || stored.DurationMin != 120 {
				t.Fatalf("unexpected stored item: %#v", stored)
			}
		})
	}
}
