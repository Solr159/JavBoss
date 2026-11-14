package directory

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"javboss/internal/db"
	"javboss/internal/models"
)

func TestScanUpsertPersistsMediaInfoAndUnchangedLegacyFilesStayUnprobed(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "new file", true: "unchanged legacy file"}[legacy], func(t *testing.T) {
			deps := newTestDependencies()
			conn, err := db.Open(filepath.Join(t.TempDir(), "scan.db"))
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, _ := conn.DB()
			defer sqlDB.Close()
			deps.database = conn
			deps.bindDatabase(t)
			dir := models.Directory{Path: t.TempDir()}
			if err := conn.Create(&dir).Error; err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir.Path, "video.mp4")
			// Deliberately not playable: an accidental ffprobe on the unchanged path fails.
			if err := os.WriteFile(path, []byte("unchanged media"), 0600); err != nil {
				t.Fatal(err)
			}
			stat, _ := os.Stat(path)
			info := models.MediaInfo{Version: models.MediaInfoVersion, VideoCodec: "h264", Width: 1920, Height: 1080, DurationSeconds: 12.25, Container: "mp4"}
			entry := &FileEntry{DirectoryID: dir.ID, DirectoryPath: dir.Path, RelativePath: "video.mp4", AbsolutePath: path, Size: stat.Size(), ModifiedAt: stat.ModTime(), DurationSec: 12, Fingerprint: info.FingerprintV2(stat.Size())}
			if !legacy {
				entry.MediaInfo = info
			}
			state, err := loadDirectorySyncState(t.Context(), dir.ID, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := upsertVideo(t.Context(), entry, state, &Summary{}); err != nil {
				t.Fatal(err)
			}
			video, err := db.Default().GetVideoByFingerprint(t.Context(), entry.Fingerprint)
			if err != nil || video == nil {
				t.Fatal(video, err)
			}
			if !reflect.DeepEqual(video.MediaInfo, entry.MediaInfo) {
				t.Fatalf("scan metadata: %+v", video.MediaInfo)
			}
			summary, err := deps.ScanDirectory(t.Context(), dir)
			if err != nil {
				t.Fatal(err)
			}
			if summary.FilesSeen != 1 || summary.Removed != 0 || summary.Inserted != 0 {
				t.Fatalf("unchanged scan: %+v", summary)
			}
			if err := conn.First(video, video.ID).Error; err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(video.MediaInfo, entry.MediaInfo) {
				t.Fatalf("unchanged file metadata changed: %+v", video.MediaInfo)
			}
		})
	}
}
