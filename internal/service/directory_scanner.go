package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"pornboss/internal/common"
	"pornboss/internal/common/logging"
	"pornboss/internal/db"
	"pornboss/internal/models"
	"pornboss/internal/util"
)

var (
	ErrDirectoryScanInProgress = errors.New("directory scan in progress")
	dirScanMu                  sync.Mutex
	dirScanActive              = map[int64]struct{}{}
)

func tryLockDirectory(id int64) bool {
	if id == 0 {
		return false
	}
	dirScanMu.Lock()
	defer dirScanMu.Unlock()
	if _, ok := dirScanActive[id]; ok {
		return false
	}
	dirScanActive[id] = struct{}{}
	return true
}

func unlockDirectory(id int64) {
	if id == 0 {
		return
	}
	dirScanMu.Lock()
	delete(dirScanActive, id)
	dirScanMu.Unlock()
}

type FileEntry struct {
	DirectoryID   int64
	DirectoryPath string
	RelativePath  string
	AbsolutePath  string
	Filename      string
	Size          int64
	ModifiedAt    time.Time
	Fingerprint   string
	PathKey       string
	DurationSec   int64
}

// Summary captures the results of a directory synchronisation run.
type Summary struct {
	FilesSeen   int
	Inserted    int
	Updated     int
	Removed     int
	Duration    time.Duration
	Directories int
}

func makePathKey(directoryID int64, relativePath string) string {
	return strconv.FormatInt(directoryID, 10) + ":" + relativePath
}

type syncState struct {
	existingByID           map[int64]*models.Video
	existingLocationByPath map[string]*models.VideoLocation
	processedLocationIDs   map[int64]struct{}
}

func newSyncState(ctx context.Context, directoryID int64) (*syncState, error) {
	existingLocations, err := db.VideoLocationsByDirectory(ctx, directoryID)
	if err != nil {
		return nil, err
	}
	existingByID := make(map[int64]*models.Video, len(existingLocations))
	existingLocationByPath := make(map[string]*models.VideoLocation, len(existingLocations))
	processedLocationIDs := make(map[int64]struct{}, len(existingLocations))
	for i := range existingLocations {
		loc := &existingLocations[i]
		existingLocationByPath[makePathKey(loc.DirectoryID, loc.RelativePath)] = loc
		if loc.Video.ID == 0 {
			continue
		}
		video := &loc.Video
		existingByID[video.ID] = video
	}

	return &syncState{
		existingByID:           existingByID,
		existingLocationByPath: existingLocationByPath,
		processedLocationIDs:   processedLocationIDs,
	}, nil
}

// SyncDirectory scans one configured media directory from disk and reconciles it with the database.
// It walks video files under the directory, reuses unchanged locations by size and mtime,
// probes changed or new files for duration/fingerprint, creates videos when needed, upserts
// video_locations, queues screenshots for newly-created videos, and hides stale locations that
// were not seen in this directory scan.
func SyncDirectory(ctx context.Context, directory models.Directory) (*Summary, error) {
	if common.DB == nil {
		return nil, errors.New("nil database")
	}
	if directory.ID == 0 || directory.IsDelete {
		return &Summary{}, nil
	}
	if !tryLockDirectory(directory.ID) {
		return nil, ErrDirectoryScanInProgress
	}
	defer unlockDirectory(directory.ID)

	start := time.Now()
	summary := &Summary{}
	logging.Info("sync directory start: id=%d path=%s", directory.ID, directory.Path)

	state, err := newSyncState(ctx, directory.ID)
	if err != nil {
		return nil, err
	}
	scanned, err := syncDirectoryWithState(ctx, directory, state, summary)
	if err != nil {
		logging.Error("sync directory failed: id=%d path=%s err=%v", directory.ID, directory.Path, err)
		return nil, err
	}
	if scanned {
		if err := hideUnprocessedVideoLocations(ctx, state.processedLocationIDs, summary, directory.ID); err != nil {
			return nil, err
		}
		summary.Directories = 1
	}
	summary.Duration = time.Since(start)
	logging.Info(
		"sync directory summary: id=%d path=%s scanned=%t files_seen=%d inserted=%d updated=%d removed=%d duration=%s",
		directory.ID,
		directory.Path,
		scanned,
		summary.FilesSeen,
		summary.Inserted,
		summary.Updated,
		summary.Removed,
		summary.Duration,
	)
	return summary, nil
}

// SyncAllDirectories scans all configured active media directories one by one.
// Each directory is reconciled independently so stale location cleanup is scoped to that
// directory, while fingerprint matching remains global so moved or duplicated files can keep
// their existing video metadata, tags, and JAV associations.
func SyncAllDirectories(ctx context.Context, directories []models.Directory) (*Summary, error) {
	if common.DB == nil {
		return nil, errors.New("nil database")
	}
	if len(directories) == 0 {
		return &Summary{}, nil
	}

	start := time.Now()
	summary := &Summary{}
	for _, dir := range directories {
		if dir.ID == 0 || dir.IsDelete {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		dirSummary, err := SyncDirectory(ctx, dir)
		if err != nil {
			if errors.Is(err, ErrDirectoryScanInProgress) {
				continue
			}
			return nil, err
		}
		summary.FilesSeen += dirSummary.FilesSeen
		summary.Inserted += dirSummary.Inserted
		summary.Updated += dirSummary.Updated
		summary.Removed += dirSummary.Removed
		summary.Directories += dirSummary.Directories
	}

	summary.Duration = time.Since(start)
	return summary, nil
}

func syncDirectoryWithState(ctx context.Context, dir models.Directory, state *syncState, summary *Summary) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	info, statErr := os.Stat(dir.Path)
	if statErr != nil {
		if util.IsPathUnavailable(statErr) {
			if err := db.SetDirectoryMissingAndHideVideos(ctx, dir.ID, true); err != nil {
				logging.Error("mark directory missing failed id=%d path=%s err=%v", dir.ID, dir.Path, err)
			}
			return false, nil
		}
		return false, fmt.Errorf("stat directory %s: %w", dir.Path, statErr)
	}
	if !info.IsDir() {
		if err := db.SetDirectoryMissingAndHideVideos(ctx, dir.ID, true); err != nil {
			logging.Error("mark directory missing failed id=%d path=%s err=%v", dir.ID, dir.Path, err)
		}
		return false, nil
	}
	if dir.Missing {
		if err := db.SetDirectoryMissingAndHideVideos(ctx, dir.ID, false); err != nil {
			logging.Error("clear directory missing failed id=%d path=%s err=%v", dir.ID, dir.Path, err)
		}
	}

	if err := walkDirectoryAndSync(ctx, dir, state.existingByID, state.existingLocationByPath, state.processedLocationIDs, summary); err != nil {
		return false, err
	}
	return true, nil
}

// walkDirectoryAndSync 扫描单个目录下的文件，实时 upsert 并记录统计/缩略图任务。
func walkDirectoryAndSync(ctx context.Context, directory models.Directory, existingByID map[int64]*models.Video, existingLocationByPath map[string]*models.VideoLocation, processedLocationIDs map[int64]struct{}, summary *Summary) error {
	// 边遍历文件边做指纹计算和 DB 更新，避免一次性构建全量快照
	normalizedRoot := filepath.Clean(directory.Path)
	return filepath.WalkDir(normalizedRoot, func(candidatePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			logging.Error("walk directory entry failed, skip: root=%s path=%s err=%v", normalizedRoot, candidatePath, walkErr)
			return nil
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		if entry.IsDir() {
			return nil
		}
		if !util.IsVideo(candidatePath) {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			if errors.Is(err, fs.ErrPermission) {
				return nil
			}
			return err
		}

		// 计算相对路径，确保只处理目录内的文件（防止符号链接等越界）
		normalizedPath := filepath.Clean(candidatePath)
		relativePath, err := filepath.Rel(normalizedRoot, normalizedPath)
		if err != nil {
			return err
		}
		if strings.HasPrefix(relativePath, "..") {
			return nil
		}

		relativePath = filepath.ToSlash(cleanRelativePath(relativePath))
		modTime := info.ModTime().UTC()
		summary.FilesSeen++

		// If file unchanged (size + mtime), skip probe and DB touches but mark as seen.
		pathKey := makePathKey(directory.ID, relativePath)
		if existingLoc, ok := existingLocationByPath[pathKey]; ok {
			existingVideo := existingByID[existingLoc.VideoID]
			if existingVideo != nil && existingVideo.Size == info.Size() && existingLoc.ModifiedAt.Equal(modTime) {
				processedLocationIDs[existingLoc.ID] = struct{}{}
				if existingLoc.IsDelete {
					saved, err := db.UpsertVideoLocation(ctx, existingLoc.VideoID, directory.ID, relativePath, modTime)
					if err != nil {
						logging.Error("unhide video location failed, skip: path=%s err=%v", normalizedPath, err)
						return nil
					}
					existingLoc.IsDelete = false
					existingLoc.ModifiedAt = modTime
					if saved != nil {
						processedLocationIDs[saved.ID] = struct{}{}
					}
					summary.Updated++
				}
				return nil
			}
		}

		logging.Info("scan file: root=%s path=%s size=%d", normalizedRoot, relativePath, info.Size())

		meta, err := util.ProbeVideo(normalizedPath)
		if err != nil {
			logging.Error("probe video metadata error: %v", err)
			return nil
		}
		fingerprint := meta.FingerprintV2(info.Size())
		durationSec := int64(math.Round(meta.DurationSeconds))

		fileEntry := &FileEntry{
			DirectoryID:   directory.ID,
			DirectoryPath: normalizedRoot,
			RelativePath:  relativePath,
			AbsolutePath:  normalizedPath,
			Filename:      info.Name(),
			Size:          info.Size(),
			ModifiedAt:    modTime,
			Fingerprint:   fingerprint,
			DurationSec:   durationSec,
		}

		return upsertVideo(ctx, fileEntry, existingByID, existingLocationByPath, processedLocationIDs, summary)
	})
}

func upsertVideo(ctx context.Context, entry *FileEntry, existingByID map[int64]*models.Video, existingLocationByPath map[string]*models.VideoLocation, processedLocationIDs map[int64]struct{}, summary *Summary) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// 已存在：检测元信息/文件是否变化，必要时更新
	var video *models.Video
	if entry.Fingerprint != "" {
		existingVideo, err := db.GetVideoByFingerprint(ctx, entry.Fingerprint)
		if err != nil {
			logging.Error("lookup video by fingerprint failed, skip: path=%s err=%v", entry.AbsolutePath, err)
			return nil
		}
		if existingVideo != nil {
			video = existingVideo
			existingByID[video.ID] = video
		}
	}

	if video != nil {
		if err := upsertLocationForEntry(ctx, video, entry, existingByID, existingLocationByPath, processedLocationIDs); err != nil {
			logging.Error("save video location failed, skip: path=%s err=%v", entry.AbsolutePath, err)
			return nil
		}
		return nil
	}

	video = &models.Video{
		DirectoryID: entry.DirectoryID,
		Size:        entry.Size,
		Fingerprint: entry.Fingerprint,
		DurationSec: entry.DurationSec,
	}

	if err := db.CreateVideo(ctx, video); err != nil {
		logging.Error("create video failed, skip: path=%s err=%v", entry.AbsolutePath, err)
		return nil
	}
	summary.Inserted++
	existingByID[video.ID] = video
	if err := upsertLocationForEntry(ctx, video, entry, existingByID, existingLocationByPath, processedLocationIDs); err != nil {
		logging.Error("save video location failed, skip: path=%s err=%v", entry.AbsolutePath, err)
		return nil
	}
	video.ModifiedAt = entry.ModifiedAt
	common.ScreenshotManager.EnqueueForVideo(video)
	return nil
}

func upsertLocationForEntry(ctx context.Context, video *models.Video, entry *FileEntry, existingByID map[int64]*models.Video, existingLocationByPath map[string]*models.VideoLocation, processedLocationIDs map[int64]struct{}) error {
	loc, err := db.UpsertVideoLocation(ctx, video.ID, entry.DirectoryID, entry.RelativePath, entry.ModifiedAt)
	if err != nil {
		return err
	}
	if loc != nil {
		processedLocationIDs[loc.ID] = struct{}{}
		existingLocationByPath[makePathKey(loc.DirectoryID, loc.RelativePath)] = loc
	}
	existingByID[video.ID] = video
	return nil
}

// hideUnprocessedVideoLocations marks file locations missing from this directory scan as deleted.
func hideUnprocessedVideoLocations(ctx context.Context, processedLocationIDs map[int64]struct{}, summary *Summary, directoryID int64) error {
	locations, err := db.VideoLocationsByDirectory(ctx, directoryID)
	if err != nil {
		return err
	}
	if len(locations) == 0 {
		return nil
	}

	staleIDs := make([]int64, 0, len(locations))
	for _, loc := range locations {
		if loc.IsDelete {
			continue
		}
		if _, ok := processedLocationIDs[loc.ID]; ok {
			continue
		}
		staleIDs = append(staleIDs, loc.ID)
	}

	if len(staleIDs) == 0 {
		return nil
	}

	logging.Info("hiding stale video locations: count=%d", len(staleIDs))
	if err := db.HideVideoLocationsByIDs(ctx, staleIDs); err != nil {
		return err
	}
	summary.Removed += len(staleIDs)
	return nil
}

func cleanRelativePath(p string) string {
	if p == "" {
		return ""
	}
	cleaned := filepath.Clean(p)
	if cleaned == "." {
		return ""
	}
	return filepath.ToSlash(cleaned)
}

// ScanDirectories loads every active directory configured in the database and runs the directory
// reconciliation scanner. The scanner compares filesystem video files against stored video
// locations, creates or relinks videos, marks missing directories, and hides locations whose files
// disappeared from a successfully scanned directory.
func ScanDirectories(ctx context.Context) error {
	if common.DB == nil {
		return errors.New("nil db")
	}
	dirs, err := db.ListActiveDirectories(ctx)
	if err != nil {
		return err
	}
	if _, err := SyncAllDirectories(ctx, dirs); err != nil {
		if errors.Is(err, ErrDirectoryScanInProgress) {
			return nil
		}
		return err
	}
	return nil
}

// StartDirectoryScanner periodically scans configured media directories.
// It runs ScanDirectories immediately and then on every interval until ctx is done, keeping video
// rows and video_locations aligned with the files currently present on disk.
func StartDirectoryScanner(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if err := ScanDirectories(ctx); err != nil {
				logging.Error("periodic directory scan failed: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
