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
	"pornboss/internal/jav"
	"pornboss/internal/models"
	"pornboss/internal/util"
)

const (
	javLinkWorkerCount = 4
	javLinkQueueSize   = 256
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

type javLinkBatch struct {
	ctx     context.Context
	tasks   chan int64
	seen    map[int64]struct{}
	mu      sync.Mutex
	closed  bool
	workers sync.WaitGroup
}

func newJavLinkBatch(ctx context.Context) *javLinkBatch {
	if ctx == nil {
		ctx = context.Background()
	}
	batch := &javLinkBatch{
		ctx:   ctx,
		tasks: make(chan int64, javLinkQueueSize),
		seen:  make(map[int64]struct{}),
	}
	for i := 0; i < javLinkWorkerCount; i++ {
		batch.workers.Add(1)
		go batch.worker()
	}
	return batch
}

func (b *javLinkBatch) Enqueue(locationID int64) {
	if b == nil || locationID <= 0 {
		return
	}

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	if _, ok := b.seen[locationID]; ok {
		b.mu.Unlock()
		return
	}
	b.seen[locationID] = struct{}{}
	b.mu.Unlock()

	select {
	case b.tasks <- locationID:
	case <-b.ctx.Done():
	}
}

func (b *javLinkBatch) Wait() {
	if b == nil {
		return
	}

	b.mu.Lock()
	if !b.closed {
		b.closed = true
		close(b.tasks)
	}
	b.mu.Unlock()
	b.workers.Wait()
}

func (b *javLinkBatch) worker() {
	defer b.workers.Done()
	for locationID := range b.tasks {
		if err := b.ctx.Err(); err != nil {
			return
		}
		if err := processVideoLocationJavLink(b.ctx, locationID); err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			logging.Error("video location jav link failed location=%d err=%v", locationID, err)
		}
	}
}

func finishJavLinkBatch(batch *javLinkBatch) {
	batch.Wait()
}

func processVideoLocationJavLink(ctx context.Context, locationID int64) error {
	v, err := db.GetVideoForJavScan(ctx, locationID)
	if err != nil || v == nil {
		return err
	}

	if v.JavID != nil {
		return nil
	}
	if v.DurationSec > 0 && v.DurationSec < 3600 {
		return nil
	}

	filename := filepath.Base(filepath.FromSlash(v.Filename))
	possibleCodes := util.ExtractCodeFromName(filename)
	if len(possibleCodes) == 0 {
		return nil
	}

	for _, code := range possibleCodes {
		existJav, err := db.GetJavByCode(ctx, code)
		if err != nil {
			logging.Error("jav lookup existing failed location=%d code=%s err=%v", v.LocationID, code, err)
			continue
		}
		if existJav == nil {
			continue
		}
		if err := db.SetVideoLocationJavIDForVideo(ctx, v.LocationID, v.VideoID, existJav.ID, v.UpdatedAt); err != nil {
			logging.Error("set video location jav failed location=%d code=%s err=%v", v.LocationID, code, err)
		} else {
			enqueueCover(existJav.Code)
		}
		return nil
	}

	if linked, err := lookupAndLinkVideoLocationJav(ctx, v, filename, possibleCodes, jav.ProviderJavBus); err != nil || linked {
		return err
	}
	if linked, err := lookupAndLinkVideoLocationJav(ctx, v, filename, possibleCodes, jav.ProviderJavDatabase); err != nil || linked {
		return err
	}
	return nil
}

func lookupAndLinkVideoLocationJav(ctx context.Context, v *db.JavScanVideo, filename string, possibleCodes []string, provider jav.Provider) (bool, error) {
	for _, code := range possibleCodes {
		info, err := jav.LookupJavByCode(code, provider)
		if err != nil {
			if errors.Is(err, jav.ResourceNotFonud) {
				continue
			}
			logging.Error("jav lookup failed provider=%s location=%s code=%s err=%v", provider.String(), filename, code, err)
			continue
		}
		if info == nil {
			continue
		}

		if _, err := db.SaveJavInfoAndLinkLocationForVideo(ctx, info, v.LocationID, v.VideoID, v.UpdatedAt); err != nil {
			logging.Error("link video location->jav failed provider=%s location=%s code=%s err=%v", provider.String(), filename, info.Code, err)
		} else {
			logging.Info("link video location->jav success provider=%s location=%s code=%s", provider.String(), filename, info.Code)
			enqueueCover(info.Code)
		}
		return true, nil
	}
	return false, nil
}

func enqueueCover(code string) {
	mgr := common.CoverManager
	if mgr == nil {
		return
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return
	}
	mgr.Enqueue(code)
}

func enqueueMissingCovers(ctx context.Context) error {
	mgr := common.CoverManager
	if common.DB == nil || mgr == nil {
		return nil
	}
	codes, err := db.ListJavCodes(ctx)
	if err != nil {
		return err
	}
	for _, c := range codes {
		code := strings.TrimSpace(c)
		if code == "" {
			continue
		}
		if mgr.Exists(code) {
			continue
		}
		mgr.Enqueue(code)
	}
	return nil
}
