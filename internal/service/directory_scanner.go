package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"pornboss/internal/common"
	"pornboss/internal/common/logging"
	"pornboss/internal/db"
	"pornboss/internal/models"
)

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
