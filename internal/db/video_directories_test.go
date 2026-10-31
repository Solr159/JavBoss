package db

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"javboss/internal/models"
)

func TestListVideoDirectoriesPreviews(t *testing.T) {
	deps := newTestDependencies()
	database := openTestDB(deps, t)
	roots := []models.Directory{{Path: "/one"}, {Path: "/two"}, {Path: "/disabled"}}
	if err := database.Create(&roots).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&roots[2]).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	add := func(root int, filename string) {
		t.Helper()
		video := models.Video{Fingerprint: fmt.Sprintf("%d:%s", root, filename)}
		if err := database.Create(&video).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := deps.store.UpsertVideoLocation(context.Background(), video.ID, roots[root].ID, filename, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	// Insert in reverse order: previews must remain stable and sorted by filename.
	for i := 8; i >= 1; i-- {
		add(0, fmt.Sprintf("parent/files/clip-%02d.mp4", i))
	}
	add(0, "parent/sibling/另一部.mp4")
	add(1, "clip-01.mp4")
	add(2, "hidden.mp4")
	items, err := deps.store.ListVideoDirectories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 5 {
		t.Fatalf("directories=%d, want 5", len(items))
	}
	for _, item := range items {
		var want []string
		count := 0
		switch {
		case item.DirectoryID == roots[2].ID:
			t.Fatalf("inactive root exposed: %+v", item)
		case item.DirectoryID == roots[1].ID:
			want, count = []string{"clip-01.mp4"}, 1
		case item.Path == "parent/files":
			want, count = []string{"clip-01.mp4", "clip-02.mp4", "clip-03.mp4", "clip-04.mp4", "clip-05.mp4"}, 8
		case item.Path == "parent/sibling":
			want, count = []string{"另一部.mp4"}, 1
		default:
			want = []string{}
		}
		if item.VideoCount != count || !reflect.DeepEqual(item.Videos, want) {
			t.Errorf("directory %d:%s: count=%d videos=%v, want %d %v", item.DirectoryID, item.Path, item.VideoCount, item.Videos, count, want)
		}
	}
}
