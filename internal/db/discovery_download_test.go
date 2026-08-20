package db

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"javboss/internal/common"
	"javboss/internal/models"
)

func TestClaimNextQueuedDownloadJobClaimsEachJobOnce(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	previousDB := common.DB
	common.DB = database
	t.Cleanup(func() {
		common.DB = previousDB
		if sqlDB, dbErr := database.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	directory := models.Directory{Path: t.TempDir()}
	if err := database.Create(&directory).Error; err != nil {
		t.Fatalf("create directory: %v", err)
	}
	item := models.JavDiscoveryItem{Code: "ABC-001", MetadataJSON: `{}`, MagnetLinksJSON: `[]`}
	if err := database.Create(&item).Error; err != nil {
		t.Fatalf("create discovery item: %v", err)
	}
	if err := SaveDownloaderSettings(context.Background(), &models.DownloaderSettings{
		ActiveProvider: models.DownloaderProviderOpenList, LocalConcurrency: 2,
	}); err != nil {
		t.Fatalf("save downloader settings: %v", err)
	}
	var sourcedJobID int64
	for _, hash := range []string{"hash-one", "hash-two"} {
		var sourceID *int64
		var sourceType *string
		if hash == "hash-one" {
			typeValue := models.DownloadSourceDiscovery
			sourceType = &typeValue
			sourceID = &item.ID
		}
		job := models.DownloadJob{
			SourceType: sourceType, SourceID: sourceID, Code: "ABC-001",
			DirectoryID: directory.ID, InfoHash: hash,
			MagnetURL: "magnet:?xt=urn:btih:" + hash,
			Provider:  models.DownloaderProviderOpenList,
		}
		if err := CreateDownloadJob(context.Background(), &job); err != nil {
			t.Fatalf("create download %s: %v", hash, err)
		}
		if sourceType != nil {
			sourcedJobID = job.ID
		}
	}
	duplicate := models.DownloadJob{
		Code: "OTHER-001", DirectoryID: directory.ID, InfoHash: "hash-one",
		MagnetURL: "magnet:?xt=urn:btih:hash-one", Provider: models.DownloaderProviderOpenList,
	}
	if err := CreateDownloadJob(context.Background(), &duplicate); !errors.Is(err, ErrDownloadJobExists) {
		t.Fatalf("duplicate target hash error = %v", err)
	}
	partialSourceID := item.ID
	partialSource := models.DownloadJob{
		SourceID: &partialSourceID, Code: "OTHER-002", DirectoryID: directory.ID,
		InfoHash: "hash-three", MagnetURL: "magnet:?xt=urn:btih:hash-three",
		Provider: models.DownloaderProviderOpenList,
	}
	if err := CreateDownloadJob(context.Background(), &partialSource); err == nil {
		t.Fatal("create download with source id but no source type succeeded")
	}

	start := make(chan struct{})
	results := make(chan *models.DownloadJob, 2)
	claimErrors := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			job, claimErr := ClaimNextQueuedDownloadJob(context.Background(), models.DownloaderProviderOpenList)
			results <- job
			claimErrors <- claimErr
		}()
	}
	close(start)

	claimed := make(map[int64]bool, 2)
	for range 2 {
		if claimErr := <-claimErrors; claimErr != nil {
			t.Fatalf("claim download: %v", claimErr)
		}
		job := <-results
		if job == nil {
			t.Fatal("claim returned no job")
		}
		if claimed[job.ID] {
			t.Fatalf("job %d was claimed twice", job.ID)
		}
		if job.Status != models.DownloadOfflineDownloading {
			t.Fatalf("claimed status = %q", job.Status)
		}
		claimed[job.ID] = true
	}
	if err := database.Delete(&item).Error; err != nil {
		t.Fatalf("delete download source: %v", err)
	}
	if _, err := GetDownloadJob(context.Background(), sourcedJobID); err != nil {
		t.Fatalf("load download after deleting source: %v", err)
	}
	jobs, err := ListDownloadJobs(context.Background(), 10)
	if err != nil {
		t.Fatalf("list downloads: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("download count = %d, want 2", len(jobs))
	}
	foundSourceFree := false
	for _, job := range jobs {
		if job.SourceType == nil && job.SourceID == nil {
			foundSourceFree = true
		}
	}
	if !foundSourceFree {
		t.Fatal("source-free download job was not preserved")
	}
	if err := SaveDownloaderSettings(context.Background(), &models.DownloaderSettings{
		ActiveProvider: models.DownloaderProviderCloudDrive2, LocalConcurrency: 2,
	}); !errors.Is(err, ErrDownloaderProviderHasActiveJobs) {
		t.Fatalf("switch provider with unfinished jobs error = %v", err)
	}
}
