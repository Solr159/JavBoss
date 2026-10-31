package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	dbpkg "javboss/internal/db"
	"javboss/internal/models"
)

func TestVideoDirectoryFilter(t *testing.T) {
	deps := newTestDependencies()
	database, err := dbpkg.Open(filepath.Join(t.TempDir(), "directories.db"))
	if err != nil {
		t.Fatal(err)
	}
	deps.database = database
	deps.bindDatabase(t)
	t.Cleanup(func() {
		if conn, err := database.DB(); err == nil {
			_ = conn.Close()
		}
	})
	roots := []models.Directory{{Path: "/media/one"}, {Path: "/media/two"}, {Path: "/media/disabled"}, {Path: "/media/deleted"}}
	if err := database.Create(&roots).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&roots[2]).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&roots[3]).Update("is_delete", true).Error; err != nil {
		t.Fatal(err)
	}
	jav := models.Jav{Code: "TEST-001"}
	if err := database.Create(&jav).Error; err != nil {
		t.Fatal(err)
	}
	for i, row := range []struct {
		root           int
		path           string
		tagged, linked bool
	}{
		{0, "root.mp4", false, false},
		{0, "旅行/a.mp4", true, false},
		{0, "旅行/2026/b.mp4", true, true},
		{0, "旅行-extra/c.mp4", false, false},
		{0, "旅行/2026/deeper/d.mp4", false, false},
		{0, "100%_fun/e.mp4", false, false},
		{0, "100xxfun/f.mp4", false, false},
		{0, "Case/g.mp4", false, false},
		{0, "case/h.mp4", false, false},
		{1, "旅行/i.mp4", false, false},
		{2, "hidden/j.mp4", false, false},
		{3, "deleted/k.mp4", false, false},
	} {
		video := models.Video{Fingerprint: fmt.Sprintf("directory-filter-%d", i)}
		if row.tagged {
			video.Tags = []models.Tag{{Name: "chosen"}}
			if i == 2 {
				if err := database.Where("name = ?", "chosen").First(&video.Tags[0]).Error; err != nil {
					t.Fatal(err)
				}
			}
		}
		if err := database.Create(&video).Error; err != nil {
			t.Fatal(err)
		}
		location, err := dbpkg.Default().UpsertVideoLocation(context.Background(), video.ID, roots[row.root].ID, row.path, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if row.linked {
			if err := database.Model(location).Update("jav_id", jav.ID).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/videos", listVideos)
	router.GET("/videos/directories", listVideoDirectories)
	for _, tc := range []struct {
		name         string
		root         int
		dir          string
		extra        url.Values
		total, count int
	}{
		{"all roots", -1, "", nil, 10, 10},
		{"one root", 0, "", nil, 9, 9},
		{"root dot", 0, ".", nil, 9, 9},
		{"recursive and separator boundary", 0, "旅行", nil, 3, 3},
		{"trailing separator", 0, "旅行/", nil, 3, 3},
		{"nested", 0, "旅行/2026", nil, 2, 2},
		{"other root same name", 1, "旅行", nil, 1, 1},
		{"disabled", 2, "", nil, 0, 0},
		{"deleted", 3, "", nil, 0, 0},
		{"missing", 0, "missing", nil, 0, 0},
		{"literal wildcards", 0, "100%_fun", nil, 1, 1},
		{"case sensitive", 0, "Case", nil, 1, 1},
		{"tag intersection", 0, "旅行", url.Values{"tags": {"chosen"}}, 2, 2},
		{"hide linked", 0, "旅行", url.Values{"tags": {"chosen"}, "hide_jav": {"1"}}, 1, 1},
		{"search", 0, "旅行", url.Values{"search": {"b.mp4"}}, 1, 1},
		{"pagination", 0, "旅行", url.Values{"limit": {"1"}, "offset": {"1"}}, 3, 1},
		{"random", 0, "旅行", url.Values{"sort": {"random"}, "seed": {"42"}}, 3, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			query := tc.extra
			if query == nil {
				query = make(url.Values)
			}
			if tc.root >= 0 {
				query.Set("directory_id", strconv.FormatInt(roots[tc.root].ID, 10))
			}
			query.Set("subdirectory", tc.dir)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/videos?"+query.Encode(), nil))
			var result struct {
				Items []models.Video `json:"items"`
				Total int            `json:"total"`
			}
			if recorder.Code != http.StatusOK || json.Unmarshal(recorder.Body.Bytes(), &result) != nil {
				t.Fatalf("response: %d %s", recorder.Code, recorder.Body.String())
			}
			if result.Total != tc.total || len(result.Items) != tc.count {
				t.Fatalf("got total=%d items=%d, want %d/%d", result.Total, len(result.Items), tc.total, tc.count)
			}
			for _, item := range result.Items {
				if tc.root >= 0 && item.DirectoryID != roots[tc.root].ID {
					t.Fatalf("unexpected root: %+v", item)
				}
			}
		})
	}
	for _, query := range []string{"directory_id=0", "directory_id=no", "subdirectory=旅行", "directory_id=1&subdirectory=../escape", "directory_id=1&subdirectory=/absolute"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/videos?"+query, nil))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("invalid query %q returned %d", query, recorder.Code)
		}
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/videos/directories", nil))
	var result struct {
		Items []dbpkg.VideoDirectory `json:"items"`
	}
	if recorder.Code != http.StatusOK || json.Unmarshal(recorder.Body.Bytes(), &result) != nil {
		t.Fatalf("directory response: %d %s", recorder.Code, recorder.Body.String())
	}
	seen := make(map[string]bool)
	for _, item := range result.Items {
		if item.DirectoryID == roots[2].ID || item.DirectoryID == roots[3].ID {
			t.Fatalf("inactive directory exposed: %+v", item)
		}
		key := fmt.Sprintf("%d:%s", item.DirectoryID, item.Path)
		if seen[key] {
			t.Fatalf("duplicate option: %s", key)
		}
		seen[key] = true
		if item.DirectoryID == roots[0].ID && item.Path == "." {
			if item.VideoCount != 1 || len(item.Videos) != 1 || item.Videos[0] != "root.mp4" {
				t.Errorf("root preview includes descendants or omits files: %+v", item)
			}
		}
		if item.DirectoryID == roots[1].ID && item.Path == "." && (item.VideoCount != 0 || len(item.Videos) != 0) {
			t.Errorf("ancestor should have no direct videos: %+v", item)
		}
	}
	for _, dir := range []string{".", "旅行", "旅行/2026", "旅行/2026/deeper", "100%_fun"} {
		if !seen[fmt.Sprintf("%d:%s", roots[0].ID, dir)] {
			t.Errorf("missing directory %q", dir)
		}
	}
}
