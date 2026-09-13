package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/gorm"
	"javboss/internal/common"
	"javboss/internal/db"
	"javboss/internal/models"
	"javboss/internal/util"
)

func transcodeTestDatabase(t *testing.T) (*gorm.DB, models.Directory) {
	t.Helper()
	gdb, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	previousDB, previousConfig := common.DB, common.AppConfig
	common.DB = gdb
	common.AppConfig = &common.Config{DatabasePath: filepath.Join(t.TempDir(), "data.db")}
	t.Cleanup(func() {
		common.DB = previousDB
		common.AppConfig = previousConfig
		sqlDB, _ := gdb.DB()
		_ = sqlDB.Close()
	})
	directory := models.Directory{Path: t.TempDir()}
	if err := gdb.Create(&directory).Error; err != nil {
		t.Fatal(err)
	}
	return gdb, directory
}

func transcodeTestSample(t *testing.T, path, codec string) string {
	t.Helper()
	binary, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg unavailable")
	}
	if _, err := util.ResolveFFprobePath(); err != nil {
		t.Skip("ffprobe unavailable")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "-hide_banner", "-loglevel", "error", "-nostdin", "-n", "-f", "lavfi", "-i", "testsrc2=size=96x64:rate=12", "-t", "0.6", "-c:v", codec, "-pix_fmt", "yuv420p", path)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate test video: %v %s", err, output)
	}
	return binary
}

func transcodeTestLocation(t *testing.T, gdb *gorm.DB, directory models.Directory, relative string, video *models.Video) models.VideoLocation {
	t.Helper()
	info, err := os.Stat(filepath.Join(directory.Path, relative))
	if err != nil {
		t.Fatal(err)
	}
	if video.ID == 0 {
		meta, err := util.ProbeVideoContext(t.Context(), filepath.Join(directory.Path, relative))
		if err != nil {
			t.Fatal(err)
		}
		video.Fingerprint = meta.FingerprintV2(info.Size())
		video.Size = info.Size()
		if err := gdb.Create(video).Error; err != nil {
			t.Fatal(err)
		}
	}
	loc := models.VideoLocation{VideoID: video.ID, DirectoryID: directory.ID, RelativePath: relative, Filename: filepath.Base(relative), ModifiedAt: info.ModTime().UTC()}
	if err := gdb.Create(&loc).Error; err != nil {
		t.Fatal(err)
	}
	return loc
}

func TestDirectoryTranscodeRealCopiesPreserveMetadataAfterRescan(t *testing.T) {
	gdb, directory := transcodeTestDatabase(t)
	source := filepath.Join(directory.Path, "one", "ABC-123.avi")
	binary := transcodeTestSample(t, source, "mpeg4")
	second := filepath.Join(directory.Path, "two", "ABC-123.avi")
	if err := os.MkdirAll(filepath.Dir(second), 0755); err != nil {
		t.Fatal(err)
	}
	if err := copyExclusiveFile(source, second); err != nil {
		t.Fatal(err)
	}
	video := models.Video{PlayCount: 17, JavScrapeOverride: ":manual:ABC-123", CoverScreenshotName: "custom.jpg", CreatedAt: time.Unix(1700000000, 0)}
	a := transcodeTestLocation(t, gdb, directory, "one/ABC-123.avi", &video)
	b := transcodeTestLocation(t, gdb, directory, "two/ABC-123.avi", &video)
	jav := models.Jav{Code: "ABC-123"}
	tag := models.Tag{Name: "kept"}
	for _, row := range []any{&jav, &tag} {
		if err := gdb.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := gdb.Model(&models.VideoLocation{}).Where("video_id = ?", video.ID).Update("jav_id", jav.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&models.VideoTag{VideoID: video.ID, TagID: tag.ID}).Error; err != nil {
		t.Fatal(err)
	}
	asset := filepath.Join(filepath.Dir(common.AppConfig.DatabasePath), "video", "1", "screenshot", "custom.jpg")
	if err := os.MkdirAll(filepath.Dir(asset), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(asset, []byte("custom screenshot"), 0600); err != nil {
		t.Fatal(err)
	}
	job := &directoryTranscodeJob{started: time.Now()}
	if err := runDirectoryTranscode(t.Context(), directory, binary, job); err != nil {
		t.Fatal(err)
	}
	if job.progress.Converted != 2 || job.progress.Failed != 0 {
		t.Fatalf("progress: %+v", job.progress)
	}
	var outputID int64
	for _, id := range []int64{a.ID, b.ID} {
		var loc models.VideoLocation
		if err := gdb.Preload("Video").Preload("Video.Tags").First(&loc, id).Error; err != nil {
			t.Fatal(err)
		}
		if outputID == 0 {
			outputID = loc.VideoID
		} else if outputID != loc.VideoID {
			t.Fatal("duplicate outputs have different video IDs")
		}
		if loc.Video.PlayCount != 17 || len(loc.Video.Tags) != 1 || loc.JavID == nil || *loc.JavID != jav.ID || loc.Video.CoverScreenshotName != "custom.jpg" {
			t.Fatalf("metadata lost: %+v %+v", loc, loc.Video)
		}
		path := filepath.Join(directory.Path, filepath.FromSlash(loc.RelativePath))
		meta, err := util.ProbeVideoContext(t.Context(), path)
		if err != nil || !util.BrowserCompatibleVideo(meta) {
			t.Fatalf("incompatible output: %+v %v", meta, err)
		}
	}
	for _, path := range []string{source, second, filepath.Join(filepath.Dir(source), util.TranscodeWorkDirectory), filepath.Join(filepath.Dir(second), util.TranscodeWorkDirectory)} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("source or temporary files retained: %s %v", path, err)
		}
	}
	var encoded models.Video
	if err := gdb.First(&encoded, outputID).Error; err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(filepath.Dir(common.AppConfig.DatabasePath), "video", "2", "screenshot", "custom.jpg")
	if data, err := os.ReadFile(copyPath); err != nil || string(data) != "custom screenshot" {
		t.Fatalf("screenshot not copied: %v", err)
	}
	// Force probing instead of the unchanged-file fast path.
	if err := gdb.Model(&models.VideoLocation{}).Where("directory_id = ?", directory.ID).Update("modified_at", time.Time{}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := ScanDirectory(t.Context(), directory); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{a.ID, b.ID} {
		var loc models.VideoLocation
		if err := gdb.Preload("Video").Preload("Video.Tags").First(&loc, id).Error; err != nil {
			t.Fatal(err)
		}
		if loc.VideoID != outputID || loc.Video.PlayCount != 17 || len(loc.Video.Tags) != 1 || loc.IsDelete {
			t.Fatalf("rescan lost converted identity: %+v", loc)
		}
	}
	again := &directoryTranscodeJob{started: time.Now()}
	if err := runDirectoryTranscode(t.Context(), directory, binary, again); err != nil {
		t.Fatal(err)
	}
	if again.progress.Converted != 0 || again.progress.Skipped != 2 {
		t.Fatalf("rerun reconverted compatible files: %+v", again.progress)
	}
}

func TestDirectoryTranscodeFailuresRetainSources(t *testing.T) {
	for _, scenario := range []string{"target conflict", "encoder failure", "database failure", "incompatible mp4"} {
		t.Run(scenario, func(t *testing.T) {
			gdb, directory := transcodeTestDatabase(t)
			name := "movie.avi"
			if scenario == "incompatible mp4" {
				name = "movie.mp4"
			}
			source := filepath.Join(directory.Path, name)
			binary := transcodeTestSample(t, source, "mpeg4")
			before, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			video := models.Video{PlayCount: 4}
			loc := transcodeTestLocation(t, gdb, directory, name, &video)
			if scenario == "target conflict" {
				if err := os.WriteFile(filepath.Join(directory.Path, "movie.mp4"), []byte("existing"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "encoder failure" {
				binary = filepath.Join(t.TempDir(), "nonexistent-ffmpeg")
			}
			if scenario == "database failure" {
				if err := gdb.Exec(`CREATE TRIGGER fail_transcode BEFORE UPDATE ON video_location BEGIN SELECT RAISE(FAIL, 'injected database failure'); END`).Error; err != nil {
					t.Fatal(err)
				}
			}
			var loaded models.VideoLocation
			if err := gdb.Preload("Video").First(&loaded, loc.ID).Error; err != nil {
				t.Fatal(err)
			}
			job := &directoryTranscodeJob{started: time.Now()}
			converted, err := transcodeDirectoryFile(t.Context(), directory, binary, name, loaded, job)
			if scenario == "incompatible mp4" {
				if err != nil || !converted {
					t.Fatalf("replace incompatible mp4: %v", err)
				}
				var saved models.VideoLocation
				if err := gdb.Preload("Video").First(&saved, loc.ID).Error; err != nil {
					t.Fatal(err)
				}
				if saved.VideoID != video.ID || saved.Video.PlayCount != 4 {
					t.Fatal("single-file replacement lost identity")
				}
				return
			}
			if err == nil || converted {
				t.Fatalf("expected failure: converted=%v err=%v", converted, err)
			}
			after, readErr := os.ReadFile(source)
			if readErr != nil || string(after) != string(before) {
				t.Fatalf("source not restored: %v", readErr)
			}
			var stored models.VideoLocation
			if err := gdb.Preload("Video").First(&stored, loc.ID).Error; err != nil {
				t.Fatal(err)
			}
			if stored.RelativePath != name || stored.Video.Fingerprint != video.Fingerprint || stored.Video.PlayCount != 4 {
				t.Fatal("failed conversion modified original metadata")
			}
			if scenario == "target conflict" {
				data, err := os.ReadFile(filepath.Join(directory.Path, "movie.mp4"))
				if err != nil || string(data) != "existing" {
					t.Fatal("existing target was overwritten")
				}
			}
		})
	}
}

func TestRecoverInterruptedTranscode(t *testing.T) {
	for _, committed := range []bool{false, true} {
		t.Run(map[bool]string{false: "rollback", true: "committed"}[committed], func(t *testing.T) {
			gdb, directory := transcodeTestDatabase(t)
			source := filepath.Join(directory.Path, "movie.avi")
			transcodeTestSample(t, source, "mpeg4")
			video := models.Video{PlayCount: 9}
			loc := transcodeTestLocation(t, gdb, directory, "movie.avi", &video)
			stage := filepath.Join(directory.Path, util.TranscodeWorkDirectory, "job-recovery")
			output := filepath.Join(stage, "output.mp4")
			transcodeTestSample(t, output, "libx264")
			meta, err := util.ProbeVideoContext(t.Context(), output)
			if err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(output)
			if err != nil {
				t.Fatal(err)
			}
			fingerprint := meta.FingerprintV2(info.Size())
			journal := transcodeJournal{DirectoryID: directory.ID, LocationID: loc.ID, Source: "movie.avi", Target: "movie.mp4", Fingerprint: fingerprint, Published: true}
			data, _ := json.Marshal(journal)
			if err := os.WriteFile(filepath.Join(stage, "journal.json"), data, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(source, filepath.Join(stage, "original")); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(directory.Path, "movie.mp4")
			if err := publishTranscodeFile(output, target); err != nil {
				t.Fatal(err)
			}
			if committed {
				if err := db.CompleteVideoTranscode(t.Context(), loc, "movie.mp4", fingerprint, info.Size(), 1, info.ModTime().UTC(), nil); err != nil {
					t.Fatal(err)
				}
			}
			if err := recoverDirectoryTranscodes(t.Context(), directory); err != nil {
				t.Fatal(err)
			}
			kept, removed := source, target
			if committed {
				kept, removed = target, source
			}
			if _, err := os.Stat(kept); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(removed); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("unexpected retained file: %s", removed)
			}
		})
	}
}

func TestTranscodeCancellationStopsFFmpeg(t *testing.T) {
	source := filepath.Join(t.TempDir(), "movie.avi")
	binary := transcodeTestSample(t, source, "mpeg4")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	reports := 0
	err := util.TranscodeBrowserMP4(ctx, binary, source, filepath.Join(t.TempDir(), "output.mp4"), func(float64, string) { reports++; cancel() })
	if !errors.Is(err, context.Canceled) || reports == 0 {
		t.Fatalf("cancellation/progress: reports=%d err=%v", reports, err)
	}
}

func TestTranscodeReservesOverlappingRootsAndReleasesOnConflict(t *testing.T) {
	resetDirectoryScanSessions(t)
	gdb, directory := transcodeTestDatabase(t)
	nested := models.Directory{Path: filepath.Join(directory.Path, "nested")}
	unrelated := models.Directory{Path: t.TempDir()}
	for _, dir := range []*models.Directory{&nested, &unrelated} {
		if err := os.MkdirAll(dir.Path, 0755); err != nil {
			t.Fatal(err)
		}
		if err := gdb.Create(dir).Error; err != nil {
			t.Fatal(err)
		}
	}
	release, err := reserveTranscodeDirectories(t.Context(), directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{directory.ID, nested.ID} {
		if _, _, err := acquireDirectoryScanSession(t.Context(), id); !errors.Is(err, ErrDirectoryScanInProgress) {
			t.Fatalf("overlapping scan accepted: id=%d err=%v", id, err)
		}
	}
	_, finish, err := acquireDirectoryScanSession(t.Context(), unrelated.ID)
	if err != nil {
		t.Fatal(err)
	}
	finish()
	release()
	blocked, err := CancelAndReserveDirectoryScan(t.Context(), nested.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer blocked()
	if _, err := reserveTranscodeDirectories(t.Context(), directory); !errors.Is(err, ErrDirectoryScanInProgress) {
		t.Fatalf("nested conflict ignored: %v", err)
	}
	_, finish, err = acquireDirectoryScanSession(t.Context(), directory.ID)
	if err != nil {
		t.Fatalf("partial reservation leaked: %v", err)
	}
	finish()
}

func TestTranscodePreservesAudioTracksAndTextSubtitles(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "movie.mkv")
	binary, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg unavailable")
	}
	subtitle := filepath.Join(root, "subtitle.srt")
	if err := os.WriteFile(subtitle, []byte("1\n00:00:00,000 --> 00:00:00,500\nHello\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "-hide_banner", "-loglevel", "error", "-nostdin", "-n", "-f", "lavfi", "-i", "testsrc2=size=96x64:rate=12", "-f", "lavfi", "-i", "sine=frequency=440", "-f", "lavfi", "-i", "sine=frequency=880", "-i", subtitle, "-t", "0.6", "-map", "0:v", "-map", "1:a", "-map", "2:a", "-map", "3:s", "-c:v", "mpeg4", "-c:a", "pcm_s16le", "-c:s", "srt", source)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate tracks: %v %s", err, output)
	}
	output := filepath.Join(root, "output.mp4")
	if err := util.TranscodeBrowserMP4(t.Context(), binary, source, output, nil); err != nil {
		t.Fatal(err)
	}
	probe, err := util.ResolveFFprobePath()
	if err != nil {
		t.Fatal(err)
	}
	data, err := exec.Command(probe, "-v", "error", "-show_entries", "stream=codec_type,codec_name", "-of", "json", output).Output()
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Streams []struct {
			Type  string `json:"codec_type"`
			Codec string `json:"codec_name"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	audio, subtitles := 0, 0
	for _, stream := range result.Streams {
		if stream.Type == "audio" && stream.Codec == "aac" {
			audio++
		}
		if stream.Type == "subtitle" && stream.Codec == "mov_text" {
			subtitles++
		}
	}
	if audio != 2 || subtitles != 1 {
		t.Fatalf("streams not preserved: %s", data)
	}
}
