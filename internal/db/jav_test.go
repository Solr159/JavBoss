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

	soloJav := models.Jav{Code: "AAA-001", Title: "Solo Work", FetchedAt: now}
	groupJav := models.Jav{Code: "BBB-001", Title: "Group Work", FetchedAt: now}
	unavailableSoloJav := models.Jav{Code: "CCC-001", Title: "Unavailable Solo Work", FetchedAt: now}
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

	items, total, err := ListJavIdols(ctx, "", "", 20, 0, nil, 0)
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

func TestDeleteJavIdolFavoriteGroupCascadesMapsOnNewConnection(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	group := models.JavIdolFavoriteGroup{Name: "Favorites"}
	otherGroup := models.JavIdolFavoriteGroup{Name: "Other"}
	if err := db.Create(&[]models.JavIdolFavoriteGroup{group, otherGroup}).Error; err != nil {
		t.Fatalf("create favorite groups: %v", err)
	}
	var groups []models.JavIdolFavoriteGroup
	if err := db.Order("name").Find(&groups).Error; err != nil {
		t.Fatalf("load favorite groups: %v", err)
	}
	group = groups[0]
	otherGroup = groups[1]

	idolA := models.JavIdol{Name: "Idol A"}
	idolB := models.JavIdol{Name: "Idol B"}
	if err := db.Create(&[]models.JavIdol{idolA, idolB}).Error; err != nil {
		t.Fatalf("create idols: %v", err)
	}
	var idols []models.JavIdol
	if err := db.Order("name").Find(&idols).Error; err != nil {
		t.Fatalf("load idols: %v", err)
	}
	idolA = idols[0]
	idolB = idols[1]

	rows := []models.JavIdolFavoriteMap{
		{JavIdolFavoriteGroupID: group.ID, JavIdolID: idolA.ID},
		{JavIdolFavoriteGroupID: group.ID, JavIdolID: idolB.ID},
		{JavIdolFavoriteGroupID: otherGroup.ID, JavIdolID: idolA.ID},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("create favorite maps: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxIdleConns(0)
	t.Cleanup(func() {
		sqlDB.SetMaxIdleConns(2)
	})

	var foreignKeysEnabled int
	if err := db.Raw("PRAGMA foreign_keys").Scan(&foreignKeysEnabled).Error; err != nil {
		t.Fatalf("read foreign_keys pragma: %v", err)
	}
	if foreignKeysEnabled != 1 {
		t.Fatalf("foreign_keys pragma = %d, want 1", foreignKeysEnabled)
	}

	if err := DeleteJavIdolFavoriteGroup(ctx, group.ID); err != nil {
		t.Fatalf("DeleteJavIdolFavoriteGroup: %v", err)
	}

	var deletedGroupMaps int64
	if err := db.Model(&models.JavIdolFavoriteMap{}).
		Where("jav_idol_favorite_group_id = ?", group.ID).
		Count(&deletedGroupMaps).Error; err != nil {
		t.Fatalf("count deleted group maps: %v", err)
	}
	if deletedGroupMaps != 0 {
		t.Fatalf("deleted group maps remain: got %d want 0", deletedGroupMaps)
	}

	var otherGroupMaps int64
	if err := db.Model(&models.JavIdolFavoriteMap{}).
		Where("jav_idol_favorite_group_id = ?", otherGroup.ID).
		Count(&otherGroupMaps).Error; err != nil {
		t.Fatalf("count other group maps: %v", err)
	}
	if otherGroupMaps != 1 {
		t.Fatalf("unexpected other group maps: got %d want 1", otherGroupMaps)
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
		{Code: "IDA-001", Title: "A and B", FetchedAt: now},
		{Code: "IDA-002", Title: "A only", FetchedAt: now},
		{Code: "IDC-001", Title: "C only", FetchedAt: now},
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
		{Code: "STA-001", Title: "Studio A One", StudioID: int64Ptr(studioA.ID), FetchedAt: now},
		{Code: "STA-002", Title: "Studio A Two", StudioID: int64Ptr(studioA.ID), FetchedAt: now},
		{Code: "STB-001", Title: "Studio B One", StudioID: int64Ptr(studioB.ID), FetchedAt: now},
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

func TestListJavSeriesAndSearchBySeries(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1710000000, 0).UTC()
	prevLang := jav.CurrentMetadataLanguage()
	t.Cleanup(func() {
		jav.SetMetadataLanguage(string(prevLang))
	})

	dir := models.Directory{Path: "/tmp/media"}
	if err := db.Create(&dir).Error; err != nil {
		t.Fatalf("create directory: %v", err)
	}

	seriesA := models.JavSeries{Name: "Series A"}
	seriesB := models.JavSeries{Name: "Series B", IsEnglish: true}
	studioA := models.JavStudio{Name: "Series Studio"}
	if err := db.Create(&studioA).Error; err != nil {
		t.Fatalf("create studio a: %v", err)
	}
	seriesA.StudioID = &studioA.ID
	if err := db.Create(&seriesA).Error; err != nil {
		t.Fatalf("create series a: %v", err)
	}
	if err := db.Create(&seriesB).Error; err != nil {
		t.Fatalf("create series b: %v", err)
	}

	javs := []models.Jav{
		{Code: "SRA-001", Title: "Series A One", SeriesID: int64Ptr(seriesA.ID), FetchedAt: now},
		{Code: "SRA-002", Title: "Series A Two", SeriesID: int64Ptr(seriesA.ID), FetchedAt: now},
		{Code: "SRB-001", Title: "Series B One", SeriesEnID: int64Ptr(seriesB.ID), FetchedAt: now},
	}
	if err := db.Create(&javs).Error; err != nil {
		t.Fatalf("create javs: %v", err)
	}

	videos := []models.Video{
		{DirectoryID: dir.ID, Path: "sra-001.mp4", Filename: "sra-001.mp4", Fingerprint: "fp-sra-001", JavID: int64Ptr(javs[0].ID), ModifiedAt: now},
		{DirectoryID: dir.ID, Path: "sra-002.mp4", Filename: "sra-002.mp4", Fingerprint: "fp-sra-002", JavID: int64Ptr(javs[1].ID), ModifiedAt: now},
		{DirectoryID: dir.ID, Path: "srb-001.mp4", Filename: "srb-001.mp4", Fingerprint: "fp-srb-001", JavID: int64Ptr(javs[2].ID), ModifiedAt: now},
	}
	if err := db.Create(&videos).Error; err != nil {
		t.Fatalf("create videos: %v", err)
	}
	createVideoLocationsForVideos(t, db, videos...)

	jav.SetMetadataLanguage("zh")
	series, total, err := ListJavSeries(ctx, "", 20, 0, nil)
	if err != nil {
		t.Fatalf("ListJavSeries: %v", err)
	}
	if total != 1 {
		t.Fatalf("unexpected zh series total: got %d want 1", total)
	}
	if len(series) != 1 {
		t.Fatalf("unexpected zh series count: got %d want 1", len(series))
	}
	if series[0].ID != seriesA.ID || series[0].WorkCount != 2 {
		t.Fatalf("unexpected zh series: %#v", series[0])
	}
	if series[0].StudioID == nil || *series[0].StudioID != studioA.ID || series[0].StudioName != studioA.Name {
		t.Fatalf("unexpected zh series studio: %#v", series[0])
	}
	if series[0].SampleCode == "" {
		t.Fatalf("expected sample code for zh series")
	}

	items, total, err := SearchJav(ctx, nil, nil, "", "code", 20, 0, nil, nil, 0, seriesA.ID)
	if err != nil {
		t.Fatalf("SearchJav by zh series: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("unexpected zh filtered javs: total=%d len=%d", total, len(items))
	}
	for _, item := range items {
		if item.SeriesID == nil || *item.SeriesID != seriesA.ID {
			t.Fatalf("unexpected zh series filtered item: %#v", item)
		}
		if item.Series == nil || item.SeriesEn != nil {
			t.Fatalf("unexpected zh series preload: %#v", item)
		}
	}

	codes, err := ListSeriesCoverCodes(ctx, seriesA.ID, nil)
	if err != nil {
		t.Fatalf("ListSeriesCoverCodes zh: %v", err)
	}
	if len(codes) != 2 {
		t.Fatalf("unexpected zh cover codes: %#v", codes)
	}

	jav.SetMetadataLanguage("en")
	series, total, err = ListJavSeries(ctx, "", 20, 0, nil)
	if err != nil {
		t.Fatalf("ListJavSeries en: %v", err)
	}
	if total != 1 {
		t.Fatalf("unexpected en series total: got %d want 1", total)
	}
	if len(series) != 1 {
		t.Fatalf("unexpected en series count: got %d want 1", len(series))
	}
	if series[0].ID != seriesB.ID || series[0].WorkCount != 1 || !series[0].IsEnglish {
		t.Fatalf("unexpected en series: %#v", series[0])
	}

	items, total, err = SearchJav(ctx, nil, nil, "", "code", 20, 0, nil, nil, 0, seriesA.ID)
	if err != nil {
		t.Fatalf("SearchJav by zh series in en mode: %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Fatalf("unexpected zh series match in en mode: total=%d len=%d", total, len(items))
	}

	items, total, err = SearchJav(ctx, nil, nil, "", "code", 20, 0, nil, nil, 0, seriesB.ID)
	if err != nil {
		t.Fatalf("SearchJav by en series: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("unexpected en filtered javs: total=%d len=%d", total, len(items))
	}
	if items[0].Series != nil || items[0].SeriesEn == nil || items[0].SeriesEn.ID != seriesB.ID {
		t.Fatalf("unexpected en series preload: %#v", items[0])
	}

	codes, err = ListSeriesCoverCodes(ctx, seriesA.ID, nil)
	if err != nil {
		t.Fatalf("ListSeriesCoverCodes zh series in en mode: %v", err)
	}
	if len(codes) != 0 {
		t.Fatalf("unexpected zh cover codes in en mode: %#v", codes)
	}
	codes, err = ListSeriesCoverCodes(ctx, seriesB.ID, nil)
	if err != nil {
		t.Fatalf("ListSeriesCoverCodes en: %v", err)
	}
	if len(codes) != 1 || codes[0] != "SRB-001" {
		t.Fatalf("unexpected en cover codes: %#v", codes)
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

func TestAppendJavIdolsIfMissingForProvider(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1710000000, 0).UTC()

	javRec := models.Jav{Code: "AVS-001", Title: "Kept Title", FetchedAt: now}
	if err := gdb.Create(&javRec).Error; err != nil {
		t.Fatalf("create jav: %v", err)
	}

	updated, err := AppendJavIdolsIfMissingForProvider(ctx, javRec.ID, []string{"小橋りえこ", "小橋りえこ"}, jav.ProviderAvsox)
	if err != nil {
		t.Fatalf("AppendJavIdolsIfMissingForProvider: %v", err)
	}
	if !updated {
		t.Fatal("expected idol map update")
	}
	assertJavIdolMaps(t, gdb, "AVS-001", map[string]bool{
		"小橋りえこ": false,
	})
	assertJavTitles(t, gdb, "AVS-001", "Kept Title", "")

	updated, err = AppendJavIdolsIfMissingForProvider(ctx, javRec.ID, []string{"別の女優"}, jav.ProviderAvsox)
	if err != nil {
		t.Fatalf("AppendJavIdolsIfMissingForProvider second call: %v", err)
	}
	if updated {
		t.Fatal("expected existing local idol map to be preserved")
	}
	assertJavIdolMaps(t, gdb, "AVS-001", map[string]bool{
		"小橋りえこ": false,
	})
}

func TestSaveJavInfoPersistsUncensoredState(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
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
	assertState := func(code string, want *bool) {
		t.Helper()
		var rec models.Jav
		if err := gdb.Where("code = ?", code).First(&rec).Error; err != nil {
			t.Fatalf("load jav %s: %v", code, err)
		}
		if want == nil {
			if rec.IsUncensored != nil {
				t.Fatalf("%s is_uncensored = %v, want nil", code, *rec.IsUncensored)
			}
			return
		}
		if rec.IsUncensored == nil || *rec.IsUncensored != *want {
			t.Fatalf("%s is_uncensored = %v, want %v", code, rec.IsUncensored, *want)
		}
	}

	uncensored := true
	censored := false
	save(&jav.JavInfo{Code: "UNC-001", Title: "Uncensored", IsUncensored: &uncensored, Provider: jav.ProviderJavBus})
	save(&jav.JavInfo{Code: "CEN-001", Title: "Censored", IsUncensored: &censored, Provider: jav.ProviderJavBus})
	save(&jav.JavInfo{Code: "UNK-001", Title: "Unknown", Provider: jav.ProviderJavDatabase})

	assertState("UNC-001", &uncensored)
	assertState("CEN-001", &censored)
	assertState("UNK-001", nil)

	save(&jav.JavInfo{Code: "UNC-001", Title: "Unknown refresh", Provider: jav.ProviderJavDatabase})
	assertState("UNC-001", &uncensored)

	var unknownRec models.Jav
	if err := gdb.Where("code = ?", "UNK-001").First(&unknownRec).Error; err != nil {
		t.Fatalf("load unknown jav: %v", err)
	}
	if err := UpdateJavIsUncensoredIfUnknown(ctx, unknownRec.ID, uncensored); err != nil {
		t.Fatalf("update unknown is_uncensored: %v", err)
	}
	assertState("UNK-001", &uncensored)
	if err := UpdateJavIsUncensoredIfUnknown(ctx, unknownRec.ID, censored); err != nil {
		t.Fatalf("update known is_uncensored: %v", err)
	}
	assertState("UNK-001", &uncensored)
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
	englishTag := models.JavTag{Name: "Plot Based"}
	userTag := models.JavTag{Name: "Favorite", IsUser: true}
	if err := gdb.Create(&englishTag).Error; err != nil {
		t.Fatalf("create english tag: %v", err)
	}
	if err := gdb.Create(&userTag).Error; err != nil {
		t.Fatalf("create user tag: %v", err)
	}
	if err := gdb.Create(&[]models.JavTagMap{
		{JavID: javRec.ID, JavTagID: englishTag.ID, Provider: int(jav.ProviderJavDatabase)},
		{JavID: javRec.ID, JavTagID: userTag.ID, Provider: int(jav.ProviderUser)},
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

func TestUserJavTagNameDoesNotModifyScrapedTag(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1710000000, 0).UTC()

	scraped := models.JavTag{Name: "Shared Name"}
	user := models.JavTag{Name: "Shared Name", IsUser: true}
	if err := gdb.Create(&scraped).Error; err != nil {
		t.Fatalf("create scraped tag: %v", err)
	}
	if err := gdb.Create(&user).Error; err != nil {
		t.Fatalf("create user tag: %v", err)
	}
	javRec := models.Jav{Code: "USR-001", Title: "User Tag", FetchedAt: now}
	if err := gdb.Create(&javRec).Error; err != nil {
		t.Fatalf("create jav: %v", err)
	}
	if err := gdb.Create(&[]models.JavTagMap{
		{JavID: javRec.ID, JavTagID: scraped.ID, Provider: int(jav.ProviderJavBus), CreatedAt: now},
		{JavID: javRec.ID, JavTagID: user.ID, Provider: int(jav.ProviderUser), CreatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create tag maps: %v", err)
	}

	if err := RenameJavTag(ctx, user.ID, "Renamed User"); err != nil {
		t.Fatalf("RenameJavTag user: %v", err)
	}
	var scrapedAfter models.JavTag
	if err := gdb.First(&scrapedAfter, scraped.ID).Error; err != nil {
		t.Fatalf("load scraped tag: %v", err)
	}
	if scrapedAfter.Name != "Shared Name" || scrapedAfter.IsUser {
		t.Fatalf("scraped tag was modified: %#v", scrapedAfter)
	}
	var userAfter models.JavTag
	if err := gdb.First(&userAfter, user.ID).Error; err != nil {
		t.Fatalf("load user tag: %v", err)
	}
	if userAfter.Name != "Renamed User" || !userAfter.IsUser {
		t.Fatalf("user tag was not renamed correctly: %#v", userAfter)
	}
}

func TestCreatedUserJavTagAppearsWithZeroCount(t *testing.T) {
	openTestDB(t)
	ctx := context.Background()

	created, err := CreateJavTag(ctx, "Empty User Tag")
	if err != nil {
		t.Fatalf("CreateJavTag: %v", err)
	}

	tags, err := ListJavTags(ctx, nil)
	if err != nil {
		t.Fatalf("ListJavTags: %v", err)
	}
	for _, tag := range tags {
		if tag.ID != created.ID {
			continue
		}
		if tag.Name != "Empty User Tag" || tag.Provider != int(jav.ProviderUser) || tag.Count != 0 {
			t.Fatalf("unexpected created tag row: %#v", tag)
		}
		return
	}
	t.Fatalf("created user tag not listed: %#v", tags)
}

func TestJavTagsFilterProvidersByCurrentLanguage(t *testing.T) {
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
	javRec := models.Jav{Code: "LANG-001", Title: "Language Tags", TitleEn: "Language Tags EN", FetchedAt: now}
	if err := gdb.Create(&javRec).Error; err != nil {
		t.Fatalf("create jav: %v", err)
	}
	javRec2 := models.Jav{Code: "LANG-002", Title: "Language Tags 2", TitleEn: "Language Tags EN 2", FetchedAt: now}
	if err := gdb.Create(&javRec2).Error; err != nil {
		t.Fatalf("create jav 2: %v", err)
	}
	videos := []models.Video{
		{DirectoryID: dir.ID, Path: "lang-001.mp4", Filename: "lang-001.mp4", Fingerprint: "fp-lang-001", JavID: int64Ptr(javRec.ID), ModifiedAt: now},
		{DirectoryID: dir.ID, Path: "lang-002.mp4", Filename: "lang-002.mp4", Fingerprint: "fp-lang-002", JavID: int64Ptr(javRec2.ID), ModifiedAt: now},
	}
	if err := gdb.Create(&videos).Error; err != nil {
		t.Fatalf("create videos: %v", err)
	}
	createVideoLocationsForVideos(t, gdb, videos...)

	tags := []models.JavTag{
		{Name: "Shared"},
		{Name: "JavDB Only"},
		{Name: "Avmoo Only"},
		{Name: "Avsox Only"},
		{Name: "English Only"},
		{Name: "TPDB Only"},
		{Name: "User Only", IsUser: true},
	}
	if err := gdb.Create(&tags).Error; err != nil {
		t.Fatalf("create tags: %v", err)
	}
	byName := map[string]models.JavTag{}
	for _, tag := range tags {
		byName[tag.Name] = tag
	}
	maps := []models.JavTagMap{
		{JavID: javRec.ID, JavTagID: byName["Shared"].ID, Provider: int(jav.ProviderJavBus), CreatedAt: now},
		{JavID: javRec2.ID, JavTagID: byName["Shared"].ID, Provider: int(jav.ProviderJavDB), CreatedAt: now},
		{JavID: javRec.ID, JavTagID: byName["JavDB Only"].ID, Provider: int(jav.ProviderJavDB), CreatedAt: now},
		{JavID: javRec.ID, JavTagID: byName["Avmoo Only"].ID, Provider: int(jav.ProviderAvmoo), CreatedAt: now},
		{JavID: javRec.ID, JavTagID: byName["Avsox Only"].ID, Provider: int(jav.ProviderAvsox), CreatedAt: now},
		{JavID: javRec.ID, JavTagID: byName["English Only"].ID, Provider: int(jav.ProviderJavDatabase), CreatedAt: now},
		{JavID: javRec.ID, JavTagID: byName["TPDB Only"].ID, Provider: int(jav.ProviderThePornDB), CreatedAt: now},
		{JavID: javRec.ID, JavTagID: byName["User Only"].ID, Provider: int(jav.ProviderUser), CreatedAt: now},
	}
	if err := gdb.Create(&maps).Error; err != nil {
		t.Fatalf("create tag maps: %v", err)
	}

	jav.SetMetadataLanguage("zh")
	zhTags, err := ListJavTags(ctx, nil)
	if err != nil {
		t.Fatalf("ListJavTags zh: %v", err)
	}
	assertJavTagProviderNames(t, zhTags, map[int][]string{
		int(jav.ProviderJavBus): {"Avmoo Only", "Avsox Only", "JavDB Only", "Shared"},
		int(jav.ProviderUser):   {"User Only"},
	})
	assertJavTagCounts(t, zhTags, map[string]int64{
		"Shared":     2,
		"JavDB Only": 1,
		"Avmoo Only": 1,
		"Avsox Only": 1,
		"User Only":  1,
	})
	items, total, err := SearchJav(ctx, nil, []int64{byName["JavDB Only"].ID}, "", "code", 20, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchJav zh tag: %v", err)
	}
	if total != 1 || len(items) != 1 || len(items[0].Tags) != 5 {
		t.Fatalf("unexpected zh search result: total=%d items=%#v", total, items)
	}

	jav.SetMetadataLanguage("en")
	enTags, err := ListJavTags(ctx, nil)
	if err != nil {
		t.Fatalf("ListJavTags en: %v", err)
	}
	assertJavTagProviderNames(t, enTags, map[int][]string{
		int(jav.ProviderJavDatabase): {"English Only", "TPDB Only"},
		int(jav.ProviderUser):        {"User Only"},
	})
	items, total, err = SearchJav(ctx, nil, []int64{byName["JavDB Only"].ID}, "", "code", 20, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchJav en zh-provider tag: %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Fatalf("unexpected zh-provider tag match in en mode: total=%d items=%#v", total, items)
	}
}

func TestSaveAndUpdateJavStudioAndSeries(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1710000000, 0).UTC()

	if err := gdb.Transaction(func(tx *gorm.DB) error {
		_, err := saveJavInfoTx(tx, &jav.JavInfo{
			Code:     "STU-001",
			Title:    "Studio metadata",
			Studio:   "Idea Pocket",
			Series:   "Beautiful Girl Series",
			Provider: jav.ProviderJavDatabase,
		}, now)
		return err
	}); err != nil {
		t.Fatalf("save jav info: %v", err)
	}
	assertJavStudio(t, gdb, "STU-001", "Idea Pocket")
	assertJavSeries(t, gdb, "STU-001", "Beautiful Girl Series", true)

	plainJav := models.Jav{Code: "STU-002", Title: "Missing studio", FetchedAt: now}
	if err := gdb.Create(&plainJav).Error; err != nil {
		t.Fatalf("create jav: %v", err)
	}
	if err := UpdateJavStudio(ctx, plainJav.ID, "S1 No. 1 Style"); err != nil {
		t.Fatalf("update jav studio: %v", err)
	}
	assertJavStudio(t, gdb, "STU-002", "S1 No. 1 Style")
	if err := UpdateJavSeries(ctx, plainJav.ID, "中年オヤジ", false); err != nil {
		t.Fatalf("update jav series: %v", err)
	}
	assertJavSeries(t, gdb, "STU-002", "中年オヤジ", false)
	var updatedJav models.Jav
	if err := gdb.Preload("Series").Where("code = ?", "STU-002").First(&updatedJav).Error; err != nil {
		t.Fatalf("load updated jav: %v", err)
	}
	if updatedJav.StudioID == nil || updatedJav.Series == nil || updatedJav.Series.StudioID == nil || *updatedJav.Series.StudioID != *updatedJav.StudioID {
		t.Fatalf("series studio was not initialized from jav studio: %#v", updatedJav.Series)
	}
}

func TestListJavsMissingTitle(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1710000000, 0).UTC()

	rows := []models.Jav{
		{Code: "MISS-001", FetchedAt: now, CreatedAt: now},
		{Code: "MISS-002", Title: "  ", TitleEn: "English Title", FetchedAt: now.Add(time.Second), CreatedAt: now.Add(time.Second)},
		{Code: "HAVE-001", Title: "中文标题", FetchedAt: now.Add(2 * time.Second), CreatedAt: now.Add(2 * time.Second)},
		{Code: "", FetchedAt: now.Add(3 * time.Second), CreatedAt: now.Add(3 * time.Second)},
	}
	if err := gdb.Create(&rows).Error; err != nil {
		t.Fatalf("create jav rows: %v", err)
	}

	items, err := ListJavsMissingTitle(ctx)
	if err != nil {
		t.Fatalf("ListJavsMissingTitle: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("unexpected item count: got %d want 2", len(items))
	}
	if items[0].Code != "MISS-001" || items[1].Code != "MISS-002" {
		t.Fatalf("unexpected codes: got %q, %q", items[0].Code, items[1].Code)
	}
}

func TestListJavsMissingUncensored(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1710000000, 0).UTC()
	uncensored := true
	censored := false

	rows := []models.Jav{
		{Code: "MISS-001", FetchedAt: now, CreatedAt: now},
		{Code: "MISS-002", FetchedAt: now.Add(time.Second), CreatedAt: now.Add(time.Second)},
		{Code: "UNC-001", IsUncensored: &uncensored, FetchedAt: now.Add(2 * time.Second), CreatedAt: now.Add(2 * time.Second)},
		{Code: "CEN-001", IsUncensored: &censored, FetchedAt: now.Add(3 * time.Second), CreatedAt: now.Add(3 * time.Second)},
		{Code: "", FetchedAt: now.Add(4 * time.Second), CreatedAt: now.Add(4 * time.Second)},
	}
	if err := gdb.Create(&rows).Error; err != nil {
		t.Fatalf("create jav rows: %v", err)
	}

	items, err := ListJavsMissingUncensored(ctx)
	if err != nil {
		t.Fatalf("ListJavsMissingUncensored: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("unexpected item count: got %d want 2", len(items))
	}
	if items[0].Code != "MISS-001" || items[1].Code != "MISS-002" {
		t.Fatalf("unexpected codes: got %q, %q", items[0].Code, items[1].Code)
	}
}

func TestListUncensoredJavsMissingAvsoxMetadata(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1710000000, 0).UTC()
	uncensored := true
	censored := false

	studio := models.JavStudio{Name: "Studio A"}
	if err := gdb.Create(&studio).Error; err != nil {
		t.Fatalf("create studio: %v", err)
	}
	series := models.JavSeries{Name: "Series A", StudioID: &studio.ID}
	if err := gdb.Create(&series).Error; err != nil {
		t.Fatalf("create series: %v", err)
	}

	rows := []models.Jav{
		{Code: "MISS-BOTH", IsUncensored: &uncensored, FetchedAt: now, CreatedAt: now},
		{Code: "MISS-STUDIO", IsUncensored: &uncensored, SeriesID: &series.ID, FetchedAt: now.Add(time.Second), CreatedAt: now.Add(time.Second)},
		{Code: "MISS-SERIES", IsUncensored: &uncensored, StudioID: &studio.ID, FetchedAt: now.Add(2 * time.Second), CreatedAt: now.Add(2 * time.Second)},
		{Code: "MISS-IDOLS", IsUncensored: &uncensored, StudioID: &studio.ID, SeriesID: &series.ID, FetchedAt: now.Add(3 * time.Second), CreatedAt: now.Add(3 * time.Second)},
		{Code: "HAVE-ALL", IsUncensored: &uncensored, StudioID: &studio.ID, SeriesID: &series.ID, FetchedAt: now.Add(4 * time.Second), CreatedAt: now.Add(4 * time.Second)},
		{Code: "CEN-MISS", IsUncensored: &censored, FetchedAt: now.Add(4 * time.Second), CreatedAt: now.Add(4 * time.Second)},
		{Code: "UNK-MISS", FetchedAt: now.Add(5 * time.Second), CreatedAt: now.Add(5 * time.Second)},
		{Code: "", IsUncensored: &uncensored, FetchedAt: now.Add(6 * time.Second), CreatedAt: now.Add(6 * time.Second)},
	}
	if err := gdb.Create(&rows).Error; err != nil {
		t.Fatalf("create jav rows: %v", err)
	}
	haveAllIdol := models.JavIdol{Name: "Existing Idol"}
	if err := gdb.Create(&haveAllIdol).Error; err != nil {
		t.Fatalf("create idol: %v", err)
	}
	var haveAll models.Jav
	if err := gdb.Where("code = ?", "HAVE-ALL").First(&haveAll).Error; err != nil {
		t.Fatalf("load have all jav: %v", err)
	}
	if err := gdb.Create(&models.JavIdolMap{JavID: haveAll.ID, JavIdolID: haveAllIdol.ID}).Error; err != nil {
		t.Fatalf("create idol map: %v", err)
	}

	items, err := ListUncensoredJavsMissingAvsoxMetadata(ctx)
	if err != nil {
		t.Fatalf("ListUncensoredJavsMissingAvsoxMetadata: %v", err)
	}
	if len(items) != 4 {
		t.Fatalf("unexpected item count: got %d want 4", len(items))
	}
	got := []string{items[0].Code, items[1].Code, items[2].Code, items[3].Code}
	want := []string{"MISS-BOTH", "MISS-STUDIO", "MISS-SERIES", "MISS-IDOLS"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected codes: got %#v want %#v", got, want)
		}
	}
	if items[1].SeriesID == nil || *items[1].SeriesID != series.ID {
		t.Fatalf("expected existing series id on second item: %#v", items[1])
	}
	if items[2].StudioID == nil || *items[2].StudioID != studio.ID {
		t.Fatalf("expected existing studio id on third item: %#v", items[2])
	}
}

func TestUpdateMissingJavSeriesStudios(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1710000000, 0).UTC()

	studioA := models.JavStudio{Name: "Studio A"}
	studioB := models.JavStudio{Name: "Studio B"}
	if err := gdb.Create(&[]models.JavStudio{studioA, studioB}).Error; err != nil {
		t.Fatalf("create studios: %v", err)
	}
	var studios []models.JavStudio
	if err := gdb.Order("name").Find(&studios).Error; err != nil {
		t.Fatalf("load studios: %v", err)
	}
	studioA = studios[0]
	studioB = studios[1]

	zhSeries := models.JavSeries{Name: "中文系列"}
	enSeries := models.JavSeries{Name: "English Series", IsEnglish: true}
	emptySeries := models.JavSeries{Name: "No Studio Series"}
	keptSeries := models.JavSeries{Name: "Kept Series", StudioID: &studioB.ID}
	if err := gdb.Create(&[]models.JavSeries{zhSeries, enSeries, emptySeries, keptSeries}).Error; err != nil {
		t.Fatalf("create series: %v", err)
	}
	var series []models.JavSeries
	if err := gdb.Order("name").Find(&series).Error; err != nil {
		t.Fatalf("load series: %v", err)
	}
	byName := map[string]models.JavSeries{}
	for _, item := range series {
		byName[item.Name] = item
	}
	zhSeries = byName["中文系列"]
	enSeries = byName["English Series"]
	emptySeries = byName["No Studio Series"]
	keptSeries = byName["Kept Series"]

	javs := []models.Jav{
		{Code: "SER-001", StudioID: &studioA.ID, SeriesID: &zhSeries.ID, FetchedAt: now},
		{Code: "SER-002", StudioID: &studioB.ID, SeriesEnID: &enSeries.ID, FetchedAt: now},
		{Code: "SER-003", SeriesID: &emptySeries.ID, FetchedAt: now},
		{Code: "SER-004", StudioID: &studioA.ID, SeriesID: &keptSeries.ID, FetchedAt: now},
	}
	if err := gdb.Create(&javs).Error; err != nil {
		t.Fatalf("create javs: %v", err)
	}

	updated, err := UpdateMissingJavSeriesStudios(ctx)
	if err != nil {
		t.Fatalf("update missing series studios: %v", err)
	}
	if updated != 2 {
		t.Fatalf("updated = %d, want 2", updated)
	}

	var got []models.JavSeries
	if err := gdb.Order("name").Find(&got).Error; err != nil {
		t.Fatalf("load updated series: %v", err)
	}
	gotByName := map[string]models.JavSeries{}
	for _, item := range got {
		gotByName[item.Name] = item
	}
	if gotByName["中文系列"].StudioID == nil || *gotByName["中文系列"].StudioID != studioA.ID {
		t.Fatalf("unexpected zh series studio: %#v", gotByName["中文系列"])
	}
	if gotByName["English Series"].StudioID == nil || *gotByName["English Series"].StudioID != studioB.ID {
		t.Fatalf("unexpected en series studio: %#v", gotByName["English Series"])
	}
	if gotByName["No Studio Series"].StudioID != nil {
		t.Fatalf("unexpected empty series studio: %#v", gotByName["No Studio Series"])
	}
	if gotByName["Kept Series"].StudioID == nil || *gotByName["Kept Series"].StudioID != studioB.ID {
		t.Fatalf("unexpected kept series studio: %#v", gotByName["Kept Series"])
	}
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
	currentJav := models.Jav{Code: "NOOP-001", Title: "Current", FetchedAt: now}
	otherJav := models.Jav{Code: "NOOP-002", Title: "Other", FetchedAt: now}
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
	if err := setVideoLocationJavIDTx(gdb, loc.ID, 0, currentJav.ID, staleUpdatedAt); err != nil {
		t.Fatalf("same jav id should be accepted as noop: %v", err)
	}
	if err := setVideoLocationJavIDTx(gdb, loc.ID, 0, otherJav.ID, staleUpdatedAt); err == nil {
		t.Fatal("different jav id with stale updated_at should fail")
	}
}

func TestSetVideoLocationJavIDForVideoAllowsStaleTimestampWhenUnlinked(t *testing.T) {
	gdb := openTestDB(t)
	now := time.Unix(1710000000, 0).UTC()

	dir := models.Directory{Path: "/tmp/media"}
	if err := gdb.Create(&dir).Error; err != nil {
		t.Fatalf("create directory: %v", err)
	}
	video := models.Video{
		DirectoryID: dir.ID,
		Path:        "stale.mp4",
		Filename:    "stale.mp4",
		Fingerprint: "fp-stale",
		DurationSec: 7200,
		ModifiedAt:  now,
	}
	if err := gdb.Create(&video).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}
	javRec := models.Jav{Code: "STALE-001", Title: "Stale", FetchedAt: now}
	if err := gdb.Create(&javRec).Error; err != nil {
		t.Fatalf("create jav: %v", err)
	}
	loc := models.VideoLocation{
		VideoID:      video.ID,
		DirectoryID:  dir.ID,
		RelativePath: "stale.mp4",
		ModifiedAt:   now,
		UpdatedAt:    now,
	}
	if err := gdb.Create(&loc).Error; err != nil {
		t.Fatalf("create video location: %v", err)
	}
	if err := gdb.Model(&models.VideoLocation{}).Where("id = ?", loc.ID).Update("updated_at", now.Add(time.Minute)).Error; err != nil {
		t.Fatalf("refresh video location updated_at: %v", err)
	}

	if err := setVideoLocationJavIDTx(gdb, loc.ID, video.ID, javRec.ID, now); err != nil {
		t.Fatalf("same video with stale updated_at should be linked: %v", err)
	}
	var got models.VideoLocation
	if err := gdb.First(&got, loc.ID).Error; err != nil {
		t.Fatalf("load video location: %v", err)
	}
	if got.JavID == nil || *got.JavID != javRec.ID {
		t.Fatalf("jav id not linked: %#v", got.JavID)
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

	javRec := models.Jav{Code: "LANG-001", Title: "Language Work", FetchedAt: now}
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

	javRec := models.Jav{Code: "IDOL-001", Title: "Idol Language Work", FetchedAt: now}
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
	idols, total, err := ListJavIdols(ctx, "", "work", 20, 0, nil, 0)
	if err != nil {
		t.Fatalf("ListJavIdols zh: %v", err)
	}
	assertJavIdolSummaries(t, idols, total, []string{"一ノ瀬アメリ"})
	if _, err := GetJavIdolSummary(ctx, englishIdol.ID, nil); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected english idol to be hidden in zh mode, got err=%v", err)
	}

	jav.SetMetadataLanguage("en")
	idols, total, err = ListJavIdols(ctx, "", "work", 20, 0, nil, 0)
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
	javRec := models.Jav{Code: "MISS-001", Title: "Missing Profile Work", FetchedAt: now}
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

	javA := models.Jav{Code: "AAA-001", Title: "A", FetchedAt: now}
	javB := models.Jav{Code: "BBB-001", Title: "B", FetchedAt: now}
	if err := gdb.Create(&javA).Error; err != nil {
		t.Fatalf("create jav a: %v", err)
	}
	if err := gdb.Create(&javB).Error; err != nil {
		t.Fatalf("create jav b: %v", err)
	}
	tag := models.JavTag{Name: "Location Count"}
	if err := gdb.Create(&tag).Error; err != nil {
		t.Fatalf("create jav tag: %v", err)
	}
	idol := models.JavIdol{Name: "Location Idol"}
	if err := gdb.Create(&idol).Error; err != nil {
		t.Fatalf("create idol: %v", err)
	}
	if err := gdb.Create(&[]models.JavTagMap{{JavID: javA.ID, JavTagID: tag.ID, Provider: int(jav.ProviderJavBus)}}).Error; err != nil {
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

	idols, _, err := ListJavIdols(ctx, "", "work", 20, 0, nil, 0)
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

	soloJav := models.Jav{Code: "DDD-001", Title: "Solo Work", FetchedAt: now}
	groupJav := models.Jav{Code: "EEE-001", Title: "Group Work", FetchedAt: now}
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
		FetchedAt:   now,
	}
	longJav := models.Jav{
		Code:        "GGG-001",
		Title:       "Long",
		DurationMin: 180,
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

	oldJav := models.Jav{Code: "HHH-001", Title: "Old Solo", FetchedAt: now}
	youngJav := models.Jav{Code: "III-001", Title: "Young Solo", FetchedAt: now}
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

	items, total, err := ListJavIdols(ctx, "", "birth", 20, 0, nil, 0)
	if err != nil {
		t.Fatalf("ListJavIdols birth: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("unexpected birth result size: len=%d total=%d", len(items), total)
	}
	if items[0].ID != youngIdol.ID {
		t.Fatalf("unexpected birth first idol: got %d want %d", items[0].ID, youngIdol.ID)
	}

	items, total, err = ListJavIdols(ctx, "", "birth_asc", 20, 0, nil, 0)
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

func TestListJavIdolsSortByRecentDirections(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1710000000, 0).UTC()

	dir := models.Directory{Path: "/tmp/media"}
	if err := db.Create(&dir).Error; err != nil {
		t.Fatalf("create directory: %v", err)
	}

	oldIdol := models.JavIdol{Name: "Old Added Idol", CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now.Add(-2 * time.Hour)}
	newIdol := models.JavIdol{Name: "New Added Idol", CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
	if err := db.Create(&oldIdol).Error; err != nil {
		t.Fatalf("create old idol: %v", err)
	}
	if err := db.Create(&newIdol).Error; err != nil {
		t.Fatalf("create new idol: %v", err)
	}

	oldJav := models.Jav{Code: "RAD-001", Title: "Old Added Work", FetchedAt: now}
	newJav := models.Jav{Code: "RAD-002", Title: "New Added Work", FetchedAt: now}
	if err := db.Create(&oldJav).Error; err != nil {
		t.Fatalf("create old jav: %v", err)
	}
	if err := db.Create(&newJav).Error; err != nil {
		t.Fatalf("create new jav: %v", err)
	}

	if err := db.Create(&[]models.JavIdolMap{
		{JavID: oldJav.ID, JavIdolID: oldIdol.ID},
		{JavID: newJav.ID, JavIdolID: newIdol.ID},
	}).Error; err != nil {
		t.Fatalf("create idol maps: %v", err)
	}

	videos := []models.Video{
		{
			DirectoryID: dir.ID,
			Path:        "old-added.mp4",
			Filename:    "old-added.mp4",
			Fingerprint: "fp-old-added",
			JavID:       int64Ptr(oldJav.ID),
			ModifiedAt:  now,
		},
		{
			DirectoryID: dir.ID,
			Path:        "new-added.mp4",
			Filename:    "new-added.mp4",
			Fingerprint: "fp-new-added",
			JavID:       int64Ptr(newJav.ID),
			ModifiedAt:  now,
		},
	}
	if err := db.Create(&videos).Error; err != nil {
		t.Fatalf("create videos: %v", err)
	}
	createVideoLocationsForVideos(t, db, videos...)

	items, total, err := ListJavIdols(ctx, "", "recent", 20, 0, nil, 0)
	if err != nil {
		t.Fatalf("ListJavIdols recent: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("unexpected recent result size: len=%d total=%d", len(items), total)
	}
	if items[0].ID != newIdol.ID {
		t.Fatalf("unexpected recent first idol: got %d want %d", items[0].ID, newIdol.ID)
	}

	items, total, err = ListJavIdols(ctx, "", "recent_asc", 20, 0, nil, 0)
	if err != nil {
		t.Fatalf("ListJavIdols recent_asc: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("unexpected recent_asc result size: len=%d total=%d", len(items), total)
	}
	if items[0].ID != oldIdol.ID {
		t.Fatalf("unexpected recent_asc first idol: got %d want %d", items[0].ID, oldIdol.ID)
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

func assertJavSeries(t *testing.T, db *gorm.DB, code, want string, isEnglish bool) {
	t.Helper()

	var rec models.Jav
	preload := "Series"
	if isEnglish {
		preload = "SeriesEn"
	}
	if err := db.Preload(preload).Where("code = ?", code).First(&rec).Error; err != nil {
		t.Fatalf("load jav %q: %v", code, err)
	}
	series := rec.Series
	if isEnglish {
		series = rec.SeriesEn
	}
	if series == nil {
		t.Fatalf("expected series for %q", code)
	}
	if series.Name != want {
		t.Fatalf("unexpected series for %q: got %q want %q", code, series.Name, want)
	}
	if series.IsEnglish != isEnglish {
		t.Fatalf("unexpected series language for %q: got %v want %v", code, series.IsEnglish, isEnglish)
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
		Select("jt.name, jtm.provider").
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

func assertJavTagProviderNames(t *testing.T, tags []JavTagCount, want map[int][]string) {
	t.Helper()

	got := map[int][]string{}
	for _, tag := range tags {
		got[tag.Provider] = append(got[tag.Provider], tag.Name)
	}
	if len(got) != len(want) {
		t.Fatalf("unexpected provider count: got=%#v want=%#v", got, want)
	}
	for provider, wantNames := range want {
		gotNames := got[provider]
		if len(gotNames) != len(wantNames) {
			t.Fatalf("unexpected names for provider %d: got=%#v want=%#v", provider, gotNames, wantNames)
		}
		for i, wantName := range wantNames {
			if gotNames[i] != wantName {
				t.Fatalf("unexpected name for provider %d at %d: got=%q want=%q all=%#v", provider, i, gotNames[i], wantName, got)
			}
		}
	}
}

func assertJavTagCounts(t *testing.T, tags []JavTagCount, want map[string]int64) {
	t.Helper()

	got := map[string]int64{}
	for _, tag := range tags {
		got[tag.Name] = tag.Count
	}
	for name, wantCount := range want {
		if got[name] != wantCount {
			t.Fatalf("unexpected count for %q: got=%d want=%d all=%#v", name, got[name], wantCount, got)
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
