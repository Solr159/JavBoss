package directory

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"javboss/internal/models"
)

func writeCleanupFixture(t *testing.T, root, name string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("fixture"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestDirectoryCleanupPreviewAndSelection(t *testing.T) {
	ctx := context.Background()
	dir := models.Directory{ID: 1, Path: t.TempDir()}
	m := New(Options{})
	garbage := []string{".DS_Store", "nested/Thumbs.db", "nested/AD.TXT", "ad.url", "ad.html", "ad.htm"}
	protected := []string{"movie.mp4", "movie.mkv", "movie.srt", "movie.ass", "movie.nfo", "poster.jpg", "cover.png", directoryProcessReportName, "download.part"}
	for _, name := range append(garbage, protected...) {
		writeCleanupFixture(t, dir.Path, name)
	}
	preview, err := m.PreviewDirectoryCleanup(ctx, dir, CleanupOptions{IncludeSystem: true, IncludeMisc: true})
	if err != nil || len(preview.Files) != len(garbage) || len(preview.Issues) != 0 {
		t.Fatalf("preview=%+v err=%v", preview, err)
	}
	if preview.RootPath != dir.Path {
		t.Fatal("missing root path")
	}
	system, err := m.PreviewDirectoryCleanup(ctx, dir, CleanupOptions{IncludeSystem: true})
	if err != nil || len(system.Files) != 2 {
		t.Fatalf("system preview=%+v err=%v", system, err)
	}
	withoutSystem, err := m.PreviewDirectoryCleanup(ctx, dir, CleanupOptions{IncludeMisc: true})
	if err != nil || len(withoutSystem.Files) != len(garbage)-2 {
		t.Fatalf("system category was not excluded: %+v err=%v", withoutSystem, err)
	}
	// Preview never mutates the filesystem; only selected entries are removed.
	for _, file := range preview.Files {
		if _, err := os.Stat(filepath.Join(dir.Path, file.Path)); err != nil {
			t.Fatal(err)
		}
	}
	result, err := m.CleanDirectoryFiles(ctx, dir, dir.Path, preview.Files[:1])
	if err != nil || len(result.Deleted) != 1 || result.DeletedBytes != 7 || len(result.Issues) != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for _, name := range append(protected, preview.Files[1].Path) {
		if _, err := os.Stat(filepath.Join(dir.Path, name)); err != nil {
			t.Fatalf("unselected or protected file removed: %s: %v", name, err)
		}
	}
	if m.DirectoryWorkStatus(dir.ID) != DirectoryWorkIdle {
		t.Fatal("reservation was not released")
	}
}

func TestDirectoryCleanupRejectsInvalidSelectionBeforeDeleting(t *testing.T) {
	ctx := context.Background()
	dir := models.Directory{ID: 1, Path: t.TempDir()}
	m := New(Options{})
	writeCleanupFixture(t, dir.Path, "ad.txt")
	preview, err := m.PreviewDirectoryCleanup(ctx, dir, CleanupOptions{IncludeSystem: true, IncludeMisc: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../outside.txt", "/tmp/outside.txt", "nested/../../outside.txt", "nested\\ad.txt", "movie.mp4", "movie.nfo", directoryProcessReportName, ".", "ad.txt"} {
		t.Run(name, func(t *testing.T) {
			bad := preview.Files[0]
			bad.Path = name
			_, err := m.CleanDirectoryFiles(ctx, dir, dir.Path, []CleanupFile{preview.Files[0], bad})
			if !errors.Is(err, ErrInvalidCleanupRequest) {
				t.Fatalf("expected invalid request, got %v", err)
			}
			if _, err := os.Stat(filepath.Join(dir.Path, "ad.txt")); err != nil {
				t.Fatal("validation caused partial deletion")
			}
		})
	}
	if _, err := m.CleanDirectoryFiles(ctx, dir, "old-path", preview.Files); !errors.Is(err, ErrInvalidCleanupRequest) {
		t.Fatalf("stale root accepted: %v", err)
	}
	if _, err := m.CleanDirectoryFiles(ctx, dir, dir.Path, nil); !errors.Is(err, ErrInvalidCleanupRequest) {
		t.Fatalf("empty selection accepted: %v", err)
	}
}

func TestDirectoryCleanupSkipsChangedFilesAndSymlinks(t *testing.T) {
	ctx := context.Background()
	dir := models.Directory{ID: 1, Path: t.TempDir()}
	m := New(Options{})
	for _, name := range []string{"changed.txt", "gone.url", "replaced.txt", "folder/ad.txt", "good.txt"} {
		writeCleanupFixture(t, dir.Path, name)
	}
	outside := t.TempDir()
	writeCleanupFixture(t, outside, "ad.txt")
	if err := os.Symlink(outside, filepath.Join(dir.Path, "external")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	preview, err := m.PreviewDirectoryCleanup(ctx, dir, CleanupOptions{IncludeSystem: true, IncludeMisc: true})
	if err != nil || len(preview.Files) != 5 {
		t.Fatalf("preview followed symlink: %+v err=%v", preview, err)
	}
	if err := os.WriteFile(filepath.Join(dir.Path, "changed.txt"), []byte("changed after preview"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"gone.url", "replaced.txt"} {
		if err := os.Remove(filepath.Join(dir.Path, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(outside, "ad.txt"), filepath.Join(dir.Path, "replaced.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(dir.Path, "folder"), filepath.Join(dir.Path, "original")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir.Path, "folder")); err != nil {
		t.Fatal(err)
	}
	result, err := m.CleanDirectoryFiles(ctx, dir, dir.Path, preview.Files)
	if err != nil || len(result.Deleted) != 1 || result.Deleted[0] != "good.txt" || len(result.Issues) != 4 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if _, err := os.Stat(filepath.Join(outside, "ad.txt")); err != nil {
		t.Fatalf("outside file touched: %v", err)
	}
}

func TestDirectoryCleanupTaskReservation(t *testing.T) {
	ctx := context.Background()
	dir := models.Directory{ID: 1, Path: t.TempDir()}
	m := New(Options{})
	_, finishScan, err := m.acquireDirectoryScanSession(ctx, dir.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.PreviewDirectoryCleanup(ctx, dir, CleanupOptions{IncludeSystem: true, IncludeMisc: true}); !errors.Is(err, ErrDirectoryWorkInProgress) {
		t.Fatalf("cleanup must not interrupt scan: %v", err)
	}
	finishScan()
	release, err := m.reserveCleanup(ctx, dir.ID, "cleaning_junk")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, _, err := m.acquireDirectoryScanSession(ctx, dir.ID); !errors.Is(err, ErrDirectoryScanInProgress) {
		t.Fatalf("scan entered during cleanup: %v", err)
	}
	if _, err := m.CancelAndReserveDirectoryScan(ctx, dir.ID); !errors.Is(err, ErrDirectoryScanInProgress) {
		t.Fatalf("path update entered during cleanup: %v", err)
	}
	if err := m.StartDirectoryProcessing(ctx, dir, DirectoryProcessSidecar, "", false); !errors.Is(err, ErrDirectoryWorkInProgress) {
		t.Fatalf("organizing entered during cleanup: %v", err)
	}
	if m.DirectoryWorkStatus(dir.ID) != "cleaning_junk" {
		t.Fatal("cleanup status missing")
	}
	release()
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := m.PreviewDirectoryCleanup(cancelled, dir, CleanupOptions{IncludeSystem: true, IncludeMisc: true}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled preview: %v", err)
	}
	if _, err := m.PreviewDirectoryCleanup(ctx, dir, CleanupOptions{IncludeSystem: true, IncludeMisc: true}); err != nil {
		t.Fatalf("reservation leaked: %v", err)
	}
}
