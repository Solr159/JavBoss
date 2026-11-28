package api

import (
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParseJavActorCounts(t *testing.T) {
	for _, tc := range []struct {
		query string
		want  []int64
		valid bool
	}{
		{"", nil, true},
		{"actor_counts=1,3,5", []int64{1, 3, 5}, true},
		{"actor_counts=2,4&solo=1", []int64{2, 4}, true},
		{"actor_counts=0", []int64{0}, true},
		{"actor_counts=0,2", []int64{0, 2}, true},
		{"actor_counts=-1", nil, false},
		{"actor_counts=6", nil, false},
		{"actor_counts=1.5", nil, false},
		{"actor_counts=one", nil, false},
		{"actor_counts=1,", nil, false},
	} {
		t.Run(tc.query, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest("GET", "/jav?"+tc.query, nil)
			query, valid := parseJavFilterQuery(ctx)
			if valid != tc.valid {
				t.Fatalf("valid=%v, want %v", valid, tc.valid)
			}
			if valid && !reflect.DeepEqual(query.ActorCounts, tc.want) {
				t.Fatalf("counts=%v, want %v", query.ActorCounts, tc.want)
			}
			if !valid && recorder.Code != 400 {
				t.Fatalf("status=%d, want 400", recorder.Code)
			}
		})
	}
}

func TestParseJavMovieType(t *testing.T) {
	for _, value := range []string{"", "censored", "uncensored", "unknown", "invalid"} {
		t.Run(value, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest("GET", "/jav?movie_type="+value, nil)
			query, valid := parseJavFilterQuery(ctx)
			if valid != (value != "invalid") {
				t.Fatalf("valid=%v", valid)
			}
			if valid && query.MovieType != value {
				t.Fatalf("movie type=%q", query.MovieType)
			}
			if !valid && recorder.Code != 400 {
				t.Fatalf("status=%d", recorder.Code)
			}
		})
	}
}
