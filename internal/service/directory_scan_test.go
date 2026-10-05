package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"javboss/internal/common"
	"javboss/internal/db"
	"javboss/internal/models"
)

func TestDeleteUnprocessedVideoLocations(t *testing.T) {
	for _, tc := range []struct {
		name       string
		rootAbsent bool
		wantErr    bool
		wantCount  int64
		readErrors int
	}{
		{name: "only confirmed missing files", wantCount: 1},
		{name: "root disappeared", rootAbsent: true, wantErr: true, wantCount: 2},
		{name: "incomplete scan retains all locations", readErrors: 1, wantCount: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gdb, err := db.Open(filepath.Join(t.TempDir(), "scan.db"))
			if err != nil {
				t.Fatal(err)
			}
			previous := common.DB
			common.DB = gdb
			t.Cleanup(func() {
				common.DB = previous
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
			for _, name := range []string{"missing.mp4", "unprocessed.mp4"} {
				loc := models.VideoLocation{VideoID: video.ID, DirectoryID: directory.ID, RelativePath: name}
				if err := gdb.Create(&loc).Error; err != nil {
					t.Fatal(err)
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
			err = deleteUnprocessedVideoLocations(t.Context(), nil, summary, directory)
			if (err != nil) != tc.wantErr {
				t.Fatalf("cleanup error = %v, want error=%t", err, tc.wantErr)
			}
			var count int64
			if err := gdb.Model(&models.VideoLocation{}).Count(&count).Error; err != nil || count != tc.wantCount {
				t.Fatalf("remaining locations = %d, err=%v, want %d", count, err, tc.wantCount)
			}
			if int64(summary.Removed) != 2-tc.wantCount {
				t.Fatalf("removed count = %d", summary.Removed)
			}
			if err := gdb.First(&video, video.ID).Error; err != nil || video.WatchedMS != 4321 || video.PlayCount != 2 {
				t.Fatalf("video history changed: %+v, err=%v", video, err)
			}
		})
	}
}

func TestScanDirectoryContinuesAfterUnreadableSubdirectory(t *testing.T) {
	directory := models.Directory{Path: t.TempDir()}
	unreadable := filepath.Join(directory.Path, "a-unreadable")
	if err := os.Mkdir(unreadable, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(unreadable, 0); err != nil {
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
	previous := common.DB
	common.DB = gdb
	t.Cleanup(func() {
		common.DB = previous
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
	summary, err := ScanDirectory(t.Context(), directory)
	if err != nil {
		t.Fatal(err)
	}
	if summary.ReadErrors != 1 || summary.FilesSeen != 1 || summary.Removed != 0 {
		t.Fatalf("partial scan summary = %+v", summary)
	}
	var count int64
	if err := gdb.Model(&models.VideoLocation{}).Count(&count).Error; err != nil || count != 3 {
		t.Fatalf("partial scan retained %d locations, err=%v", count, err)
	}

	// Once the subtree is readable, a complete scan can remove missing locations.
	if err := os.Chmod(unreadable, 0700); err != nil {
		t.Fatal(err)
	}
	summary, err = ScanDirectory(t.Context(), directory)
	if err != nil {
		t.Fatal(err)
	}
	if summary.ReadErrors != 0 || summary.FilesSeen != 1 || summary.Removed != 2 {
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
