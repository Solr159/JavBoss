package db

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"pornboss/internal/common"
	"pornboss/internal/jav"
	"pornboss/internal/models"

	"gorm.io/gorm"
)

func TestListJavIdolsOnlyIncludesIdolsWithVisibleSoloWorks(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1710000000, 0).UTC()

	dir := models.Directory{Path: "/tmp/media"}
	if err := db.Create(&dir).Error; err != nil {
		t.Fatalf("create directory: %v", err)
	}

	soloIdol := models.JavIdol{Name: "Solo Idol"}
	groupOnlyIdol := models.JavIdol{Name: "Group Only Idol"}
	if err := db.Create(&soloIdol).Error; err != nil {
		t.Fatalf("create solo idol: %v", err)
	}
	if err := db.Create(&groupOnlyIdol).Error; err != nil {
		t.Fatalf("create group idol: %v", err)
	}

	soloJav := models.Jav{Code: "AAA-001", Title: "Solo Work", Provider: 1, FetchedAt: now}
	groupJav := models.Jav{Code: "BBB-001", Title: "Group Work", Provider: 1, FetchedAt: now}
	unavailableSoloJav := models.Jav{Code: "CCC-001", Title: "Unavailable Solo Work", Provider: 1, FetchedAt: now}
	if err := db.Create(&soloJav).Error; err != nil {
		t.Fatalf("create solo jav: %v", err)
	}
	if err := db.Create(&groupJav).Error; err != nil {
		t.Fatalf("create group jav: %v", err)
	}
	if err := db.Create(&unavailableSoloJav).Error; err != nil {
		t.Fatalf("create unavailable solo jav: %v", err)
	}

	maps := []models.JavIdolMap{
		{JavID: soloJav.ID, JavIdolID: soloIdol.ID},
		{JavID: groupJav.ID, JavIdolID: soloIdol.ID},
		{JavID: groupJav.ID, JavIdolID: groupOnlyIdol.ID},
		{JavID: unavailableSoloJav.ID, JavIdolID: groupOnlyIdol.ID},
	}
	if err := db.Create(&maps).Error; err != nil {
		t.Fatalf("create idol maps: %v", err)
	}

	videos := []models.Video{
		{
			DirectoryID: dir.ID,
			Path:        "solo.mp4",
			Filename:    "solo.mp4",
			Fingerprint: "fp-solo",
			JavID:       int64Ptr(soloJav.ID),
			ModifiedAt:  now,
		},
		{
			DirectoryID: dir.ID,
			Path:        "group.mp4",
			Filename:    "group.mp4",
			Fingerprint: "fp-group",
			JavID:       int64Ptr(groupJav.ID),
			ModifiedAt:  now,
		},
		{
			DirectoryID: dir.ID,
			Path:        "unavailable.mp4",
			Filename:    "unavailable.mp4",
			Fingerprint: "fp-unavailable",
			JavID:       int64Ptr(unavailableSoloJav.ID),
			ModifiedAt:  now,
		},
	}
	if err := db.Create(&videos).Error; err != nil {
		t.Fatalf("create videos: %v", err)
	}
	createVideoLocationsForVideos(t, db, videos...)
	if err := db.Model(&models.VideoLocation{}).
		Where("video_id = ?", videos[2].ID).
		Update("is_delete", true).Error; err != nil {
		t.Fatalf("mark unavailable video location deleted: %v", err)
	}

	items, total, err := ListJavIdols(ctx, "", "", 20, 0, nil)
	if err != nil {
		t.Fatalf("ListJavIdols: %v", err)
	}

	if total != 1 {
		t.Fatalf("unexpected total: got %d want 1", total)
	}
	if len(items) != 1 {
		t.Fatalf("unexpected item count: got %d want 1", len(items))
	}
	if items[0].ID != soloIdol.ID {
		t.Fatalf("unexpected idol id: got %d want %d", items[0].ID, soloIdol.ID)
	}
	if items[0].WorkCount != 2 {
		t.Fatalf("unexpected work count: got %d want 2", items[0].WorkCount)
	}
	if items[0].SampleCode != soloJav.Code {
		t.Fatalf("unexpected sample code: got %q want %q", items[0].SampleCode, soloJav.Code)
	}
}

func TestSearchJavFiltersByIdolIDs(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1710000000, 0).UTC()

	dir := models.Directory{Path: "/tmp/media"}
	if err := db.Create(&dir).Error; err != nil {
		t.Fatalf("create directory: %v", err)
	}

	idolA := models.JavIdol{Name: "Idol A"}
	idolB := models.JavIdol{Name: "Idol B"}
	idolC := models.JavIdol{Name: "Idol C"}
	if err := db.Create(&[]models.JavIdol{idolA, idolB, idolC}).Error; err != nil {
		t.Fatalf("create idols: %v", err)
	}
	var idols []models.JavIdol
	if err := db.Order("name").Find(&idols).Error; err != nil {
		t.Fatalf("load idols: %v", err)
	}
	idolA, idolB, idolC = idols[0], idols[1], idols[2]

	javs := []models.Jav{
		{Code: "IDA-001", Title: "A and B", Provider: 1, FetchedAt: now},
		{Code: "IDA-002", Title: "A only", Provider: 1, FetchedAt: now},
		{Code: "IDC-001", Title: "C only", Provider: 1, FetchedAt: now},
	}
	if err := db.Create(&javs).Error; err != nil {
		t.Fatalf("create javs: %v", err)
	}
	maps := []models.JavIdolMap{
		{JavID: javs[0].ID, JavIdolID: idolA.ID},
		{JavID: javs[0].ID, JavIdolID: idolB.ID},
		{JavID: javs[1].ID, JavIdolID: idolA.ID},
		{JavID: javs[2].ID, JavIdolID: idolC.ID},
	}
	if err := db.Create(&maps).Error; err != nil {
		t.Fatalf("create idol maps: %v", err)
	}
	videos := []models.Video{
		{DirectoryID: dir.ID, Path: "ida-001.mp4", Filename: "ida-001.mp4", Fingerprint: "fp-ida-001", JavID: int64Ptr(javs[0].ID), ModifiedAt: now},
		{DirectoryID: dir.ID, Path: "ida-002.mp4", Filename: "ida-002.mp4", Fingerprint: "fp-ida-002", JavID: int64Ptr(javs[1].ID), ModifiedAt: now},
		{DirectoryID: dir.ID, Path: "idc-001.mp4", Filename: "idc-001.mp4", Fingerprint: "fp-idc-001", JavID: int64Ptr(javs[2].ID), ModifiedAt: now},
	}
	if err := db.Create(&videos).Error; err != nil {
		t.Fatalf("create videos: %v", err)
	}
	createVideoLocationsForVideos(t, db, videos...)

	items, total, err := SearchJav(ctx, []int64{idolA.ID, idolB.ID}, nil, "", "code", 20, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchJav by idol ids: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("unexpected filtered javs: total=%d len=%d", total, len(items))
	}
	if items[0].Code != "IDA-001" {
		t.Fatalf("unexpected jav code: got %q want IDA-001", items[0].Code)
	}
}

func TestListJavStudiosAndSearchByStudio(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1710000000, 0).UTC()

	dir := models.Directory{Path: "/tmp/media"}
	if err := db.Create(&dir).Error; err != nil {
		t.Fatalf("create directory: %v", err)
	}

	studioA := models.JavStudio{Name: "Studio A"}
	studioB := models.JavStudio{Name: "Studio B"}
	if err := db.Create(&studioA).Error; err != nil {
		t.Fatalf("create studio a: %v", err)
	}
	if err := db.Create(&studioB).Error; err != nil {
		t.Fatalf("create studio b: %v", err)
	}

	javs := []models.Jav{
		{Code: "STA-001", Title: "Studio A One", StudioID: int64Ptr(studioA.ID), Provider: 1, FetchedAt: now},
		{Code: "STA-002", Title: "Studio A Two", StudioID: int64Ptr(studioA.ID), Provider: 1, FetchedAt: now},
		{Code: "STB-001", Title: "Studio B One", StudioID: int64Ptr(studioB.ID), Provider: 1, FetchedAt: now},
	}
	if err := db.Create(&javs).Error; err != nil {
		t.Fatalf("create javs: %v", err)
	}

	videos := []models.Video{
		{DirectoryID: dir.ID, Path: "sta-001.mp4", Filename: "sta-001.mp4", Fingerprint: "fp-sta-001", JavID: int64Ptr(javs[0].ID), ModifiedAt: now},
		{DirectoryID: dir.ID, Path: "sta-002.mp4", Filename: "sta-002.mp4", Fingerprint: "fp-sta-002", JavID: int64Ptr(javs[1].ID), ModifiedAt: now},
		{DirectoryID: dir.ID, Path: "stb-001.mp4", Filename: "stb-001.mp4", Fingerprint: "fp-stb-001", JavID: int64Ptr(javs[2].ID), ModifiedAt: now},
	}
	if err := db.Create(&videos).Error; err != nil {
		t.Fatalf("create videos: %v", err)
	}
	createVideoLocationsForVideos(t, db, videos...)

	studios, total, err := ListJavStudios(ctx, "", 20, 0, nil)
	if err != nil {
		t.Fatalf("ListJavStudios: %v", err)
	}
	if total != 2 {
		t.Fatalf("unexpected studio total: got %d want 2", total)
	}
	if len(studios) != 2 {
		t.Fatalf("unexpected studio count: got %d want 2", len(studios))
	}
	if studios[0].ID != studioA.ID || studios[0].WorkCount != 2 {
		t.Fatalf("unexpected first studio: %#v", studios[0])
	}
	if studios[0].SampleCode == "" {
		t.Fatalf("expected sample code for first studio")
	}

	items, total, err := SearchJav(ctx, nil, nil, "", "code", 20, 0, nil, nil, studioA.ID)
	if err != nil {
		t.Fatalf("SearchJav by studio: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("unexpected filtered javs: total=%d len=%d", total, len(items))
	}
	for _, item := range items {
		if item.StudioID == nil || *item.StudioID != studioA.ID {
			t.Fatalf("unexpected studio filtered item: %#v", item)
		}
	}
}

func TestSaveJavInfoAppendsIdolsOnlyWhenLanguageMappingMissing(t *testing.T) {
	gdb := openTestDB(t)
	now := time.Unix(1710000000, 0).UTC()

	save := func(info *jav.JavInfo) {
		t.Helper()
		if err := gdb.Transaction(func(tx *gorm.DB) error {
			_, err := saveJavInfoTx(tx, info, now)
			return err
		}); err != nil {
			t.Fatalf("save jav info: %v", err)
		}
	}

	save(&jav.JavInfo{
		Code:     "AAA-001",
		Title:    "Japanese metadata",
		Actors:   []string{"岬ななみ"},
		Provider: jav.ProviderJavBus,
	})
	assertJavIdolMaps(t, gdb, "AAA-001", map[string]bool{
		"岬ななみ": false,
	})

	save(&jav.JavInfo{
		Code:     "AAA-001",
		Title:    "Japanese metadata refreshed",
		Actors:   []string{"別の女優"},
		Provider: jav.ProviderJavBus,
	})
	assertJavIdolMaps(t, gdb, "AAA-001", map[string]bool{
		"岬ななみ": false,
	})

	save(&jav.JavInfo{
		Code:     "AAA-001",
		Title:    "English metadata",
		Actors:   []string{"Nanami Misaki"},
		Provider: jav.ProviderJavDatabase,
	})
	assertJavTitles(t, gdb, "AAA-001", "Japanese metadata refreshed", "English metadata")
	assertJavIdolMaps(t, gdb, "AAA-001", map[string]bool{
		"岬ななみ":          false,
		"Nanami Misaki": true,
	})

	save(&jav.JavInfo{
		Code:     "AAA-001",
		Title:    "English metadata refreshed",
		Actors:   []string{"Other Actress"},
		Provider: jav.ProviderJavDatabase,
	})
	assertJavTitles(t, gdb, "AAA-001", "Japanese metadata refreshed", "English metadata refreshed")
	assertJavIdolMaps(t, gdb, "AAA-001", map[string]bool{
		"岬ななみ":          false,
		"Nanami Misaki": true,
	})

	save(&jav.JavInfo{
		Code:     "BBB-001",
		Title:    "Shared-name metadata",
		Actors:   []string{"Shared Name"},
		Provider: jav.ProviderJavBus,
	})
	save(&jav.JavInfo{
		Code:     "BBB-001",
		Title:    "Shared-name metadata refreshed",
		Actors:   []string{"Shared Name"},
		Provider: jav.ProviderJavDatabase,
	})
	assertJavTitles(t, gdb, "BBB-001", "Shared-name metadata", "Shared-name metadata refreshed")
	assertJavIdolLanguageCount(t, gdb, "Shared Name", 2)
	assertJavIdolMapLanguages(t, gdb, "BBB-001", "Shared Name", []bool{false, true})

	save(&jav.JavInfo{
		Code:     "CCC-001",
		Title:    "Japanese-name English-provider metadata",
		Actors:   []string{"三上悠亜"},
		Provider: jav.ProviderJavDatabase,
	})
	assertJavIdolMaps(t, gdb, "CCC-001", map[string]bool{
		"三上悠亜": true,
	})

	save(&jav.JavInfo{
		Code:     "DDD-001",
		Title:    "English alias metadata",
		Actors:   []string{"Ameri Ichinose (Ayaka Misora)"},
		Provider: jav.ProviderJavDatabase,
	})
	assertJavIdolMaps(t, gdb, "DDD-001", map[string]bool{
		"Ameri Ichinose (Ayaka Misora)": true,
	})

	save(&jav.JavInfo{
		Code:     "EEE-001",
		Title:    "JavBus romanized stage name",
		Actors:   []string{"AIKA"},
		Provider: jav.ProviderJavBus,
	})
	assertJavIdolMaps(t, gdb, "EEE-001", map[string]bool{
		"AIKA": false,
	})
	save(&jav.JavInfo{
		Code:     "EEE-001",
		Title:    "English romanized stage name",
		Actors:   []string{"AIKA"},
		Provider: jav.ProviderJavDatabase,
	})
	assertJavIdolLanguageCount(t, gdb, "AIKA", 2)
	assertJavIdolMapLanguages(t, gdb, "EEE-001", "AIKA", []bool{false, true})

	save(&jav.JavInfo{
		Code:     "FFF-001",
		Title:    "ThePornDB metadata",
		Actors:   []string{"English Performer"},
		Provider: jav.ProviderThePornDB,
	})
	assertJavTitles(t, gdb, "FFF-001", "", "ThePornDB metadata")
	assertJavIdolMaps(t, gdb, "FFF-001", map[string]bool{
		"English Performer": true,
	})
}

func TestSaveJavInfoReplacesOnlyCurrentProviderTags(t *testing.T) {
	gdb := openTestDB(t)
	now := time.Unix(1710000000, 0).UTC()

	save := func(info *jav.JavInfo) {
		t.Helper()
		if err := gdb.Transaction(func(tx *gorm.DB) error {
			_, err := saveJavInfoTx(tx, info, now)
			return err
		}); err != nil {
			t.Fatalf("save jav info: %v", err)
		}
	}

	save(&jav.JavInfo{
		Code:     "TAG-001",
		Title:    "Initial metadata",
		Tags:     []string{"Drama", "Featured Actress"},
		Provider: jav.ProviderJavBus,
	})

	var javRec models.Jav
	if err := gdb.Where("code = ?", "TAG-001").First(&javRec).Error; err != nil {
		t.Fatalf("load jav: %v", err)
	}
	englishTag := models.JavTag{Name: "Plot Based", Provider: int(jav.ProviderJavDatabase)}
	userTag := models.JavTag{Name: "Favorite", Provider: int(jav.ProviderUser)}
	if err := gdb.Create(&englishTag).Error; err != nil {
		t.Fatalf("create english tag: %v", err)
	}
	if err := gdb.Create(&userTag).Error; err != nil {
		t.Fatalf("create user tag: %v", err)
	}
	if err := gdb.Create(&[]models.JavTagMap{
		{JavID: javRec.ID, JavTagID: englishTag.ID},
		{JavID: javRec.ID, JavTagID: userTag.ID},
	}).Error; err != nil {
		t.Fatalf("create extra tag maps: %v", err)
	}

	save(&jav.JavInfo{
		Code:     "TAG-001",
		Title:    "Refreshed metadata",
		Tags:     []string{"Cosplay"},
		Provider: jav.ProviderJavBus,
	})

	assertJavTagMaps(t, gdb, "TAG-001", map[string]int{
		"Cosplay":    int(jav.ProviderJavBus),
		"Plot Based": int(jav.ProviderJavDatabase),
		"Favorite":   int(jav.ProviderUser),
	})
}

func TestSaveAndUpdateJavStudio(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1710000000, 0).UTC()

	if err := gdb.Transaction(func(tx *gorm.DB) error {
		_, err := saveJavInfoTx(tx, &jav.JavInfo{
			Code:     "STU-001",
			Title:    "Studio metadata",
			Studio:   "Idea Pocket",
			Provider: jav.ProviderJavDatabase,
		}, now)
		return err
	}); err != nil {
		t.Fatalf("save jav info: %v", err)
	}
	assertJavStudio(t, gdb, "STU-001", "Idea Pocket")

	plainJav := models.Jav{Code: "STU-002", Title: "Missing studio", Provider: int(jav.ProviderJavBus), FetchedAt: now}
	if err := gdb.Create(&plainJav).Error; err != nil {
		t.Fatalf("create jav: %v", err)
	}
	if err := UpdateJavStudio(ctx, plainJav.ID, "S1 No. 1 Style"); err != nil {
		t.Fatalf("update jav studio: %v", err)
	}
	assertJavStudio(t, gdb, "STU-002", "S1 No. 1 Style")
}

func TestSetVideoLocationJavIDAllowsStaleNoop(t *testing.T) {
	gdb := openTestDB(t)
	now := time.Unix(1710000000, 0).UTC()

	dir := models.Directory{Path: "/tmp/media"}
	if err := gdb.Create(&dir).Error; err != nil {
		t.Fatalf("create directory: %v", err)
	}
	video := models.Video{
		DirectoryID: dir.ID,
		Path:        "noop.mp4",
		Filename:    "noop.mp4",
		Fingerprint: "fp-noop",
		DurationSec: 7200,
		ModifiedAt:  now,
	}
	if err := gdb.Create(&video).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}
	currentJav := models.Jav{Code: "NOOP-001", Title: "Current", Provider: int(jav.ProviderJavBus), FetchedAt: now}
	otherJav := models.Jav{Code: "NOOP-002", Title: "Other", Provider: int(jav.ProviderJavDatabase), FetchedAt: now}
	if err := gdb.Create(&currentJav).Error; err != nil {
		t.Fatalf("create current jav: %v", err)
	}
	if err := gdb.Create(&otherJav).Error; err != nil {
		t.Fatalf("create other jav: %v", err)
	}
	loc := models.VideoLocation{
		VideoID:      video.ID,
		DirectoryID:  dir.ID,
		RelativePath: "noop.mp4",
		ModifiedAt:   now,
		JavID:        int64Ptr(currentJav.ID),
	}
	if err := gdb.Create(&loc).Error; err != nil {
		t.Fatalf("create video location: %v", err)
	}

	staleUpdatedAt := now.Add(-time.Hour)
	if err := setVideoLocationJavIDTx(gdb, loc.ID, currentJav.ID, staleUpdatedAt); err != nil {
		t.Fatalf("same jav id should be accepted as noop: %v", err)
	}
	if err := setVideoLocationJavIDTx(gdb, loc.ID, otherJav.ID, staleUpdatedAt); err == nil {
		t.Fatal("different jav id with stale updated_at should fail")
	}
}

func TestSearchJavPreloadsOnlyCurrentLanguageIdols(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1710000000, 0).UTC()
	prevLang := jav.CurrentMetadataLanguage()
	t.Cleanup(func() {
		jav.SetMetadataLanguage(string(prevLang))
	})

	dir := models.Directory{Path: "/tmp/media"}
	if err := gdb.Create(&dir).Error; err != nil {
		t.Fatalf("create directory: %v", err)
	}

	javRec := models.Jav{Code: "LANG-001", Title: "Language Work", Provider: 1, FetchedAt: now}
	if err := gdb.Create(&javRec).Error; err != nil {
		t.Fatalf("create jav: %v", err)
	}

	japaneseIdol := models.JavIdol{Name: "岬ななみ", IsEnglish: false}
	englishIdol := models.JavIdol{Name: "Nanami Misaki", IsEnglish: true}
	if err := gdb.Create(&japaneseIdol).Error; err != nil {
		t.Fatalf("create japanese idol: %v", err)
	}
	if err := gdb.Create(&englishIdol).Error; err != nil {
		t.Fatalf("create english idol: %v", err)
	}
	if err := gdb.Create(&[]models.JavIdolMap{
		{JavID: javRec.ID, JavIdolID: japaneseIdol.ID},
		{JavID: javRec.ID, JavIdolID: englishIdol.ID},
	}).Error; err != nil {
		t.Fatalf("create idol maps: %v", err)
	}

	video := models.Video{
		DirectoryID: dir.ID,
		Path:        "lang-001.mp4",
		Filename:    "lang-001.mp4",
		Fingerprint: "fp-lang",
		DurationSec: 7200,
		ModifiedAt:  now,
	}
	if err := gdb.Create(&video).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}
	loc := models.VideoLocation{
		VideoID:      video.ID,
		DirectoryID:  dir.ID,
		RelativePath: "lang-001.mp4",
		ModifiedAt:   now,
		JavID:        int64Ptr(javRec.ID),
	}
	if err := gdb.Create(&loc).Error; err != nil {
		t.Fatalf("create video location: %v", err)
	}

	jav.SetMetadataLanguage("zh")
	items, total, err := SearchJav(ctx, nil, nil, "", "code", 20, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchJav zh: %v", err)
	}
	assertSearchJavIdols(t, items, total, []string{"岬ななみ"})

	jav.SetMetadataLanguage("en")
	items, total, err = SearchJav(ctx, nil, nil, "", "code", 20, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchJav en: %v", err)
	}
	assertSearchJavIdols(t, items, total, []string{"Nanami Misaki"})
}

func TestSearchJavUsesCurrentLanguageTitleField(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1710000000, 0).UTC()
	prevLang := jav.CurrentMetadataLanguage()
	t.Cleanup(func() {
		jav.SetMetadataLanguage(string(prevLang))
	})

	dir := models.Directory{Path: "/tmp/media"}
	if err := gdb.Create(&dir).Error; err != nil {
		t.Fatalf("create directory: %v", err)
	}

	javRec := models.Jav{
		Code:      "TITLE-001",
		Title:     "日本語タイトル",
		TitleEn:   "English Title",
		Provider:  int(jav.ProviderJavDatabase),
		FetchedAt: now,
	}
	if err := gdb.Create(&javRec).Error; err != nil {
		t.Fatalf("create jav: %v", err)
	}
	video := models.Video{
		DirectoryID: dir.ID,
		Path:        "title-001.mp4",
		Filename:    "title-001.mp4",
		Fingerprint: "fp-title",
		JavID:       int64Ptr(javRec.ID),
		DurationSec: 7200,
		ModifiedAt:  now,
	}
	if err := gdb.Create(&video).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}
	createVideoLocationsForVideos(t, gdb, video)

	jav.SetMetadataLanguage("zh")
	items, total, err := SearchJav(ctx, nil, nil, "日本語", "code", 20, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchJav zh title: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].Title != "日本語タイトル" {
		t.Fatalf("unexpected zh search result: total=%d items=%#v", total, items)
	}
	items, total, err = SearchJav(ctx, nil, nil, "English", "code", 20, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchJav zh english title: %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Fatalf("zh search should not match title_en: total=%d items=%#v", total, items)
	}

	jav.SetMetadataLanguage("en")
	items, total, err = SearchJav(ctx, nil, nil, "English", "code", 20, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchJav en title: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].TitleEn != "English Title" {
		t.Fatalf("unexpected en search result: total=%d items=%#v", total, items)
	}
	items, total, err = SearchJav(ctx, nil, nil, "日本語", "code", 20, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchJav en japanese title: %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Fatalf("en search should not match title: total=%d items=%#v", total, items)
	}
}

func TestJavIdolAPIFiltersCurrentLanguageIdols(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1710000000, 0).UTC()
	prevLang := jav.CurrentMetadataLanguage()
	t.Cleanup(func() {
		jav.SetMetadataLanguage(string(prevLang))
	})

	dir := models.Directory{Path: "/tmp/media"}
	if err := gdb.Create(&dir).Error; err != nil {
		t.Fatalf("create directory: %v", err)
	}

	javRec := models.Jav{Code: "IDOL-001", Title: "Idol Language Work", Provider: 1, FetchedAt: now}
	if err := gdb.Create(&javRec).Error; err != nil {
		t.Fatalf("create jav: %v", err)
	}

	japaneseIdol := models.JavIdol{Name: "一ノ瀬アメリ", IsEnglish: false}
	englishIdol := models.JavIdol{Name: "Ameri Ichinose (Ayaka Misora)", IsEnglish: true}
	if err := gdb.Create(&japaneseIdol).Error; err != nil {
		t.Fatalf("create japanese idol: %v", err)
	}
	if err := gdb.Create(&englishIdol).Error; err != nil {
		t.Fatalf("create english idol: %v", err)
	}
	if err := gdb.Create(&[]models.JavIdolMap{
		{JavID: javRec.ID, JavIdolID: japaneseIdol.ID},
		{JavID: javRec.ID, JavIdolID: englishIdol.ID},
	}).Error; err != nil {
		t.Fatalf("create idol maps: %v", err)
	}

	video := models.Video{
		DirectoryID: dir.ID,
		Path:        "idol-001.mp4",
		Filename:    "idol-001.mp4",
		Fingerprint: "fp-idol-language",
		DurationSec: 7200,
		ModifiedAt:  now,
	}
	if err := gdb.Create(&video).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}
	loc := models.VideoLocation{
		VideoID:      video.ID,
		DirectoryID:  dir.ID,
		RelativePath: "idol-001.mp4",
		ModifiedAt:   now,
		JavID:        int64Ptr(javRec.ID),
	}
	if err := gdb.Create(&loc).Error; err != nil {
		t.Fatalf("create video location: %v", err)
	}

	jav.SetMetadataLanguage("zh")
	idols, total, err := ListJavIdols(ctx, "", "work", 20, 0, nil)
	if err != nil {
		t.Fatalf("ListJavIdols zh: %v", err)
	}
	assertJavIdolSummaries(t, idols, total, []string{"一ノ瀬アメリ"})
	if _, err := GetJavIdolSummary(ctx, englishIdol.ID, nil); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected english idol to be hidden in zh mode, got err=%v", err)
	}

	jav.SetMetadataLanguage("en")
	idols, total, err = ListJavIdols(ctx, "", "work", 20, 0, nil)
	if err != nil {
		t.Fatalf("ListJavIdols en: %v", err)
	}
	assertJavIdolSummaries(t, idols, total, []string{"Ameri Ichinose (Ayaka Misora)"})
	if _, err := GetJavIdolSummary(ctx, japaneseIdol.ID, nil); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected japanese idol to be hidden in en mode, got err=%v", err)
	}
}

func TestListIdolsMissingProfileFiltersCurrentLanguage(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1710000000, 0).UTC()
	prevLang := jav.CurrentMetadataLanguage()
	t.Cleanup(func() {
		jav.SetMetadataLanguage(string(prevLang))
	})

	dir := models.Directory{Path: "/tmp/media"}
	if err := gdb.Create(&dir).Error; err != nil {
		t.Fatalf("create directory: %v", err)
	}
	javRec := models.Jav{Code: "MISS-001", Title: "Missing Profile Work", Provider: 1, FetchedAt: now}
	if err := gdb.Create(&javRec).Error; err != nil {
		t.Fatalf("create jav: %v", err)
	}
	japaneseIdol := models.JavIdol{Name: "あいか", IsEnglish: false}
	englishIdol := models.JavIdol{Name: "AIKA", IsEnglish: true}
	if err := gdb.Create(&japaneseIdol).Error; err != nil {
		t.Fatalf("create japanese idol: %v", err)
	}
	if err := gdb.Create(&englishIdol).Error; err != nil {
		t.Fatalf("create english idol: %v", err)
	}
	if err := gdb.Create(&[]models.JavIdolMap{
		{JavID: javRec.ID, JavIdolID: japaneseIdol.ID},
		{JavID: javRec.ID, JavIdolID: englishIdol.ID},
	}).Error; err != nil {
		t.Fatalf("create idol maps: %v", err)
	}
	video := models.Video{
		DirectoryID: dir.ID,
		Path:        "miss-001.mp4",
		Filename:    "miss-001.mp4",
		Fingerprint: "fp-missing-profile",
		DurationSec: 7200,
		ModifiedAt:  now,
	}
	if err := gdb.Create(&video).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}
	loc := models.VideoLocation{
		VideoID:      video.ID,
		DirectoryID:  dir.ID,
		RelativePath: "miss-001.mp4",
		ModifiedAt:   now,
		JavID:        int64Ptr(javRec.ID),
	}
	if err := gdb.Create(&loc).Error; err != nil {
		t.Fatalf("create video location: %v", err)
	}

	jav.SetMetadataLanguage("zh")
	idols, err := ListIdolsMissingProfile(ctx)
	if err != nil {
		t.Fatalf("ListIdolsMissingProfile zh: %v", err)
	}
	assertJavIdolNames(t, idols, []string{"あいか"})

	jav.SetMetadataLanguage("en")
	idols, err = ListIdolsMissingProfile(ctx)
	if err != nil {
		t.Fatalf("ListIdolsMissingProfile en: %v", err)
	}
	assertJavIdolNames(t, idols, []string{"AIKA"})
	code, err := FindIdolSoloCode(ctx, englishIdol.ID)
	if err != nil {
		t.Fatalf("FindIdolSoloCode english: %v", err)
	}
	if code != javRec.Code {
		t.Fatalf("unexpected english solo code: got %q want %q", code, javRec.Code)
	}
}

func TestJavBindingUsesVideoLocationsAndCountsTagWorks(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1710000000, 0).UTC()

	dir := models.Directory{Path: "/tmp/media"}
	if err := gdb.Create(&dir).Error; err != nil {
		t.Fatalf("create directory: %v", err)
	}
	video := models.Video{
		DirectoryID: dir.ID,
		Path:        "aaa-001.mp4",
		Filename:    "aaa-001.mp4",
		Fingerprint: "same-content-location-jav",
		DurationSec: 7200,
		ModifiedAt:  now,
	}
	if err := gdb.Create(&video).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}

	javA := models.Jav{Code: "AAA-001", Title: "A", Provider: 1, FetchedAt: now}
	javB := models.Jav{Code: "BBB-001", Title: "B", Provider: 1, FetchedAt: now}
	if err := gdb.Create(&javA).Error; err != nil {
		t.Fatalf("create jav a: %v", err)
	}
	if err := gdb.Create(&javB).Error; err != nil {
		t.Fatalf("create jav b: %v", err)
	}
	tag := models.JavTag{Name: "Location Count", Provider: 1}
	if err := gdb.Create(&tag).Error; err != nil {
		t.Fatalf("create jav tag: %v", err)
	}
	idol := models.JavIdol{Name: "Location Idol"}
	if err := gdb.Create(&idol).Error; err != nil {
		t.Fatalf("create idol: %v", err)
	}
	if err := gdb.Create(&[]models.JavTagMap{{JavID: javA.ID, JavTagID: tag.ID}}).Error; err != nil {
		t.Fatalf("create tag map: %v", err)
	}
	if err := gdb.Create(&[]models.JavIdolMap{
		{JavID: javA.ID, JavIdolID: idol.ID},
		{JavID: javB.ID, JavIdolID: idol.ID},
	}).Error; err != nil {
		t.Fatalf("create idol maps: %v", err)
	}

	locs := []models.VideoLocation{
		{VideoID: video.ID, DirectoryID: dir.ID, RelativePath: "aaa-001-a.mp4", ModifiedAt: now, JavID: int64Ptr(javA.ID)},
		{VideoID: video.ID, DirectoryID: dir.ID, RelativePath: "aaa-001-b.mp4", ModifiedAt: now.Add(time.Second), JavID: int64Ptr(javA.ID)},
		{VideoID: video.ID, DirectoryID: dir.ID, RelativePath: "bbb-001.mp4", ModifiedAt: now.Add(2 * time.Second), JavID: int64Ptr(javB.ID)},
	}
	if err := gdb.Create(&locs).Error; err != nil {
		t.Fatalf("create locations: %v", err)
	}

	items, total, err := SearchJav(ctx, nil, nil, "", "code", 20, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchJav: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("unexpected jav result size: len=%d total=%d", len(items), total)
	}
	byCode := map[string]models.Jav{}
	for _, item := range items {
		byCode[item.Code] = item
	}
	if got := len(byCode["AAA-001"].Videos); got != 2 {
		t.Fatalf("AAA-001 video locations = %d, want 2", got)
	}
	if got := len(byCode["BBB-001"].Videos); got != 1 {
		t.Fatalf("BBB-001 video locations = %d, want 1", got)
	}
	if byCode["AAA-001"].Videos[0].ID != video.ID || byCode["BBB-001"].Videos[0].ID != video.ID {
		t.Fatal("expected location-backed videos to keep the original video id")
	}

	tags, err := ListJavTags(ctx, nil)
	if err != nil {
		t.Fatalf("ListJavTags: %v", err)
	}
	tagCounts := map[string]int64{}
	for _, item := range tags {
		tagCounts[item.Name] = item.Count
	}
	if tagCounts[tag.Name] != 1 {
		t.Fatalf("tag count = %d, want 1", tagCounts[tag.Name])
	}

	idols, _, err := ListJavIdols(ctx, "", "work", 20, 0, nil)
	if err != nil {
		t.Fatalf("ListJavIdols: %v", err)
	}
	if len(idols) != 1 || idols[0].ID != idol.ID {
		t.Fatalf("unexpected idols: %#v", idols)
	}
	if idols[0].WorkCount != 2 {
		t.Fatalf("idol work count = %d, want 2", idols[0].WorkCount)
	}
}

func TestGetJavIdolSummaryReturnsSampleCodeAndWorkCount(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1710000000, 0).UTC()

	dir := models.Directory{Path: "/tmp/media"}
	if err := db.Create(&dir).Error; err != nil {
		t.Fatalf("create directory: %v", err)
	}

	idol := models.JavIdol{Name: "Preview Idol"}
	if err := db.Create(&idol).Error; err != nil {
		t.Fatalf("create idol: %v", err)
	}

	soloJav := models.Jav{Code: "DDD-001", Title: "Solo Work", Provider: 1, FetchedAt: now}
	groupJav := models.Jav{Code: "EEE-001", Title: "Group Work", Provider: 1, FetchedAt: now}
	coIdol := models.JavIdol{Name: "Other Idol"}
	if err := db.Create(&soloJav).Error; err != nil {
		t.Fatalf("create solo jav: %v", err)
	}
	if err := db.Create(&groupJav).Error; err != nil {
		t.Fatalf("create group jav: %v", err)
	}
	if err := db.Create(&coIdol).Error; err != nil {
		t.Fatalf("create co idol: %v", err)
	}

	maps := []models.JavIdolMap{
		{JavID: soloJav.ID, JavIdolID: idol.ID},
		{JavID: groupJav.ID, JavIdolID: idol.ID},
		{JavID: groupJav.ID, JavIdolID: coIdol.ID},
	}
	if err := db.Create(&maps).Error; err != nil {
		t.Fatalf("create idol maps: %v", err)
	}

	videos := []models.Video{
		{
			DirectoryID: dir.ID,
			Path:        "solo-preview.mp4",
			Filename:    "solo-preview.mp4",
			Fingerprint: "fp-solo-preview",
			JavID:       int64Ptr(soloJav.ID),
			ModifiedAt:  now,
		},
		{
			DirectoryID: dir.ID,
			Path:        "group-preview.mp4",
			Filename:    "group-preview.mp4",
			Fingerprint: "fp-group-preview",
			JavID:       int64Ptr(groupJav.ID),
			ModifiedAt:  now,
		},
	}
	if err := db.Create(&videos).Error; err != nil {
		t.Fatalf("create videos: %v", err)
	}
	createVideoLocationsForVideos(t, db, videos...)
	extraLocation := models.VideoLocation{
		VideoID:      videos[0].ID,
		DirectoryID:  dir.ID,
		RelativePath: "solo-preview-copy.mp4",
		Filename:     "solo-preview-copy.mp4",
		ModifiedAt:   now.Add(time.Second),
		JavID:        int64Ptr(soloJav.ID),
	}
	if err := db.Create(&extraLocation).Error; err != nil {
		t.Fatalf("create extra location: %v", err)
	}

	item, err := GetJavIdolSummary(ctx, idol.ID, nil)
	if err != nil {
		t.Fatalf("GetJavIdolSummary: %v", err)
	}
	if item.WorkCount != 2 {
		t.Fatalf("unexpected work count: got %d want 2", item.WorkCount)
	}
	if item.SampleCode != soloJav.Code {
		t.Fatalf("unexpected sample code: got %q want %q", item.SampleCode, soloJav.Code)
	}
}

func TestSearchJavSortByDurationDesc(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1710000000, 0).UTC()

	dir := models.Directory{Path: "/tmp/media"}
	if err := db.Create(&dir).Error; err != nil {
		t.Fatalf("create directory: %v", err)
	}

	shortJav := models.Jav{
		Code:        "FFF-001",
		Title:       "Short",
		DurationMin: 90,
		Provider:    1,
		FetchedAt:   now,
	}
	longJav := models.Jav{
		Code:        "GGG-001",
		Title:       "Long",
		DurationMin: 180,
		Provider:    1,
		FetchedAt:   now,
	}
	if err := db.Create(&shortJav).Error; err != nil {
		t.Fatalf("create short jav: %v", err)
	}
	if err := db.Create(&longJav).Error; err != nil {
		t.Fatalf("create long jav: %v", err)
	}

	videos := []models.Video{
		{
			DirectoryID: dir.ID,
			Path:        "short.mp4",
			Filename:    "short.mp4",
			Fingerprint: "fp-short",
			JavID:       int64Ptr(shortJav.ID),
			ModifiedAt:  now,
		},
		{
			DirectoryID: dir.ID,
			Path:        "long.mp4",
			Filename:    "long.mp4",
			Fingerprint: "fp-long",
			JavID:       int64Ptr(longJav.ID),
			ModifiedAt:  now,
		},
	}
	if err := db.Create(&videos).Error; err != nil {
		t.Fatalf("create videos: %v", err)
	}
	createVideoLocationsForVideos(t, db, videos...)

	items, total, err := SearchJav(ctx, nil, nil, "", "duration", 20, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchJav: %v", err)
	}
	if total != 2 {
		t.Fatalf("unexpected total: got %d want 2", total)
	}
	if len(items) != 2 {
		t.Fatalf("unexpected item count: got %d want 2", len(items))
	}
	if items[0].ID != longJav.ID {
		t.Fatalf("unexpected first jav: got %d want %d", items[0].ID, longJav.ID)
	}
	if items[1].ID != shortJav.ID {
		t.Fatalf("unexpected second jav: got %d want %d", items[1].ID, shortJav.ID)
	}

	items, total, err = SearchJav(ctx, nil, nil, "", "duration_asc", 20, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchJav duration_asc: %v", err)
	}
	if total != 2 {
		t.Fatalf("unexpected asc total: got %d want 2", total)
	}
	if len(items) != 2 {
		t.Fatalf("unexpected asc item count: got %d want 2", len(items))
	}
	if items[0].ID != shortJav.ID {
		t.Fatalf("unexpected asc first jav: got %d want %d", items[0].ID, shortJav.ID)
	}
	if items[1].ID != longJav.ID {
		t.Fatalf("unexpected asc second jav: got %d want %d", items[1].ID, longJav.ID)
	}
}

func TestListJavIdolsSortByAgeDirections(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1710000000, 0).UTC()
	oldBirth := time.Date(1988, 1, 1, 0, 0, 0, 0, time.UTC)
	youngBirth := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)

	dir := models.Directory{Path: "/tmp/media"}
	if err := db.Create(&dir).Error; err != nil {
		t.Fatalf("create directory: %v", err)
	}

	oldIdol := models.JavIdol{Name: "Old Idol", BirthDate: &oldBirth}
	youngIdol := models.JavIdol{Name: "Young Idol", BirthDate: &youngBirth}
	if err := db.Create(&oldIdol).Error; err != nil {
		t.Fatalf("create old idol: %v", err)
	}
	if err := db.Create(&youngIdol).Error; err != nil {
		t.Fatalf("create young idol: %v", err)
	}

	oldJav := models.Jav{Code: "HHH-001", Title: "Old Solo", Provider: 1, FetchedAt: now}
	youngJav := models.Jav{Code: "III-001", Title: "Young Solo", Provider: 1, FetchedAt: now}
	if err := db.Create(&oldJav).Error; err != nil {
		t.Fatalf("create old jav: %v", err)
	}
	if err := db.Create(&youngJav).Error; err != nil {
		t.Fatalf("create young jav: %v", err)
	}

	maps := []models.JavIdolMap{
		{JavID: oldJav.ID, JavIdolID: oldIdol.ID},
		{JavID: youngJav.ID, JavIdolID: youngIdol.ID},
	}
	if err := db.Create(&maps).Error; err != nil {
		t.Fatalf("create idol maps: %v", err)
	}

	videos := []models.Video{
		{
			DirectoryID: dir.ID,
			Path:        "old.mp4",
			Filename:    "old.mp4",
			Fingerprint: "fp-old",
			JavID:       int64Ptr(oldJav.ID),
			ModifiedAt:  now,
		},
		{
			DirectoryID: dir.ID,
			Path:        "young.mp4",
			Filename:    "young.mp4",
			Fingerprint: "fp-young",
			JavID:       int64Ptr(youngJav.ID),
			ModifiedAt:  now,
		},
	}
	if err := db.Create(&videos).Error; err != nil {
		t.Fatalf("create videos: %v", err)
	}
	createVideoLocationsForVideos(t, db, videos...)

	items, total, err := ListJavIdols(ctx, "", "birth", 20, 0, nil)
	if err != nil {
		t.Fatalf("ListJavIdols birth: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("unexpected birth result size: len=%d total=%d", len(items), total)
	}
	if items[0].ID != youngIdol.ID {
		t.Fatalf("unexpected birth first idol: got %d want %d", items[0].ID, youngIdol.ID)
	}

	items, total, err = ListJavIdols(ctx, "", "birth_asc", 20, 0, nil)
	if err != nil {
		t.Fatalf("ListJavIdols birth_asc: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("unexpected birth_asc result size: len=%d total=%d", len(items), total)
	}
	if items[0].ID != oldIdol.ID {
		t.Fatalf("unexpected birth_asc first idol: got %d want %d", items[0].ID, oldIdol.ID)
	}
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	prevDB := common.DB
	common.DB = db
	t.Cleanup(func() {
		common.DB = prevDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func int64Ptr(v int64) *int64 {
	return &v
}

func assertJavIdolMaps(t *testing.T, db *gorm.DB, code string, want map[string]bool) {
	t.Helper()

	var rows []struct {
		Name      string
		IsEnglish bool
	}
	if err := db.Table("jav_idol_map jim").
		Select("ji.name, ji.is_english").
		Joins("JOIN jav j ON j.id = jim.jav_id").
		Joins("JOIN jav_idol ji ON ji.id = jim.jav_idol_id").
		Where("j.code = ?", code).
		Order("ji.name").
		Scan(&rows).Error; err != nil {
		t.Fatalf("list jav idol maps: %v", err)
	}
	if len(rows) != len(want) {
		t.Fatalf("unexpected idol map count: got %d want %d rows=%#v", len(rows), len(want), rows)
	}
	for _, row := range rows {
		wantEnglish, ok := want[row.Name]
		if !ok {
			t.Fatalf("unexpected idol map row: %#v", row)
		}
		if row.IsEnglish != wantEnglish {
			t.Fatalf("unexpected is_english for %q: got %t want %t", row.Name, row.IsEnglish, wantEnglish)
		}
	}
}

func assertJavTitles(t *testing.T, db *gorm.DB, code, wantTitle, wantTitleEn string) {
	t.Helper()

	var rec models.Jav
	if err := db.Where("code = ?", code).First(&rec).Error; err != nil {
		t.Fatalf("load jav %q: %v", code, err)
	}
	if rec.Title != wantTitle || rec.TitleEn != wantTitleEn {
		t.Fatalf("unexpected titles for %q: title=%q title_en=%q want title=%q title_en=%q", code, rec.Title, rec.TitleEn, wantTitle, wantTitleEn)
	}
}

func assertJavStudio(t *testing.T, db *gorm.DB, code, want string) {
	t.Helper()

	var rec models.Jav
	if err := db.Preload("Studio").Where("code = ?", code).First(&rec).Error; err != nil {
		t.Fatalf("load jav %q: %v", code, err)
	}
	if rec.Studio == nil {
		t.Fatalf("missing studio for %q", code)
	}
	if rec.Studio.Name != want {
		t.Fatalf("unexpected studio for %q: got %q want %q", code, rec.Studio.Name, want)
	}
}

func assertJavIdolLanguageCount(t *testing.T, db *gorm.DB, name string, want int64) {
	t.Helper()

	var got int64
	if err := db.Model(&models.JavIdol{}).Where("name = ?", name).Count(&got).Error; err != nil {
		t.Fatalf("count jav idol languages: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected language row count for %q: got %d want %d", name, got, want)
	}
}

func assertJavIdolMapLanguages(t *testing.T, db *gorm.DB, code, name string, want []bool) {
	t.Helper()

	var got []bool
	if err := db.Table("jav_idol_map jim").
		Select("ji.is_english").
		Joins("JOIN jav j ON j.id = jim.jav_id").
		Joins("JOIN jav_idol ji ON ji.id = jim.jav_idol_id").
		Where("j.code = ? AND ji.name = ?", code, name).
		Order("ji.is_english").
		Pluck("ji.is_english", &got).Error; err != nil {
		t.Fatalf("list jav idol map languages: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("unexpected language count for %q/%q: got %v want %v", code, name, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected language at %d for %q/%q: got %v want %v", i, code, name, got, want)
		}
	}
}

func assertJavIdolNames(t *testing.T, idols []models.JavIdol, want []string) {
	t.Helper()

	if len(idols) != len(want) {
		t.Fatalf("unexpected idol count: got %d want %d idols=%#v", len(idols), len(want), idols)
	}
	for i, name := range want {
		if idols[i].Name != name {
			t.Fatalf("unexpected idol at %d: got %q want %q", i, idols[i].Name, name)
		}
	}
}

func assertJavTagMaps(t *testing.T, db *gorm.DB, code string, want map[string]int) {
	t.Helper()

	var rows []struct {
		Name     string
		Provider int
	}
	if err := db.Table("jav_tag_map jtm").
		Select("jt.name, jt.provider").
		Joins("JOIN jav j ON j.id = jtm.jav_id").
		Joins("JOIN jav_tag jt ON jt.id = jtm.jav_tag_id").
		Where("j.code = ?", code).
		Order("jt.name").
		Scan(&rows).Error; err != nil {
		t.Fatalf("list jav tag maps: %v", err)
	}
	if len(rows) != len(want) {
		t.Fatalf("unexpected tag map count: got %d want %d rows=%#v", len(rows), len(want), rows)
	}
	for _, row := range rows {
		wantProvider, ok := want[row.Name]
		if !ok {
			t.Fatalf("unexpected tag map row: %#v", row)
		}
		if row.Provider != wantProvider {
			t.Fatalf("unexpected provider for %q: got %d want %d", row.Name, row.Provider, wantProvider)
		}
	}
}

func assertSearchJavIdols(t *testing.T, items []models.Jav, total int64, want []string) {
	t.Helper()

	if total != 1 || len(items) != 1 {
		t.Fatalf("unexpected jav result size: len=%d total=%d", len(items), total)
	}
	if len(items[0].Idols) != len(want) {
		t.Fatalf("unexpected idol count: got %d want %d idols=%#v", len(items[0].Idols), len(want), items[0].Idols)
	}
	for i, name := range want {
		if items[0].Idols[i].Name != name {
			t.Fatalf("unexpected idol at %d: got %q want %q", i, items[0].Idols[i].Name, name)
		}
	}
}

func assertJavIdolSummaries(t *testing.T, items []JavIdolSummary, total int64, want []string) {
	t.Helper()

	if total != int64(len(want)) || len(items) != len(want) {
		t.Fatalf("unexpected idol result size: len=%d total=%d want=%d items=%#v", len(items), total, len(want), items)
	}
	for i, name := range want {
		if items[i].Name != name {
			t.Fatalf("unexpected idol at %d: got %q want %q", i, items[i].Name, name)
		}
	}
}
