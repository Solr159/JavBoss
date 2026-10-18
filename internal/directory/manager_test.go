package directory

import (
	"context"
	"testing"
)

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
