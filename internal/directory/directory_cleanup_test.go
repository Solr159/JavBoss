package directory

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"javboss/internal/db"
	"javboss/internal/models"
)

func newCleanupTestDependencies(t *testing.T) *testDependencies {
	t.Helper()
	deps := newTestDependencies()
	var err error
	deps.database, err = db.Open(filepath.Join(t.TempDir(), "cleanup.db"))
	if err != nil {
		t.Fatal(err)
	}
	conn, err := deps.database.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	deps.bindDatabase(t)
	return deps
}

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
	m := newCleanupTestDependencies(t).Manager
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
	m := newCleanupTestDependencies(t).Manager
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
	m := newCleanupTestDependencies(t).Manager
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
	m := newCleanupTestDependencies(t).Manager
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
	if err := m.StartDirectoryProcessing(ctx, dir, DirectoryProcessSidecar, "", false, "", false); !errors.Is(err, ErrDirectoryWorkInProgress) {
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

func TestCleanupProtectsIndexedVideosInEveryCategory(t *testing.T) {
	for _, overlapping := range []bool{false, true} {
		name := "selected root"
		if overlapping {
			name = "overlapping root"
		}
		t.Run(name, func(t *testing.T) {
			deps := newCleanupTestDependencies(t)
			dir := models.Directory{Path: t.TempDir()}
			if err := deps.database.Create(&dir).Error; err != nil {
				t.Fatal(err)
			}
			indexedDir := dir
			prefix := ""
			if overlapping {
				indexedDir = models.Directory{Path: filepath.Join(dir.Path, "nested")}
				if err := os.Mkdir(indexedDir.Path, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := deps.database.Create(&indexedDir).Error; err != nil {
					t.Fatal(err)
				}
				prefix = "nested/"
			}
			names := []string{"Thumbs.db", "video.txt", "video.torrent", "video.lnk", "qr-video.png", "video.aria2"}
			for _, filename := range names {
				writeCleanupFixture(t, indexedDir.Path, filename)
			}
			writeCleanupFixture(t, dir.Path, "real-junk.txt")
			options := CleanupOptions{IncludeSystem: true, IncludeMisc: true, IncludeTorrents: true, IncludeShortcuts: true, IncludeAdImages: true, IncludeDownloads: true}
			preview, err := deps.PreviewDirectoryCleanup(t.Context(), dir, options)
			if err != nil || len(preview.Files) != len(names)+1 {
				t.Fatalf("preview before indexing: %+v, %v", preview, err)
			}
			// Index after preview: deletion must refresh its snapshot, not trust the selection.
			video := models.Video{Fingerprint: "indexed-misleading-extensions"}
			if err := deps.database.Create(&video).Error; err != nil {
				t.Fatal(err)
			}
			for _, filename := range names {
				loc := models.VideoLocation{VideoID: video.ID, DirectoryID: indexedDir.ID, RelativePath: filename, Filename: filename}
				if err := deps.database.Create(&loc).Error; err != nil {
					t.Fatal(err)
				}
			}
			for _, downloads := range []bool{false, true} {
				options.IncludeDownloads = downloads
				protected, err := deps.PreviewDirectoryCleanup(t.Context(), dir, options)
				if err != nil || len(protected.Files) != 1 || protected.Files[0].Path != "real-junk.txt" {
					t.Fatalf("indexed files offered for cleanup (downloads=%v): %+v, %v", downloads, protected, err)
				}
			}
			// Ordinary categories must be protected even when no download is selected.
			var downloads, ordinary []CleanupFile
			for _, file := range preview.Files {
				if file.Reason == "download" {
					downloads = append(downloads, file)
				} else {
					ordinary = append(ordinary, file)
				}
			}
			result, err := deps.CleanDirectoryFiles(t.Context(), dir, dir.Path, ordinary)
			if err != nil || len(result.Deleted) != 1 || result.Deleted[0] != "real-junk.txt" || len(result.Issues) != len(names)-1 {
				t.Fatalf("ordinary cleanup: %+v, %v", result, err)
			}
			for _, issue := range result.Issues {
				if issue.Reason != "indexed_video" {
					t.Fatalf("wrong protection reason: %+v", issue)
				}
			}
			result, err = deps.CleanDirectoryFiles(t.Context(), dir, dir.Path, downloads)
			if err != nil || len(result.Deleted) != 0 || len(result.Issues) != 1 || result.Issues[0].Reason != "indexed_video" {
				t.Fatalf("download cleanup: %+v, %v", result, err)
			}
			for _, filename := range names {
				if _, err := os.Stat(filepath.Join(dir.Path, filepath.FromSlash(prefix+filename))); err != nil {
					t.Fatalf("indexed video was removed: %s: %v", filename, err)
				}
			}
		})
	}
}

func TestCleanupIndexFailureDoesNotDeleteFiles(t *testing.T) {
	deps := newCleanupTestDependencies(t)
	dir := models.Directory{ID: 1, Path: t.TempDir()}
	writeCleanupFixture(t, dir.Path, "ad.txt")
	preview, err := deps.PreviewDirectoryCleanup(t.Context(), dir, CleanupOptions{IncludeMisc: true})
	if err != nil || len(preview.Files) != 1 {
		t.Fatalf("preview: %+v, %v", preview, err)
	}
	conn, err := deps.database.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := deps.PreviewDirectoryCleanup(t.Context(), dir, CleanupOptions{IncludeMisc: true}); err == nil {
		t.Fatal("preview must fail if the index is unavailable")
	}
	if _, err := deps.CleanDirectoryFiles(t.Context(), dir, dir.Path, preview.Files); err == nil {
		t.Fatal("cleanup must fail if the index is unavailable")
	}
	if _, err := os.Stat(filepath.Join(dir.Path, "ad.txt")); err != nil {
		t.Fatalf("file removed without checking index: %v", err)
	}
	if deps.DirectoryWorkStatus(dir.ID) != DirectoryWorkIdle {
		t.Fatal("failed index lookup leaked the directory reservation")
	}
}
