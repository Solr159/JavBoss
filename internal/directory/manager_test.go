package directory

import (
	"context"
	"errors"
	"testing"

	"javboss/internal/models"
)

func TestBackgroundJobsRejectCanceledApplication(t *testing.T) {
	previous := applicationTasks.Load()
	t.Cleanup(func() { applicationTasks.Store(previous) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	Init(ctx, "")
	if Context() != ctx {
		t.Fatal("directory jobs are not bound to the application context")
	}
	cancel()
	dir := models.Directory{ID: 1, Path: t.TempDir()}
	for name, start := range map[string]func() error{
		"manual scan":                 func() error { return StartManualDirectoryScan(dir) },
		"scan after directory update": func() error { _, err := ScanDirectory(Context(), dir); return err },
		"directory processing":        func() error { return StartDirectoryProcessing(context.Background(), dir, "", "") },
	} {
		t.Run(name, func(t *testing.T) {
			if err := start(); !errors.Is(err, context.Canceled) {
				t.Fatalf("start after application cancellation: %v", err)
			}
			if DirectoryWorkStatus(dir.ID) != DirectoryWorkIdle {
				t.Fatal("canceled task reserved the directory")
			}
		})
	}
}

func TestManagersIsolateDirectoryWork(t *testing.T) {
	first, second := New(Options{}), New(Options{})
	firstCtx, finishFirst, err := first.acquireDirectoryScanSession(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer finishFirst()
	if second.IsDirectoryScanning(1) {
		t.Fatal("second manager inherited first scan")
	}
	secondCtx, finishSecond, err := second.acquireDirectoryScanSession(context.Background(), 1)
	if err != nil {
		t.Fatalf("same ID in another library must not conflict: %v", err)
	}
	defer finishSecond()
	first.setDirectoryProcessingStatus(1, DirectoryWorkOrganizing)
	if got := second.activeDirectoryProcessingStatus(1); got != "" {
		t.Fatalf("second manager inherited processing status: %s", got)
	}
	finishFirst()
	if firstCtx.Err() == nil {
		t.Fatal("finished scan context remains active")
	}
	if secondCtx.Err() != nil || !second.IsDirectoryScanning(1) {
		t.Fatal("finishing first scan affected second manager")
	}
}
