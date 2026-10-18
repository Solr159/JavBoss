package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"javboss/internal/common"
	dbpkg "javboss/internal/db"
	"javboss/internal/models"

	"github.com/gin-gonic/gin"
)

func TestJavIdolEditingWithoutSoloWorks(t *testing.T) {
	deps := newTestDependencies()
	for _, workType := range []string{"group", "none", "solo"} {
		t.Run(workType, func(t *testing.T) {
			database, err := dbpkg.Open(filepath.Join(t.TempDir(), "test.db"))
			if err != nil {
				t.Fatalf("open test database: %v", err)
			}

			deps.database = database
			deps.config = &common.Config{JavCoverDir: t.TempDir()}
			deps.bindDatabase(t)
			t.Cleanup(func() {

				if sqlDB, err := database.DB(); err == nil {
					_ = sqlDB.Close()
				}
			})
			create := func(value any) {
				t.Helper()
				if err := database.Create(value).Error; err != nil {
					t.Fatalf("create %T: %v", value, err)
				}
			}
			idol := models.JavIdol{Name: "Editable Idol"}
			create(&idol)
			var work models.Jav
			var wantWorkCount int64
			var wantDefaultCover string
			if workType != "none" {
				work = models.Jav{Code: "EDIT-001"}
				create(&work)
				create(&models.JavIdolMap{JavID: work.ID, JavIdolID: idol.ID})
				if workType == "group" {
					coIdol := models.JavIdol{Name: "Co Idol"}
					create(&coIdol)
					create(&models.JavIdolMap{JavID: work.ID, JavIdolID: coIdol.ID})
				}
				directory := models.Directory{Path: t.TempDir()}
				create(&directory)
				video := models.Video{Fingerprint: "idol-edit"}
				create(&video)
				create(&models.VideoLocation{
					VideoID: video.ID, DirectoryID: directory.ID,
					RelativePath: "edit.mp4", Filename: "edit.mp4", JavID: &work.ID,
				})
				wantWorkCount = 1
				if workType == "solo" {
					wantDefaultCover = work.Code
				}
			}

			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.GET("/jav/idols/:id", getJavIdol)
			router.PATCH("/jav/idols/:id", updateJavIdol)
			router.PUT("/jav/idols/:id/cover", updateJavIdolCover)
			path := fmt.Sprintf("/jav/idols/%d", idol.ID)
			request := func(method, path, body string, wantStatus int) dbpkg.JavIdolSummary {
				t.Helper()
				recorder := httptest.NewRecorder()
				req := httptest.NewRequest(method, path, strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(recorder, req)
				if recorder.Code != wantStatus {
					t.Fatalf("%s %s: status = %d, want %d; body=%s", method, path, recorder.Code, wantStatus, recorder.Body.String())
				}
				var item dbpkg.JavIdolSummary
				if wantStatus == http.StatusOK {
					if err := json.Unmarshal(recorder.Body.Bytes(), &item); err != nil {
						t.Fatalf("decode idol: %v", err)
					}
					if item.ID != idol.ID || item.WorkCount != wantWorkCount {
						t.Fatalf("unexpected idol summary: %#v", item)
					}
					if item.CoverJavID == nil && item.CoverCode != wantDefaultCover {
						t.Fatalf("default cover = %q, want %q", item.CoverCode, wantDefaultCover)
					}
				}
				return item
			}

			request(http.MethodGet, path, "", http.StatusOK)
			updated := request(http.MethodPatch, path,
				`{"name":"Updated Idol","chinese_name":"新名字","height_cm":165,"aliases":["Alias"]}`, http.StatusOK)
			if updated.Name != "Updated Idol" || updated.ChineseName != "新名字" ||
				updated.HeightCM == nil || *updated.HeightCM != 165 || !reflect.DeepEqual(updated.Aliases, []string{"Alias"}) {
				t.Fatalf("unexpected updated profile: %#v", updated)
			}
			if loaded := request(http.MethodGet, path, "", http.StatusOK); !reflect.DeepEqual(loaded, updated) {
				t.Fatalf("profile was not persisted: got %#v, want %#v", loaded, updated)
			}

			if workType != "none" {
				updated = request(http.MethodPut, path+"/cover",
					fmt.Sprintf(`{"jav_id":%d,"crop_left":0.25}`, work.ID), http.StatusOK)
				if updated.CoverJavID == nil || *updated.CoverJavID != work.ID ||
					updated.CoverCode != work.Code || updated.CoverCropLeft != 0.25 {
					t.Fatalf("unexpected selected cover: %#v", updated)
				}
				if loaded := request(http.MethodGet, path, "", http.StatusOK); !reflect.DeepEqual(loaded, updated) {
					t.Fatalf("cover was not persisted: got %#v, want %#v", loaded, updated)
				}
			}
			updated = request(http.MethodPut, path+"/cover", `{"jav_id":0,"crop_left":0}`, http.StatusOK)
			if updated.CoverJavID != nil || updated.CoverCropLeft != 0.53 {
				t.Fatalf("unexpected reset cover: %#v", updated)
			}
			if loaded := request(http.MethodGet, path, "", http.StatusOK); !reflect.DeepEqual(loaded, updated) {
				t.Fatalf("cover reset was not persisted: got %#v, want %#v", loaded, updated)
			}

			request(http.MethodGet, "/jav/idols/999999", "", http.StatusNotFound)
			request(http.MethodPatch, "/jav/idols/999999", `{"name":"Missing Idol"}`, http.StatusNotFound)
			request(http.MethodPut, "/jav/idols/999999/cover", `{"jav_id":0}`, http.StatusNotFound)
		})
	}
}

func TestParseJavIdolFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/jav/idols?idol_height_min=155&idol_height_max=170&idol_age_min=25&idol_age_max=35&idol_cup_min=3&idol_cup_max=7&idol_bust_min=80&idol_bust_max=95&idol_waist_min=50&idol_waist_max=65&idol_hips_min=80&idol_hips_max=100", nil)

	filters, ok := parseJavIdolFilters(context)
	if !ok {
		t.Fatalf("parseJavIdolFilters rejected valid ranges: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	assertRange := func(name string, min, max *int, wantMin, wantMax int) {
		t.Helper()
		if min == nil || max == nil || *min != wantMin || *max != wantMax {
			t.Fatalf("%s range = %v-%v, want %d-%d", name, min, max, wantMin, wantMax)
		}
	}
	assertRange("height", filters.Height.Min, filters.Height.Max, 155, 170)
	assertRange("age", filters.Age.Min, filters.Age.Max, 25, 35)
	assertRange("cup", filters.Cup.Min, filters.Cup.Max, 3, 7)
	assertRange("bust", filters.Bust.Min, filters.Bust.Max, 80, 95)
	assertRange("waist", filters.Waist.Min, filters.Waist.Max, 50, 65)
	assertRange("hips", filters.Hips.Min, filters.Hips.Max, 80, 100)
}

func TestParseJavIdolFiltersRejectsIncompleteRange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/jav/idols?idol_height_min=155", nil)

	if _, ok := parseJavIdolFilters(context); ok {
		t.Fatal("parseJavIdolFilters accepted an incomplete range")
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestParseJavIdolFiltersRejectsOutOfBoundsRange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name  string
		query string
	}{
		{name: "height", query: "idol_height_min=129&idol_height_max=170"},
		{name: "age", query: "idol_age_min=18&idol_age_max=61"},
		{name: "cup", query: "idol_cup_min=1&idol_cup_max=12"},
		{name: "bust", query: "idol_bust_min=59&idol_bust_max=90"},
		{name: "waist", query: "idol_waist_min=45&idol_waist_max=101"},
		{name: "hips", query: "idol_hips_min=64&idol_hips_max=100"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodGet, "/jav/idols?"+test.query, nil)

			if _, ok := parseJavIdolFilters(context); ok {
				t.Fatalf("parseJavIdolFilters accepted out-of-bounds %s range", test.name)
			}
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
			}
		})
	}
}
