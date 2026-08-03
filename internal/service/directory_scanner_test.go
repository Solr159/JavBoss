package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"javboss/internal/common"
	"javboss/internal/db"
	"javboss/internal/models"
)

func TestCancelAndReserveDirectoryScanCancelsActiveSession(t *testing.T) {
	resetDirectoryScanSessions(t)

	scanCtx, finish, err := startDirectoryScanSession(context.Background(), 42)
	if err != nil {
		t.Fatalf("begin scan: %v", err)
	}
	scanDone := make(chan struct{})
	go func() {
		<-scanCtx.Done()
		finish()
		close(scanDone)
	}()

	release, err := CancelAndReserveDirectoryScan(context.Background(), 42)
	if err != nil {
		t.Fatalf("cancel and reserve scan: %v", err)
	}
	defer func() {
		if release != nil {
			release()
		}
	}()

	select {
	case <-scanDone:
	default:
		t.Fatal("active scan should be canceled and finished before reservation is returned")
	}
	if _, _, err := startDirectoryScanSession(context.Background(), 42); !errors.Is(err, ErrDirectoryScanInProgress) {
		t.Fatalf("reservation should block new scans: %v", err)
	}

	release()
	release = nil
	_, nextFinish, err := startDirectoryScanSession(context.Background(), 42)
	if err != nil {
		t.Fatalf("begin scan after reservation release: %v", err)
	}
	nextFinish()
}

func TestCancelAndReserveDirectoryScanHonorsContext(t *testing.T) {
	resetDirectoryScanSessions(t)

	_, finish, err := startDirectoryScanSession(context.Background(), 42)
	if err != nil {
		t.Fatalf("begin scan: %v", err)
	}
	defer finish()

	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	release, err := CancelAndReserveDirectoryScan(ctx, 42)
	if release != nil {
		release()
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancel should honor context deadline: %v", err)
	}
}

func TestIsDirectoryScanning(t *testing.T) {
	resetDirectoryScanSessions(t)

	if IsDirectoryScanning(42) {
		t.Fatal("directory should be idle before a scan starts")
	}

	_, finish, err := startDirectoryScanSession(context.Background(), 42)
	if err != nil {
		t.Fatalf("begin scan: %v", err)
	}
	if !IsDirectoryScanning(42) {
		t.Fatal("directory should report scanning while its session is active")
	}

	finish()
	if IsDirectoryScanning(42) {
		t.Fatal("directory should be idle after its scan finishes")
	}

	release, err := CancelAndReserveDirectoryScan(context.Background(), 42)
	if err != nil {
		t.Fatalf("reserve scan: %v", err)
	}
	defer release()
	if IsDirectoryScanning(42) {
		t.Fatal("directory update reservation should not be reported as scanning")
	}
}

func TestDifferentDirectoryScansCanRunConcurrently(t *testing.T) {
	resetDirectoryScanSessions(t)

	_, finishFirst, err := startDirectoryScanSession(context.Background(), 41)
	if err != nil {
		t.Fatalf("begin first scan: %v", err)
	}
	defer finishFirst()

	_, finishSecond, err := startDirectoryScanSession(context.Background(), 42)
	if err != nil {
		t.Fatalf("begin concurrent scan for another directory: %v", err)
	}
	finishSecond()
}

func TestSyncDirectoryPersistsLatestSuccessfulSummary(t *testing.T) {
	resetDirectoryScanSessions(t)
	gdb, err := db.Open(filepath.Join(t.TempDir(), "scan-summary.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	previousDB := common.DB
	common.DB = gdb
	t.Cleanup(func() {
		common.DB = previousDB
		sqlDB, dbErr := gdb.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	dir := models.Directory{Path: t.TempDir()}
	if err := gdb.Create(&dir).Error; err != nil {
		t.Fatalf("create directory: %v", err)
	}
	startedAt := time.Now().Add(-time.Second).UnixMilli()
	summary, err := SyncDirectory(t.Context(), dir)
	if err != nil {
		t.Fatalf("sync directory: %v", err)
	}
	if summary.Directories != 1 {
		t.Fatalf("scanned directories = %d, want 1", summary.Directories)
	}

	var refreshed models.Directory
	if err := gdb.First(&refreshed, dir.ID).Error; err != nil {
		t.Fatalf("reload directory: %v", err)
	}
	got := refreshed.LastScanSummary
	if got.FinishedAtUnixMS < startedAt || got.FinishedAtUnixMS > time.Now().UnixMilli() {
		t.Fatalf("scan finished time = %d, want current timestamp", got.FinishedAtUnixMS)
	}
	if got.FilesSeen != summary.FilesSeen ||
		got.Inserted != summary.Inserted ||
		got.Updated != summary.Updated ||
		got.Removed != summary.Removed {
		t.Fatalf("stored summary = %+v, runtime summary = %+v", got, summary)
	}
	if got.DurationMS < 0 || got.DurationMS > summary.Duration.Milliseconds() {
		t.Fatalf("stored duration = %dms, total runtime = %s", got.DurationMS, summary.Duration)
	}

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := SyncDirectory(canceledCtx, dir); err == nil {
		t.Fatal("canceled scan should fail")
	}
	var afterCanceled models.Directory
	if err := gdb.First(&afterCanceled, dir.ID).Error; err != nil {
		t.Fatalf("reload directory after canceled scan: %v", err)
	}
	if afterCanceled.LastScanSummary != got {
		t.Fatalf(
			"canceled scan replaced successful summary: got %+v, want %+v",
			afterCanceled.LastScanSummary,
			got,
		)
	}
}

func TestDirectoryAutoScanDue(t *testing.T) {
	now := time.Unix(1750000000, 0)
	tests := []struct {
		name      string
		directory models.Directory
		want      bool
	}{
		{
			name:      "disabled",
			directory: models.Directory{ID: 1, AutoScanEnabled: false, AutoScanIntervalMinutes: 1},
			want:      false,
		},
		{
			name:      "never scanned",
			directory: models.Directory{ID: 1, AutoScanEnabled: true, AutoScanIntervalMinutes: 30},
			want:      true,
		},
		{
			name: "interval not elapsed",
			directory: models.Directory{
				ID:                      1,
				AutoScanEnabled:         true,
				AutoScanIntervalMinutes: 30,
				LastScanSummary: models.DirectoryScanSummary{
					FinishedAtUnixMS: now.Add(-29 * time.Minute).UnixMilli(),
				},
			},
			want: false,
		},
		{
			name: "interval elapsed",
			directory: models.Directory{
				ID:                      1,
				AutoScanEnabled:         true,
				AutoScanIntervalMinutes: 30,
				LastScanSummary: models.DirectoryScanSummary{
					FinishedAtUnixMS: now.Add(-30 * time.Minute).UnixMilli(),
				},
			},
			want: true,
		},
		{
			name:      "deleted",
			directory: models.Directory{ID: 1, IsDelete: true, AutoScanEnabled: true, AutoScanIntervalMinutes: 1},
			want:      false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := directoryAutoScanDue(test.directory, now); got != test.want {
				t.Fatalf("directoryAutoScanDue() = %t, want %t", got, test.want)
			}
		})
	}
}

func resetDirectoryScanSessions(t *testing.T) {
	t.Helper()

	dirScanMu.Lock()
	previous := dirScanActive
	dirScanActive = map[int64]*directoryScanSession{}
	dirScanMu.Unlock()

	t.Cleanup(func() {
		dirScanMu.Lock()
		dirScanActive = previous
		dirScanMu.Unlock()
	})
}
