package db

import (
	"context"
	"path/filepath"
	"testing"

	"javboss/internal/common"
	"javboss/internal/models"
)

func TestCreateDownloadJobAllowsRepeatedMagnet(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "download.db"))
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

	downloadDirectory := t.TempDir()
	defaults, err := GetDownloaderSettings(context.Background())
	if err != nil {
		t.Fatalf("get default downloader settings: %v", err)
	}
	if defaults.ActiveProvider != models.DownloaderProviderCloudDrive2 || defaults.MinVideoSizeBytes != models.DefaultMinVideoSizeBytes {
		t.Fatalf("default small-video settings = %#v", defaults)
	}
	if err := SaveDownloaderSettings(context.Background(), &models.DownloaderSettings{
		ActiveProvider:    models.DownloaderProviderCloudDrive2,
		DownloadDirectory: downloadDirectory, LocalConcurrency: 2,
	}); err != nil {
		t.Fatalf("save downloader settings: %v", err)
	}

	job := &models.DownloadJob{
		DownloadDirectory: downloadDirectory,
		InfoHash:          "0123456789abcdef0123456789abcdef01234567",
		MagnetURL:         "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567",
		MagnetName:        "manual download",
		Provider:          models.DownloaderProviderCloudDrive2,
	}
	if err := CreateDownloadJob(context.Background(), job); err != nil {
		t.Fatalf("create download job: %v", err)
	}
	if job.ID <= 0 || job.Status != models.DownloadQueued {
		t.Fatalf("created download job = %#v", job)
	}

	duplicate := *job
	duplicate.ID = 0
	if err := CreateDownloadJob(context.Background(), &duplicate); err != nil {
		t.Fatalf("create repeated download job: %v", err)
	}
	if duplicate.ID <= 0 || duplicate.ID == job.ID {
		t.Fatalf("repeated download job id = %d, first id = %d", duplicate.ID, job.ID)
	}
	var count int64
	if err := database.Model(&models.DownloadJob{}).
		Where("download_directory = ? AND info_hash = ?", downloadDirectory, job.InfoHash).
		Count(&count).Error; err != nil {
		t.Fatalf("count repeated download jobs: %v", err)
	}
	if count != 2 {
		t.Fatalf("repeated download job count = %d, want 2", count)
	}
	jobs, err := ListDownloadJobs(context.Background(), 10)
	if err != nil {
		t.Fatalf("list download jobs: %v", err)
	}
	if len(jobs) != 2 || jobs[0].MagnetURL != job.MagnetURL {
		t.Fatalf("listed download jobs = %#v", jobs)
	}
}

func TestCreateDownloadJobUsesForcedCloudDrive2Provider(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "forced-provider.db"))
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

	downloadDirectory := t.TempDir()
	if err := SaveDownloaderSettings(context.Background(), &models.DownloaderSettings{
		DownloadDirectory: downloadDirectory, LocalConcurrency: 2,
		MinVideoSizeBytes: 75 * 1024 * 1024,
	}); err != nil {
		t.Fatalf("save downloader settings: %v", err)
	}
	settings, err := GetDownloaderSettings(context.Background())
	if err != nil {
		t.Fatalf("get downloader settings: %v", err)
	}
	if settings.ActiveProvider != models.DownloaderProviderCloudDrive2 || settings.MinVideoSizeBytes != 75*1024*1024 {
		t.Fatalf("stored small-video settings = %#v", settings)
	}
	job := &models.DownloadJob{
		DownloadDirectory: downloadDirectory,
		InfoHash:          "0123456789abcdef0123456789abcdef01234567",
		MagnetURL:         "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567",
		Provider:          models.DownloaderProviderCloudDrive2,
	}
	if err := CreateDownloadJob(context.Background(), job); err != nil {
		t.Fatalf("create download job: %v", err)
	}
}
