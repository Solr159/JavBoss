package db

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"pornboss/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestListVideosSortByDurationDirections(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1710000000, 0).UTC()

	dir := models.Directory{Path: "/tmp/media"}
	if err := db.Create(&dir).Error; err != nil {
		t.Fatalf("create directory: %v", err)
	}

	shortVideo := models.Video{
		DirectoryID: dir.ID,
		Path:        "short.mp4",
		Filename:    "short.mp4",
		Fingerprint: "video-fp-short",
		DurationSec: 90,
		ModifiedAt:  now,
		CreatedAt:   now,
	}
	longVideo := models.Video{
		DirectoryID: dir.ID,
		Path:        "long.mp4",
		Filename:    "long.mp4",
		Fingerprint: "video-fp-long",
		DurationSec: 180,
		ModifiedAt:  now,
		CreatedAt:   now.Add(time.Second),
	}
	if err := db.Create(&shortVideo).Error; err != nil {
		t.Fatalf("create short video: %v", err)
	}
	if err := db.Create(&longVideo).Error; err != nil {
		t.Fatalf("create long video: %v", err)
	}
	createVideoLocationsForVideos(t, db, shortVideo, longVideo)

	items, err := ListVideos(ctx, 20, 0, nil, "", "duration", nil, nil)
	if err != nil {
		t.Fatalf("ListVideos duration: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("unexpected item count: got %d want 2", len(items))
	}
	if items[0].ID != longVideo.ID {
		t.Fatalf("unexpected first video: got %d want %d", items[0].ID, longVideo.ID)
	}

	items, err = ListVideos(ctx, 20, 0, nil, "", "duration_asc", nil, nil)
	if err != nil {
		t.Fatalf("ListVideos duration_asc: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("unexpected asc item count: got %d want 2", len(items))
	}
	if items[0].ID != shortVideo.ID {
		t.Fatalf("unexpected asc first video: got %d want %d", items[0].ID, shortVideo.ID)
	}
}

func TestListVideosCanIncludeRecognizedJav(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1710000000, 0).UTC()

	dir := models.Directory{Path: "/tmp/media"}
	if err := gdb.Create(&dir).Error; err != nil {
		t.Fatalf("create directory: %v", err)
	}
	plainVideo := models.Video{
		DirectoryID: dir.ID,
		Path:        "plain.mp4",
		Filename:    "plain.mp4",
		Fingerprint: "plain-fp",
		ModifiedAt:  now,
	}
	javVideo := models.Video{
		DirectoryID: dir.ID,
		Path:        "abc-001.mp4",
		Filename:    "abc-001.mp4",
		Fingerprint: "jav-fp",
		ModifiedAt:  now.Add(time.Second),
	}
	if err := gdb.Create(&plainVideo).Error; err != nil {
		t.Fatalf("create plain video: %v", err)
	}
	if err := gdb.Create(&javVideo).Error; err != nil {
		t.Fatalf("create jav video: %v", err)
	}
	createVideoLocationsForVideos(t, gdb, plainVideo, javVideo)

	javRec := models.Jav{Code: "ABC-001", Title: "recognized"}
	if err := gdb.Create(&javRec).Error; err != nil {
		t.Fatalf("create jav: %v", err)
	}
	if err := gdb.Model(&models.VideoLocation{}).
		Where("video_id = ?", javVideo.ID).
		Update("jav_id", javRec.ID).Error; err != nil {
		t.Fatalf("mark jav location: %v", err)
	}
	tag := models.Tag{Name: "shared"}
	if err := gdb.Create(&tag).Error; err != nil {
		t.Fatalf("create tag: %v", err)
	}
	if err := gdb.Create(&[]models.VideoTag{
		{VideoID: plainVideo.ID, TagID: tag.ID, CreatedAt: now},
		{VideoID: javVideo.ID, TagID: tag.ID, CreatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create video tags: %v", err)
	}

	defaultItems, err := ListVideos(ctx, 20, 0, nil, "", "recent", nil, nil)
	if err != nil {
		t.Fatalf("ListVideos default: %v", err)
	}
	if len(defaultItems) != 1 || defaultItems[0].ID != plainVideo.ID {
		t.Fatalf("default list should hide recognized jav: %#v", defaultItems)
	}
	defaultCount, err := CountVideos(ctx, nil, "", nil)
	if err != nil {
		t.Fatalf("CountVideos default: %v", err)
	}
	if defaultCount != 1 {
		t.Fatalf("default count should hide recognized jav: got %d want 1", defaultCount)
	}
	defaultTags, err := ListTags(ctx, nil)
	if err != nil {
		t.Fatalf("ListTags default: %v", err)
	}
	if len(defaultTags) != 1 || defaultTags[0].Count != 1 {
		t.Fatalf("default tag count should hide recognized jav: %#v", defaultTags)
	}

	allItems, err := ListVideos(ctx, 20, 0, nil, "", "recent", nil, nil, false)
	if err != nil {
		t.Fatalf("ListVideos include jav: %v", err)
	}
	if len(allItems) != 2 {
		t.Fatalf("include jav list should return both videos: got %d", len(allItems))
	}
	allCount, err := CountVideos(ctx, nil, "", nil, false)
	if err != nil {
		t.Fatalf("CountVideos include jav: %v", err)
	}
	if allCount != 2 {
		t.Fatalf("include jav count should return both videos: got %d want 2", allCount)
	}
	allTags, err := ListTags(ctx, nil, false)
	if err != nil {
		t.Fatalf("ListTags include jav: %v", err)
	}
	if len(allTags) != 1 || allTags[0].Count != 2 {
		t.Fatalf("include jav tag count should return both videos: %#v", allTags)
	}
}

func TestVideoLocationsAllowSameVideoInMultipleDirectories(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1710000000, 0).UTC()

	dirA := models.Directory{Path: "/tmp/media-a"}
	dirB := models.Directory{Path: "/tmp/media-b"}
	if err := gdb.Create(&dirA).Error; err != nil {
		t.Fatalf("create dir a: %v", err)
	}
	if err := gdb.Create(&dirB).Error; err != nil {
		t.Fatalf("create dir b: %v", err)
	}

	video := models.Video{
		DirectoryID: dirA.ID,
		Path:        "movie.mp4",
		Filename:    "movie.mp4",
		Fingerprint: "same-content",
		Size:        1024,
		DurationSec: 120,
		ModifiedAt:  now,
		Hidden:      true,
	}
	if err := gdb.Create(&video).Error; err != nil {
		t.Fatalf("create video: %v", err)
	}

	locA, err := UpsertVideoLocation(ctx, video.ID, dirA.ID, "movie.mp4", now)
	if err != nil {
		t.Fatalf("upsert loc a: %v", err)
	}
	locB, err := UpsertVideoLocation(ctx, video.ID, dirB.ID, "copies/movie.mp4", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("upsert loc b: %v", err)
	}
	if err := ReconcileAllVideoPaths(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	items, err := ListVideos(ctx, 20, 0, nil, "", "recent", nil, nil)
	if err != nil {
		t.Fatalf("ListVideos: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("unexpected item count: got %d want 2", len(items))
	}
	if len(items[0].Locations) != 1 || len(items[1].Locations) != 1 {
		t.Fatalf("list rows should be location-level: %#v", items)
	}
	count, err := CountVideos(ctx, nil, "", nil)
	if err != nil {
		t.Fatalf("CountVideos: %v", err)
	}
	if count != 2 {
		t.Fatalf("unexpected location count: got %d want 2", count)
	}
	dirBItems, err := ListVideos(ctx, 20, 0, nil, "", "recent", nil, []int64{dirB.ID})
	if err != nil {
		t.Fatalf("ListVideos dir filter: %v", err)
	}
	if len(dirBItems) != 1 || dirBItems[0].LocationID != locB.ID || dirBItems[0].Path != "copies/movie.mp4" {
		t.Fatalf("unexpected filtered items: %#v", dirBItems)
	}
	dirBCount, err := CountVideos(ctx, nil, "", []int64{dirB.ID})
	if err != nil {
		t.Fatalf("CountVideos dir filter: %v", err)
	}
	if dirBCount != 1 {
		t.Fatalf("unexpected filtered count: got %d want 1", dirBCount)
	}
	pathSearchItems, err := ListVideos(ctx, 20, 0, nil, "copies", "recent", nil, nil)
	if err != nil {
		t.Fatalf("ListVideos path search: %v", err)
	}
	if len(pathSearchItems) != 0 {
		t.Fatalf("search should use filename, not relative path directories: %#v", pathSearchItems)
	}
	filenameSearchItems, err := ListVideos(ctx, 20, 0, nil, "movie.mp4", "recent", nil, nil)
	if err != nil {
		t.Fatalf("ListVideos filename search: %v", err)
	}
	if len(filenameSearchItems) != 2 {
		t.Fatalf("filename search should match both copies: got %d want 2", len(filenameSearchItems))
	}
	disabledItems, err := ListVideos(ctx, 20, 0, nil, "", "recent", nil, []int64{0})
	if err != nil {
		t.Fatalf("ListVideos disabled dir filter: %v", err)
	}
	if len(disabledItems) != 0 {
		t.Fatalf("disabled directory filter should return no rows: %#v", disabledItems)
	}
	disabledCount, err := CountVideos(ctx, nil, "", []int64{0})
	if err != nil {
		t.Fatalf("CountVideos disabled dir filter: %v", err)
	}
	if disabledCount != 0 {
		t.Fatalf("disabled directory filter count: got %d want 0", disabledCount)
	}

	videoID, err := GetVideoIDByPath(ctx, dirB.Path, "copies/movie.mp4")
	if err != nil {
		t.Fatalf("GetVideoIDByPath: %v", err)
	}
	if videoID != video.ID {
		t.Fatalf("unexpected video id by second location: got %d want %d", videoID, video.ID)
	}

	if err := HideVideoLocationsByIDs(ctx, []int64{locA.ID}); err != nil {
		t.Fatalf("hide loc a: %v", err)
	}
	if err := ReconcileAllVideoPaths(ctx); err != nil {
		t.Fatalf("reconcile after hiding loc a: %v", err)
	}
	visible, err := GetVideo(ctx, video.ID)
	if err != nil {
		t.Fatalf("GetVideo: %v", err)
	}
	if visible == nil {
		t.Fatal("video should remain visible while one location is active")
	}
	if len(visible.Locations) != 1 || visible.Locations[0].ID != locB.ID {
		t.Fatalf("unexpected remaining locations: %#v", visible.Locations)
	}

	if err := HideVideoLocationsByIDs(ctx, []int64{locB.ID}); err != nil {
		t.Fatalf("hide loc b: %v", err)
	}
	if err := ReconcileAllVideoPaths(ctx); err != nil {
		t.Fatalf("reconcile after hiding loc b: %v", err)
	}
	unavailable, err := GetVideo(ctx, video.ID)
	if err != nil {
		t.Fatalf("GetVideo unavailable: %v", err)
	}
	if unavailable != nil {
		t.Fatal("video should be unavailable when all locations are deleted")
	}
}

type legacyVideoForMigration struct {
	ID          int64  `gorm:"primaryKey"`
	DirectoryID int64  `gorm:"index;not null"`
	Path        string `gorm:"index"`
	Filename    string
	Size        int64
	ModifiedAt  time.Time
	Fingerprint string `gorm:"uniqueIndex"`
	DurationSec int64
	PlayCount   int64 `gorm:"not null;default:0"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	JavID       *int64           `gorm:"index"`
	Jav         *models.Jav      `gorm:"foreignKey:JavID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	Directory   models.Directory `gorm:"foreignKey:DirectoryID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	Hidden      bool             `gorm:"index"`
	Tags        []models.Tag     `gorm:"many2many:video_tag"`
}

func (legacyVideoForMigration) TableName() string {
	return "video"
}

func TestOpenMigratesLegacyGormSchemaPreservesVideoTags(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy-gorm.db")
	legacy, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	if err := legacy.AutoMigrate(
		&models.Directory{},
		&legacyVideoForMigration{},
		&models.Tag{},
		&models.VideoTag{},
		&models.Config{},
		&models.Jav{},
		&models.JavTag{},
		&models.JavIdol{},
		&models.JavTagMap{},
		&models.JavIdolMap{},
	); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}

	now := time.Unix(1710000000, 0).UTC()
	dir := models.Directory{Path: "/tmp/media"}
	if err := legacy.Create(&dir).Error; err != nil {
		t.Fatalf("create legacy directory: %v", err)
	}
	video := legacyVideoForMigration{
		DirectoryID: dir.ID,
		Path:        "tagged.mp4",
		Filename:    "tagged.mp4",
		Fingerprint: "legacy-tagged-fp",
		ModifiedAt:  now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := legacy.Create(&video).Error; err != nil {
		t.Fatalf("create legacy video: %v", err)
	}
	tag := models.Tag{Name: "favorite", CreatedAt: now, UpdatedAt: now}
	if err := legacy.Create(&tag).Error; err != nil {
		t.Fatalf("create legacy tag: %v", err)
	}
	if err := legacy.Create(&models.VideoTag{VideoID: video.ID, TagID: tag.ID, CreatedAt: now}).Error; err != nil {
		t.Fatalf("create legacy video tag: %v", err)
	}
	var beforeCount int64
	if err := legacy.Model(&models.VideoTag{}).Count(&beforeCount).Error; err != nil {
		t.Fatalf("count legacy video tags: %v", err)
	}
	if beforeCount != 1 {
		t.Fatalf("legacy video_tag count = %d, want 1", beforeCount)
	}
	legacySQL, err := legacy.DB()
	if err != nil {
		t.Fatalf("legacy sql db: %v", err)
	}
	_ = legacySQL.Close()

	migrated, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open migrated db: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, err := migrated.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	var afterCount int64
	if err := migrated.Model(&models.VideoTag{}).Where("video_id = ? AND tag_id = ?", video.ID, tag.ID).Count(&afterCount).Error; err != nil {
		t.Fatalf("count migrated video tags: %v", err)
	}
	if afterCount != 1 {
		t.Fatalf("migrated video_tag count = %d, want 1", afterCount)
	}
	assertVideoContentSchema(t, migrated)
	assertModelIndexes(t, migrated)
}

func assertVideoContentSchema(t *testing.T, db *gorm.DB) {
	t.Helper()

	rows, err := db.Raw("PRAGMA table_info(video)").Rows()
	if err != nil {
		t.Fatalf("load video columns: %v", err)
	}
	defer rows.Close()

	columns := map[string]bool{}
	for rows.Next() {
		var (
			cid        int
			name       string
			colType    string
			notNull    int
			defaultVal any
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultVal, &primaryKey); err != nil {
			t.Fatalf("scan video column: %v", err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate video columns: %v", err)
	}

	wantColumns := []string{"id", "size", "fingerprint", "duration_sec", "play_count", "created_at", "updated_at"}
	if len(columns) != len(wantColumns) {
		t.Fatalf("unexpected video columns: got %#v want %v", columns, wantColumns)
	}
	for _, name := range wantColumns {
		if !columns[name] {
			t.Fatalf("missing video column %q in %#v", name, columns)
		}
	}
	for _, name := range []string{"directory_id", "path", "filename", "modified_at", "jav_id", "hidden"} {
		if columns[name] {
			t.Fatalf("obsolete video column %q still exists", name)
		}
	}

	indexRows, err := db.Raw("PRAGMA index_list(video)").Rows()
	if err != nil {
		t.Fatalf("load video indexes: %v", err)
	}
	defer indexRows.Close()

	indexes := map[string]bool{}
	for indexRows.Next() {
		var (
			seq     int
			name    string
			unique  int
			origin  string
			partial int
		)
		if err := indexRows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			t.Fatalf("scan video index: %v", err)
		}
		indexes[name] = true
	}
	if err := indexRows.Err(); err != nil {
		t.Fatalf("iterate video indexes: %v", err)
	}
	if !indexes["idx_video_fingerprint"] {
		t.Fatalf("missing idx_video_fingerprint in %#v", indexes)
	}
	for _, name := range []string{"idx_video_directory_id", "idx_video_path", "idx_video_jav_id", "idx_video_hidden", "idx_video_jav_id_visible"} {
		if indexes[name] {
			t.Fatalf("obsolete video index %q still exists", name)
		}
	}
}

func assertModelIndexes(t *testing.T, db *gorm.DB) {
	t.Helper()

	assertTableIndexes(t, db, "directory", []string{
		"idx_directory_is_delete",
		"idx_directory_missing",
		"idx_directory_path",
	})
	assertTableIndexes(t, db, "jav", []string{
		"idx_jav_code",
		"idx_jav_studio_id",
	})
	assertTableIndexes(t, db, "jav_studio", []string{
		"idx_jav_studio_name",
	})
	assertTableIndexes(t, db, "video", []string{
		"idx_video_fingerprint",
	})
	assertTableIndexes(t, db, "video_location", []string{
		"idx_video_location_directory_id",
		"idx_video_location_directory_path",
		"idx_video_location_filename",
		"idx_video_location_is_delete",
		"idx_video_location_jav_id",
		"idx_video_location_jav_id_is_delete",
		"idx_video_location_video_id",
		"idx_video_location_video_id_jav_id",
		"idx_video_location_visible_filename",
		"idx_video_location_visible_path",
	})
	assertTableIndexes(t, db, "tag", []string{
		"idx_tag_name",
	})
	assertTableIndexes(t, db, "jav_tag", []string{
		"idx_jav_tag_name_source",
		"idx_jav_tag_provider",
	})
	assertTableIndexes(t, db, "jav_idol", []string{
		"idx_jav_idol_name_language",
	})
	assertTableIndexes(t, db, "config", nil)
	assertTableIndexes(t, db, "video_tag", nil)
	assertTableIndexes(t, db, "jav_tag_map", nil)
	assertTableIndexes(t, db, "jav_idol_map", []string{
		"idx_jav_idol_map_jav_idol_id_jav_id",
	})
}

func assertTableIndexes(t *testing.T, db *gorm.DB, table string, want []string) {
	t.Helper()

	rows, err := db.Raw("PRAGMA index_list(" + table + ")").Rows()
	if err != nil {
		t.Fatalf("load %s indexes: %v", table, err)
	}
	defer rows.Close()

	got := map[string]bool{}
	for rows.Next() {
		var (
			seq     int
			name    string
			unique  int
			origin  string
			partial int
		)
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			t.Fatalf("scan %s index: %v", table, err)
		}
		if origin == "pk" {
			continue
		}
		got[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate %s indexes: %v", table, err)
	}

	if len(got) != len(want) {
		t.Fatalf("unexpected %s indexes: got %#v want %v", table, got, want)
	}
	for _, name := range want {
		if !got[name] {
			t.Fatalf("missing %s index %q in %#v", table, name, got)
		}
	}
}

func createVideoLocationsForVideos(t *testing.T, db *gorm.DB, videos ...models.Video) {
	t.Helper()

	for _, video := range videos {
		filename := video.Filename
		if filename == "" {
			filename = filepath.Base(video.Path)
		}
		loc := models.VideoLocation{
			VideoID:      video.ID,
			DirectoryID:  video.DirectoryID,
			RelativePath: video.Path,
			Filename:     filename,
			ModifiedAt:   video.ModifiedAt,
			JavID:        video.JavID,
			IsDelete:     video.Hidden,
			CreatedAt:    video.CreatedAt,
			UpdatedAt:    video.UpdatedAt,
		}
		if err := db.Create(&loc).Error; err != nil {
			t.Fatalf("create video location for video %d: %v", video.ID, err)
		}
	}
}
