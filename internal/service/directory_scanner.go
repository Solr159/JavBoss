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

	"javboss/internal/common"
	"javboss/internal/common/logging"
	"javboss/internal/db"
	"javboss/internal/models"
	"javboss/internal/util"
)

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
	javLinks               *javLinkBatch
}

func newSyncState(ctx context.Context, directoryID int64, javLinks *javLinkBatch) (*syncState, error) {
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
		javLinks:               javLinks,
	}, nil
}

var (
	ErrDirectoryScanInProgress = errors.New("directory scan in progress")
	dirScanMu                  sync.Mutex
	dirScanActive              = map[int64]*directoryScanSession{}
)

type directoryScanSession struct {
	cancel  context.CancelFunc
	done    chan struct{}
	reserve bool
}

func startDirectoryScanSession(ctx context.Context, id int64) (context.Context, func(), error) {
	if id <= 0 {
		return nil, nil, errors.New("directory id cannot be zero")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	dirScanMu.Lock()
	defer dirScanMu.Unlock()
	if _, ok := dirScanActive[id]; ok {
		return nil, nil, ErrDirectoryScanInProgress
	}

	scanCtx, cancel := context.WithCancel(ctx)
	session := &directoryScanSession{
		cancel: cancel,
		done:   make(chan struct{}),
	}
	dirScanActive[id] = session

	var finishOnce sync.Once
	finish := func() {
		finishOnce.Do(func() {
			cancel()
			dirScanMu.Lock()
			if dirScanActive[id] == session {
				delete(dirScanActive, id)
			}
			close(session.done)
			dirScanMu.Unlock()
		})
	}
	return scanCtx, finish, nil
}

// SyncDirectory scans one configured media directory from disk and reconciles it with the database.
// It also queues every seen video location for asynchronous JAV metadata linking and waits for
// that queue before returning.
func SyncDirectory(ctx context.Context, directory models.Directory) (*Summary, error) {
	start := time.Now()
	javLinks := newJavLinkBatch(ctx)
	summary, err := syncDirectory(ctx, directory, javLinks)
	finishJavLinkBatch(javLinks)
	if summary != nil {
		summary.Duration = time.Since(start)
	}
	return summary, err
}

func syncDirectory(ctx context.Context, directory models.Directory, javLinks *javLinkBatch) (*Summary, error) {
	if common.DB == nil {
		return nil, errors.New("nil database")
	}
	if directory.ID == 0 || directory.IsDelete {
		return &Summary{}, nil
	}
	scanCtx, finish, err := startDirectoryScanSession(ctx, directory.ID)
	if err != nil {
		return nil, err
	}
	defer finish()

	start := time.Now()
	summary := &Summary{}
	logging.Info("sync directory start: id=%d path=%s", directory.ID, directory.Path)

	state, err := newSyncState(scanCtx, directory.ID, javLinks)
	if err != nil {
		return nil, err
	}
	scanned, err := syncDirectoryWithState(scanCtx, directory, state, summary)
	if err != nil {
		if errors.Is(err, context.Canceled) && ctx.Err() == nil {
			logging.Info("sync directory canceled: id=%d path=%s", directory.ID, directory.Path)
		} else {
			logging.Error("sync directory failed: id=%d path=%s err=%v", directory.ID, directory.Path, err)
		}
		return nil, err
	}
	if scanned {
		if err := hideUnprocessedVideoLocations(scanCtx, state.processedLocationIDs, summary, directory.ID); err != nil {
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

	if err := walkDirectoryAndSync(ctx, dir, state, summary); err != nil {
		return false, err
	}
	return true, nil
}

// walkDirectoryAndSync 扫描单个目录下的文件，实时 upsert 并记录统计/缩略图/JAV 关联任务。
func walkDirectoryAndSync(ctx context.Context, directory models.Directory, state *syncState, summary *Summary) error {
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
		if existingLoc, ok := state.existingLocationByPath[pathKey]; ok {
			existingVideo := state.existingByID[existingLoc.VideoID]
			if existingVideo != nil && existingVideo.Size == info.Size() && existingLoc.ModifiedAt.Equal(modTime) {
				state.processedLocationIDs[existingLoc.ID] = struct{}{}
				if existingLoc.IsDelete {
					saved, err := db.UpsertVideoLocation(ctx, existingLoc.VideoID, directory.ID, relativePath, modTime)
					if err != nil {
						logging.Error("unhide video location failed, skip: path=%s err=%v", normalizedPath, err)
						return nil
					}
					existingLoc.IsDelete = false
					existingLoc.ModifiedAt = modTime
					if saved != nil {
						state.processedLocationIDs[saved.ID] = struct{}{}
						state.existingLocationByPath[makePathKey(saved.DirectoryID, saved.RelativePath)] = saved
						state.javLinks.Enqueue(saved.ID)
					}
					summary.Updated++
				} else {
					state.javLinks.Enqueue(existingLoc.ID)
				}
				return nil
			}
		}

		logging.Info("scan file: root=%s path=%s size=%d", normalizedRoot, relativePath, info.Size())

		meta, err := util.ProbeVideoContext(ctx, normalizedPath)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
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

		return upsertVideo(ctx, fileEntry, state, summary)
	})
}

func upsertVideo(ctx context.Context, entry *FileEntry, state *syncState, summary *Summary) error {
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
			state.existingByID[video.ID] = video
		}
	}

	if video != nil {
		if err := upsertLocationForEntry(ctx, video, entry, state); err != nil {
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
	state.existingByID[video.ID] = video
	if err := upsertLocationForEntry(ctx, video, entry, state); err != nil {
		logging.Error("save video location failed, skip: path=%s err=%v", entry.AbsolutePath, err)
		return nil
	}
	video.ModifiedAt = entry.ModifiedAt
	common.ScreenshotManager.EnqueueForVideo(video)
	return nil
}

func upsertLocationForEntry(ctx context.Context, video *models.Video, entry *FileEntry, state *syncState) error {
	loc, err := db.UpsertVideoLocation(ctx, video.ID, entry.DirectoryID, entry.RelativePath, entry.ModifiedAt)
	if err != nil {
		return err
	}
	if loc != nil {
		state.processedLocationIDs[loc.ID] = struct{}{}
		state.existingLocationByPath[makePathKey(loc.DirectoryID, loc.RelativePath)] = loc
		state.javLinks.Enqueue(loc.ID)
	}
	state.existingByID[video.ID] = video
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

// CancelAndReserveDirectoryScan cancels any active scan for id, waits until it exits,
// then reserves the scan slot until the returned release function is called.
func CancelAndReserveDirectoryScan(ctx context.Context, id int64) (func(), error) {
	if id <= 0 {
		return nil, errors.New("directory id cannot be zero")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	for {
		dirScanMu.Lock()
		session := dirScanActive[id]
		if session == nil {
			reservation := &directoryScanSession{done: make(chan struct{}), reserve: true}
			dirScanActive[id] = reservation
			dirScanMu.Unlock()
			var releaseOnce sync.Once
			return func() {
				releaseOnce.Do(func() {
					dirScanMu.Lock()
					if dirScanActive[id] == reservation {
						delete(dirScanActive, id)
					}
					close(reservation.done)
					dirScanMu.Unlock()
				})
			}, nil
		}
		if session.reserve {
			dirScanMu.Unlock()
			return nil, ErrDirectoryScanInProgress
		}
		session.cancel()
		done := session.done
		dirScanMu.Unlock()

		select {
		case <-done:
		case <-ctx.Done():
			return nil, fmt.Errorf("cancel directory scan: %w", ctx.Err())
		}
	}
}

// SyncAllDirectories scans all configured active media directories one by one.
// Each directory is reconciled independently so stale location cleanup is scoped to that
// directory, while fingerprint matching remains global so moved or duplicated files can keep
// their existing video metadata, tags, and JAV associations. JAV links are processed as one
// batch, and the global missing-cover sweep runs once after the whole pass.
func SyncAllDirectories(ctx context.Context, directories []models.Directory) (*Summary, error) {
	if common.DB == nil {
		return nil, errors.New("nil database")
	}
	if len(directories) == 0 {
		return &Summary{}, nil
	}

	start := time.Now()
	summary := &Summary{}
	javLinks := newJavLinkBatch(ctx)
	finishJavLinks := func() {
		if javLinks == nil {
			return
		}
		finishJavLinkBatch(javLinks)
		javLinks = nil
	}
	defer finishJavLinks()

	for _, dir := range directories {
		if dir.ID == 0 || dir.IsDelete {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		dirSummary, err := syncDirectory(ctx, dir, javLinks)
		if err != nil {
			if errors.Is(err, ErrDirectoryScanInProgress) {
				continue
			}
			if errors.Is(err, context.Canceled) && ctx.Err() == nil {
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

	finishJavLinks()
	if err := enqueueMissingCovers(ctx); err != nil {
		logging.Error("jav cover enqueue failed: %v", err)
		return nil, err
	}
	summary.Duration = time.Since(start)
	return summary, nil
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
