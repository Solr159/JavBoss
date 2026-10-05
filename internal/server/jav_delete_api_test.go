package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"javboss/internal/common"
	dbpkg "javboss/internal/db"
	"javboss/internal/models"
)

func TestDeleteJavVideos(t *testing.T) {
	for _, scenario := range []string{"all copies", "missing file", "no videos", "missing directory", "traversal", "symlink", "shared video", "invalid id", "not found"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "data"), 0755); err != nil {
				t.Fatal(err)
			}
			database, err := dbpkg.Open(filepath.Join(root, "data", "test.db"))
			if err != nil {
				t.Fatal(err)
			}
			prevDB, prevConfig := common.DB, common.AppConfig
			common.DB = database
			common.AppConfig = &common.Config{DatabasePath: filepath.Join(root, "data", "test.db"), JavCoverDir: filepath.Join(root, "covers")}
			t.Cleanup(func() { common.DB, common.AppConfig = prevDB, prevConfig; sqlDB, _ := database.DB(); _ = sqlDB.Close() })
			create := func(value any) {
				t.Helper()
				if err := database.Create(value).Error; err != nil {
					t.Fatal(err)
				}
			}
			write := func(path string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("test"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			item, other := models.Jav{Code: "ABC-123"}, models.Jav{Code: "ABC-124"}
			create(&item)
			create(&other)
			dir := models.Directory{Path: filepath.Join(root, "media")}
			create(&dir)
			video := models.Video{Fingerprint: "delete-me"}
			create(&video)
			keepVideo := models.Video{Fingerprint: "keep-me"}
			create(&keepVideo)
			first := filepath.Join(dir.Path, "ABC-123.mp4")
			second := filepath.Join(dir.Path, "copy.mp4")
			keep := filepath.Join(dir.Path, "ABC-124.mp4")
			cover := filepath.Join(common.AppConfig.JavCoverDir, "abc-123.jpg")
			shot := filepath.Join(root, "data", "video", fmt.Sprint(video.ID), "screenshot", "1.jpg")
			sidecars := []string{
				filepath.Join(dir.Path, "ABC-123.ass"),
				filepath.Join(dir.Path, "ABC-123.srt"),
				filepath.Join(dir.Path, "ABC-123.sub"),
				filepath.Join(dir.Path, "ABC-123.idx"),
				filepath.Join(dir.Path, "ABC-123.zh.SUB"),
				filepath.Join(dir.Path, "ABC-123.zh.IDX"),
				filepath.Join(dir.Path, "ABC-123.nfo"),
				filepath.Join(dir.Path, "ABC-123.jpg"),
				filepath.Join(dir.Path, "ABC-123-poster.jpg"),
				filepath.Join(dir.Path, "ABC-123.zh.srt"),
				filepath.Join(dir.Path, "copy.nfo"),
			}
			preserved := []string{keep, filepath.Join(dir.Path, "ABC-124.nfo"), filepath.Join(dir.Path, "ABC-1234.srt"), filepath.Join(dir.Path, "ABC-123.part2.mp4"), filepath.Join(dir.Path, "ABC-123.part2.zh.srt"), filepath.Join(dir.Path, "ABC-123.notes.txt")}
			preserved = append(preserved, sidecars...)
			for _, path := range append([]string{first, second, cover, shot}, preserved...) {
				write(path)
			}
			// Binary VobSub subtitles must also remain, regardless of their MPEG signature.
			for _, name := range []string{"ABC-123.sub", "ABC-123.zh.SUB"} {
				content := make([]byte, 2048)
				copy(content, []byte{0x00, 0x00, 0x01, 0xba, 0x44, 0x00, 0x04, 0x00})
				if err := os.WriteFile(filepath.Join(dir.Path, name), content, 0600); err != nil {
					t.Fatal(err)
				}
			}
			loc := models.VideoLocation{VideoID: video.ID, DirectoryID: dir.ID, RelativePath: "ABC-123.mp4", JavID: &item.ID}
			copyLoc := models.VideoLocation{VideoID: video.ID, DirectoryID: dir.ID, RelativePath: "copy.mp4", IsDelete: true}
			wantStatus := http.StatusOK
			switch scenario {
			case "missing file":
				if err := os.Remove(first); err != nil {
					t.Fatal(err)
				}
			case "missing directory":
				database.Model(&dir).Update("missing", true)
				wantStatus = http.StatusConflict
			case "traversal":
				copyLoc.RelativePath = "../outside.mp4"
				wantStatus = http.StatusConflict
			case "symlink":
				outside := filepath.Join(root, "outside.mp4")
				write(outside)
				if err := os.Remove(second); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, second); err != nil {
					t.Skip(err)
				}
				wantStatus = http.StatusConflict
			case "shared video":
				copyLoc.JavID = &other.ID
				wantStatus = http.StatusConflict
			case "invalid id":
				wantStatus = http.StatusBadRequest
			case "not found":
				wantStatus = http.StatusNotFound
			}
			if scenario != "no videos" {
				create(&loc)
				create(&copyLoc)
			}
			create(&models.VideoLocation{VideoID: keepVideo.ID, DirectoryID: dir.ID, RelativePath: "ABC-124.mp4", JavID: &other.ID})
			tag := models.JavTag{Name: "keep tag"}
			create(&tag)
			idol := models.JavIdol{Name: "keep idol", CoverJavID: &item.ID}
			create(&idol)
			create(&models.JavTagMap{JavID: item.ID, JavTagID: tag.ID})
			create(&models.JavIdolMap{JavID: item.ID, JavIdolID: idol.ID})
			videoTag := models.Tag{Name: "video tag"}
			create(&videoTag)
			create(&models.VideoTag{VideoID: video.ID, TagID: videoTag.ID})
			group := models.JavFavoriteGroup{Name: "keep group", EntityType: "jav"}
			create(&group)
			create(&models.JavFavoriteMap{JavFavoriteGroupID: group.ID, EntityType: "jav", EntityID: item.ID})
			router := gin.New()
			router.DELETE("/jav/items/:id/videos", deleteJavVideos)
			target := fmt.Sprint(item.ID)
			if scenario == "invalid id" {
				target = "0"
			}
			if scenario == "not found" {
				target = "999999"
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/jav/items/"+target+"/videos", nil))
			if response.Code != wantStatus {
				t.Fatalf("status=%d want=%d: %s", response.Code, wantStatus, response.Body.String())
			}
			var count int64
			database.Model(&models.Jav{}).Where("id = ?", item.ID).Count(&count)
			if count != 1 {
				t.Fatalf("remaining jav count=%d", count)
			}
			for _, path := range append(preserved, cover) {
				if _, err := os.Stat(path); err != nil {
					t.Fatalf("preserved file %s: %v", path, err)
				}
			}
			if wantStatus == http.StatusOK {
				var deleted []string
				if scenario != "no videos" {
					deleted = append(deleted, first, second, filepath.Dir(filepath.Dir(shot)))
				}
				for _, path := range deleted {
					if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
						t.Errorf("file remains %s: %v", path, err)
					}
				}
				for _, table := range []string{"jav_tag_map", "jav_idol_map", "jav_favorite_map"} {
					database.Table(table).Count(&count)
					if count != 1 {
						t.Errorf("%s rows=%d", table, count)
					}
				}
				database.First(&idol, idol.ID)
				if idol.CoverJavID == nil || *idol.CoverJavID != item.ID {
					t.Error("idol cover reference was changed")
				}
				if scenario != "no videos" {
					for _, table := range []string{"video", "video_location", "video_tag"} {
						column := "video_id"
						if table == "video" {
							column = "id"
						}
						database.Table(table).Where(column+" = ?", video.ID).Count(&count)
						if count != 0 {
							t.Errorf("%s deleted video rows=%d", table, count)
						}
					}
				}
				items, total, err := dbpkg.SearchJav(context.Background(), nil, nil, item.Code, "recent", 10, 0, nil, nil)
				if err != nil || total != 0 || len(items) != 0 {
					t.Fatalf("JAV without videos remains in list: items=%v total=%d err=%v", items, total, err)
				}
				retained, err := dbpkg.GetJav(context.Background(), item.ID, nil)
				if err != nil || retained.Code != item.Code || len(retained.Videos) != 0 || len(retained.Idols) != 1 || retained.FavoriteCount != 1 {
					t.Fatalf("JAV metadata was changed: item=%+v err=%v", retained, err)
				}
				if scenario == "all copies" {
					// Importing a new video can reuse the retained metadata and restore visibility.
					reimported := models.Video{Fingerprint: "reimported"}
					create(&reimported)
					create(&models.VideoLocation{VideoID: reimported.ID, DirectoryID: dir.ID, RelativePath: "reimported.mp4", JavID: &item.ID})
					items, total, err = dbpkg.SearchJav(context.Background(), nil, nil, item.Code, "recent", 10, 0, nil, nil)
					if err != nil || total != 1 || len(items) != 1 || items[0].ID != item.ID {
						t.Fatalf("reimported JAV did not return: items=%v total=%d err=%v", items, total, err)
					}
				}
			} else {
				for _, path := range []string{first, cover, shot} {
					if _, err := os.Stat(path); err != nil {
						t.Errorf("failure removed %s: %v", path, err)
					}
				}
			}
			for _, table := range []string{"jav_tag", "jav_idol", "jav_favorite_group", "tag"} {
				database.Table(table).Count(&count)
				if count != 1 {
					t.Errorf("shared metadata %s rows=%d", table, count)
				}
			}
			database.Model(&models.Jav{}).Where("id = ?", other.ID).Count(&count)
			if count != 1 {
				t.Error("other jav removed")
			}
			var violations []struct{ Table string }
			if err := database.Raw("PRAGMA foreign_key_check").Scan(&violations).Error; err != nil || len(violations) > 0 {
				t.Fatalf("foreign key violations=%v err=%v", violations, err)
			}
		})
	}
}
