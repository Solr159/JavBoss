package service

import (
	"context"
	"errors"
	"testing"
	"time"
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
