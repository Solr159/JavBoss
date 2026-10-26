package directory

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"javboss/internal/db"
	"javboss/internal/models"
)

func TestDeleteUnprocessedVideoLocations(t *testing.T) {
	deps := newTestDependencies()
	for _, tc := range []struct {
		name       string
		rootAbsent bool
		wantErr    bool
		wantCount  int64
		readErrors int
		canceled   bool
	}{
		{name: "only processed locations remain", wantCount: 1},
		{name: "no root reprobe after traversal", rootAbsent: true, wantCount: 1},
		{name: "read errors do not prevent reconciliation", readErrors: 1, wantCount: 1},
		{name: "cancellation prevents cleanup", canceled: true, wantErr: true, wantCount: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gdb, err := db.Open(filepath.Join(t.TempDir(), "scan.db"))
			if err != nil {
				t.Fatal(err)
			}

			deps.database = gdb
			deps.bindDatabase(t)
			t.Cleanup(func() {

				if sqlDB, err := gdb.DB(); err == nil {
					sqlDB.Close()
				}
			})
			directory := models.Directory{Path: t.TempDir()}
			video := models.Video{Fingerprint: "retained-history", WatchedMS: 4321, PlayCount: 2}
			for _, row := range []any{&directory, &video} {
				if err := gdb.Create(row).Error; err != nil {
					t.Fatal(err)
				}
			}
			processed := make(map[int64]struct{})
			for _, name := range []string{"missing.mp4", "unprocessed.mp4", "processed.mp4"} {
				loc := models.VideoLocation{VideoID: video.ID, DirectoryID: directory.ID, RelativePath: name}
				if err := gdb.Create(&loc).Error; err != nil {
					t.Fatal(err)
				}
				if name == "processed.mp4" {
					processed[loc.ID] = struct{}{}
				}
			}
			if tc.rootAbsent {
				if err := os.Remove(directory.Path); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(filepath.Join(directory.Path, "unprocessed.mp4"), []byte("failed probe"), 0600); err != nil {
				t.Fatal(err)
			}
			summary := &Summary{ReadErrors: tc.readErrors}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.canceled {
				cancel()
			}
			err = deleteUnprocessedVideoLocations(ctx, processed, summary, directory)
			if (err != nil) != tc.wantErr {
				t.Fatalf("cleanup error = %v, want error=%t", err, tc.wantErr)
			}
			var count int64
			if err := gdb.Model(&models.VideoLocation{}).Count(&count).Error; err != nil || count != tc.wantCount {
				t.Fatalf("remaining locations = %d, err=%v, want %d", count, err, tc.wantCount)
			}
			if int64(summary.Removed) != 3-tc.wantCount {
				t.Fatalf("removed count = %d", summary.Removed)
			}
			if err := gdb.First(&video, video.ID).Error; err != nil || video.WatchedMS != 4321 || video.PlayCount != 2 {
				t.Fatalf("video history changed: %+v, err=%v", video, err)
			}
			if !tc.canceled {
				var remaining models.VideoLocation
				if err := gdb.First(&remaining).Error; err != nil || remaining.RelativePath != "processed.mp4" {
					t.Fatalf("remaining location = %+v, err=%v", remaining, err)
				}
			}
			if !tc.rootAbsent {
				if _, err := os.Stat(filepath.Join(directory.Path, "unprocessed.mp4")); err != nil {
					t.Fatalf("cleanup must not delete files: %v", err)
				}
			}
		})
	}
}

func TestScanDirectoryContinuesAfterUnreadableSubdirectory(t *testing.T) {
	for _, mode := range []os.FileMode{0100, 0000} {
		t.Run(fmt.Sprintf("mode_%04o", mode), func(t *testing.T) {
			testScanDirectoryWithUnreadableSubdirectory(t, mode)
		})
	}
}

func TestScanDirectoryCleanupAfterScanOutcome(t *testing.T) {
	for _, scenario := range []string{"probe failure", "missing root", "canceled scan"} {
		t.Run(scenario, func(t *testing.T) {
			deps := newTestDependencies()
			gdb, err := db.Open(filepath.Join(t.TempDir(), "scan.db"))
			if err != nil {
				t.Fatal(err)
			}
			deps.database = gdb
			deps.bindDatabase(t)
			t.Cleanup(func() {
				if sqlDB, err := gdb.DB(); err == nil {
					_ = sqlDB.Close()
				}
			})
			dir := models.Directory{Path: t.TempDir()}
			video := models.Video{Fingerprint: "retained"}
			for _, row := range []any{&dir, &video} {
				if err := gdb.Create(row).Error; err != nil {
					t.Fatal(err)
				}
			}
			location := models.VideoLocation{DirectoryID: dir.ID, VideoID: video.ID, RelativePath: "observed.mp4"}
			if err := gdb.Create(&location).Error; err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch scenario {
			case "probe failure":
				if err := os.WriteFile(filepath.Join(dir.Path, location.RelativePath), []byte("not a video"), 0600); err != nil {
					t.Fatal(err)
				}
			case "missing root":
				if err := os.Remove(dir.Path); err != nil {
					t.Fatal(err)
				}
			case "canceled scan":
				cancel()
			}
			summary, err := deps.ScanDirectory(ctx, dir)
			if scenario == "canceled scan" {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("scan error = %v, want cancellation", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if scenario == "probe failure" && (summary.FilesSeen != 1 || summary.Removed != 1) {
				t.Fatalf("unprocessed location was not removed: %+v", summary)
			}
			wantCount := int64(1)
			if scenario == "probe failure" {
				wantCount = 0
			}
			var count int64
			if err := gdb.Model(&models.VideoLocation{}).Where("id = ?", location.ID).Count(&count).Error; err != nil || count != wantCount {
				t.Fatalf("remaining locations = %d, want %d; err=%v", count, wantCount, err)
			}
			if err := gdb.First(&dir, dir.ID).Error; err != nil {
				t.Fatal(err)
			}
			if scenario != "probe failure" && dir.LastScanSummary.FinishedAtUnixMS != 0 {
				t.Fatalf("unscanned directory marked complete: %+v", dir.LastScanSummary)
			}
		})
	}
}

func testScanDirectoryWithUnreadableSubdirectory(t *testing.T, mode os.FileMode) {
	t.Helper()
	deps := newTestDependencies()
	directory := models.Directory{Path: t.TempDir()}
	unreadable := filepath.Join(directory.Path, "a-unreadable")
	if err := os.Mkdir(unreadable, 0700); err != nil {
		t.Fatal(err)
	}
	// Cleanup must succeed whether or not individual paths could be checked.
	if err := os.Chmod(unreadable, mode); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0700) })
	if _, err := os.ReadDir(unreadable); err == nil {
		t.Skip("environment permits reading directories without permission")
	}

	gdb, err := db.Open(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatal(err)
	}

	deps.database = gdb
	deps.bindDatabase(t)
	t.Cleanup(func() {

		if sqlDB, err := gdb.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	video := models.Video{Fingerprint: "readable-video", Size: 1, DurationSec: 1}
	for _, row := range []any{&directory, &video} {
		if err := gdb.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	// Lexical ordering ensures this video is reached after the unreadable subtree.
	path := filepath.Join(directory.Path, "z-readable.mp4")
	if err := os.WriteFile(path, []byte{0}, 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"z-readable.mp4", "a-unreadable/missing.mp4", "missing.mp4"} {
		loc := models.VideoLocation{VideoID: video.ID, DirectoryID: directory.ID, RelativePath: name, ModifiedAt: info.ModTime().UTC()}
		if err := gdb.Create(&loc).Error; err != nil {
			t.Fatal(err)
		}
	}
	summary, err := deps.ScanDirectory(t.Context(), directory)
	if err != nil {
		t.Fatal(err)
	}
	if summary.ReadErrors != 1 || summary.FilesSeen != 1 || summary.Removed != 2 {
		t.Fatalf("partial scan summary = %+v", summary)
	}
	var completed models.Directory
	if err := gdb.First(&completed, directory.ID).Error; err != nil {
		t.Fatal(err)
	}
	if completed.LastScanSummary.FinishedAtUnixMS == 0 || completed.LastScanSummary.Removed != 2 {
		t.Fatalf("scan summary was not saved: %+v", completed.LastScanSummary)
	}
	var count int64
	if err := gdb.Model(&models.VideoLocation{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("partial scan retained %d locations, err=%v", count, err)
	}

	// Once the subtree is readable, no stale locations remain to be removed.
	if err := os.Chmod(unreadable, 0700); err != nil {
		t.Fatal(err)
	}
	summary, err = deps.ScanDirectory(t.Context(), directory)
	if err != nil {
		t.Fatal(err)
	}
	if summary.ReadErrors != 0 || summary.FilesSeen != 1 || summary.Removed != 0 {
		t.Fatalf("recovered scan summary = %+v", summary)
	}
	if err := gdb.Model(&models.VideoLocation{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("recovered scan retained %d locations, err=%v", count, err)
	}
}

func TestWalkAndReconcileVideoFilesHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	directory := models.Directory{Path: t.TempDir()}
	if err := walkAndReconcileVideoFiles(ctx, directory, &syncState{}, &Summary{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("walk error = %v, want context canceled", err)
	}
}

func TestWalkAndReconcileVideoFilesReturnsTraversalError(t *testing.T) {
	// A missing root deterministically exercises a WalkDir error on every platform.

	directory := models.Directory{Path: filepath.Join(t.TempDir(), "unavailable")}
	if err := walkAndReconcileVideoFiles(t.Context(), directory, &syncState{}, &Summary{}); err == nil {
		t.Fatal("incomplete traversal must fail before stale locations can be deleted")
	}
}
