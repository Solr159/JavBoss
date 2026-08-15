package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

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
