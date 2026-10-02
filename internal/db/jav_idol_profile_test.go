package db

import (
	"testing"
	"time"

	"javboss/internal/models"
)

// ListIdolsMissingProfile drives the idol profile enrichment job. It must include
// idols whose only works are multi-actress titles, otherwise their profiles are
// never filled in.
func TestListIdolsMissingProfileIncludesMultiActressWorks(t *testing.T) {
	gdb := openTestDB(t)

	dir := models.Directory{Path: "/media", Enabled: true}
	if err := gdb.Create(&dir).Error; err != nil {
		t.Fatalf("create directory: %v", err)
	}

	completeIdol := func(name string) models.JavIdol {
		height, bust, waist, hips, cup := 160, 88, 60, 88, 5
		birth := time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)
		return models.JavIdol{
			Name:         name,
			RomanName:    "Roman Name",
			JapaneseName: name,
			ChineseName:  "中文名",
			HeightCM:     &height,
			BirthDate:    &birth,
			Bust:         &bust,
			Waist:        &waist,
			Hips:         &hips,
			Cup:          &cup,
		}
	}
	blankIdol := func(name string) models.JavIdol { return models.JavIdol{Name: name} }

	idols := []models.JavIdol{
		blankIdol("Solo Idol"),
		blankIdol("Duo A"),
		blankIdol("Duo B"),
		blankIdol("Orphan Idol"),
		completeIdol("Complete Idol"),
		blankIdol("Deleted Work Idol"),
	}
	if err := gdb.Create(&idols).Error; err != nil {
		t.Fatalf("create idols: %v", err)
	}

	linkWork := func(code string, deleted bool, idolIndexes ...int) {
		t.Helper()
		item := models.Jav{Code: code}
		if err := gdb.Create(&item).Error; err != nil {
			t.Fatalf("create jav %s: %v", code, err)
		}
		video := models.Video{Fingerprint: code}
		if err := gdb.Create(&video).Error; err != nil {
			t.Fatalf("create video %s: %v", code, err)
		}
		location := models.VideoLocation{
			VideoID:      video.ID,
			DirectoryID:  dir.ID,
			RelativePath: code + ".mp4",
			JavID:        &item.ID,
			IsDelete:     deleted,
		}
		if err := gdb.Create(&location).Error; err != nil {
			t.Fatalf("create location %s: %v", code, err)
		}
		for _, index := range idolIndexes {
			if err := gdb.Create(&models.JavIdolMap{JavID: item.ID, JavIdolID: idols[index].ID}).Error; err != nil {
				t.Fatalf("map idol %d to %s: %v", index, code, err)
			}
		}
	}

	linkWork("SOLO-001", false, 0)   // solo work
	linkWork("DUO-001", false, 1, 2) // multi-actress work
	linkWork("DONE-001", false, 4)   // already complete profile
	linkWork("DELETED-001", true, 5) // only a deleted location

	got, err := ListIdolsMissingProfile(t.Context())
	if err != nil {
		t.Fatalf("ListIdolsMissingProfile: %v", err)
	}
	names := make(map[string]bool, len(got))
	for _, idol := range got {
		names[idol.Name] = true
	}

	for _, want := range []string{"Solo Idol", "Duo A", "Duo B"} {
		if !names[want] {
			t.Errorf("candidate list missing %q: %v", want, names)
		}
	}
	for _, unwanted := range []string{"Orphan Idol", "Complete Idol", "Deleted Work Idol"} {
		if names[unwanted] {
			t.Errorf("candidate list should exclude %q: %v", unwanted, names)
		}
	}
}
