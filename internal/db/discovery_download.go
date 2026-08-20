package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"javboss/internal/common"
	"javboss/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrDownloadJobExists               = errors.New("download job already exists")
	ErrDownloaderProviderChanged       = errors.New("active downloader provider changed")
	ErrDownloaderProviderHasActiveJobs = errors.New("downloader provider has active jobs")
)

type DownloadJobResult struct {
	models.DownloadJob
	DirectoryPath string   `json:"directory_path"`
	LocalFiles    []string `json:"local_files"`
}

func GetDownloaderSettings(ctx context.Context) (*models.DownloaderSettings, error) {
	if common.DB == nil {
		return nil, errors.New("get downloader settings: nil db")
	}
	var settings models.DownloaderSettings
	err := common.DB.WithContext(ctx).First(&settings, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &models.DownloaderSettings{ID: 1, LocalConcurrency: 2}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get downloader settings: %w", err)
	}
	return &settings, nil
}

func SaveDownloaderSettings(ctx context.Context, settings *models.DownloaderSettings) error {
	if common.DB == nil {
		return errors.New("save downloader settings: nil db")
	}
	if settings == nil {
		return errors.New("save downloader settings: missing settings")
	}
	settings.ID = 1
	settings.ActiveProvider = strings.TrimSpace(settings.ActiveProvider)
	if settings.LocalConcurrency < 1 || settings.LocalConcurrency > 5 {
		settings.LocalConcurrency = 2
	}
	if err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current models.DownloaderSettings
		err := tx.First(&current, 1).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if current.ActiveProvider != settings.ActiveProvider {
			terminal := []string{
				models.DownloadCompleted,
				models.DownloadFailed,
				models.DownloadCanceled,
			}
			var count int64
			if err := tx.Model(&models.DownloadJob{}).Where("status NOT IN ?", terminal).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return ErrDownloaderProviderHasActiveJobs
			}
		}
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"active_provider", "directory_id", "local_concurrency", "updated_at",
			}),
		}).Create(settings).Error
	}); err != nil {
		if errors.Is(err, ErrDownloaderProviderHasActiveJobs) {
			return err
		}
		return fmt.Errorf("save downloader settings: %w", err)
	}
	return nil
}

func GetDownloaderProviderSettings(ctx context.Context, provider string) (*models.DownloaderProviderSettings, error) {
	if common.DB == nil {
		return nil, errors.New("get downloader provider settings: nil db")
	}
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return nil, errors.New("get downloader provider settings: missing provider")
	}
	var settings models.DownloaderProviderSettings
	err := common.DB.WithContext(ctx).First(&settings, "provider = ?", provider).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &models.DownloaderProviderSettings{Provider: provider}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get downloader provider settings: %w", err)
	}
	return &settings, nil
}

func SaveDownloaderProviderSettings(ctx context.Context, settings *models.DownloaderProviderSettings) error {
	if common.DB == nil {
		return errors.New("save downloader provider settings: nil db")
	}
	if settings == nil {
		return errors.New("save downloader provider settings: missing settings")
	}
	settings.Provider = strings.TrimSpace(settings.Provider)
	if settings.Provider == "" {
		return errors.New("save downloader provider settings: missing provider")
	}
	settings.Address = strings.TrimSpace(settings.Address)
	settings.RemoteFolder = strings.TrimSpace(settings.RemoteFolder)
	if err := common.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "provider"}},
		DoUpdates: clause.AssignmentColumns([]string{"address", "api_token", "remote_folder", "updated_at"}),
	}).Create(settings).Error; err != nil {
		return fmt.Errorf("save downloader provider settings: %w", err)
	}
	return nil
}

func CreateDownloadJob(ctx context.Context, job *models.DownloadJob) error {
	if common.DB == nil {
		return errors.New("create download job: nil db")
	}
	if job == nil {
		return errors.New("create download job: missing job")
	}
	job.Code = strings.TrimSpace(job.Code)
	job.InfoHash = strings.TrimSpace(job.InfoHash)
	job.MagnetURL = strings.TrimSpace(job.MagnetURL)
	job.MagnetName = strings.TrimSpace(job.MagnetName)
	hasSource := job.SourceType != nil || job.SourceID != nil
	if hasSource && (job.SourceType == nil || job.SourceID == nil || strings.TrimSpace(*job.SourceType) == "" || *job.SourceID <= 0) {
		return errors.New("create download job: incomplete source")
	}
	if job.SourceType != nil {
		normalizedSourceType := strings.TrimSpace(*job.SourceType)
		job.SourceType = &normalizedSourceType
	}
	if job.DirectoryID <= 0 || job.Code == "" || job.InfoHash == "" || job.MagnetURL == "" {
		return errors.New("create download job: invalid job")
	}
	if job.Provider != models.DownloaderProviderCloudDrive2 && job.Provider != models.DownloaderProviderOpenList {
		return errors.New("create download job: invalid provider")
	}
	job.Status = models.DownloadQueued
	job.LocalFilesJSON = "[]"
	err := common.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var settings models.DownloaderSettings
		if err := tx.First(&settings, 1).Error; err != nil {
			return err
		}
		if settings.ActiveProvider != job.Provider {
			return ErrDownloaderProviderChanged
		}
		return tx.Create(job).Error
	})
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return ErrDownloadJobExists
		}
		if errors.Is(err, ErrDownloaderProviderChanged) {
			return err
		}
		return fmt.Errorf("create download job: %w", err)
	}
	return nil
}

func GetDownloadJob(ctx context.Context, id int64) (*models.DownloadJob, error) {
	if common.DB == nil {
		return nil, errors.New("get download job: nil db")
	}
	var job models.DownloadJob
	if err := common.DB.WithContext(ctx).First(&job, id).Error; err != nil {
		return nil, fmt.Errorf("get download job: %w", err)
	}
	return &job, nil
}

func ListDownloadJobs(ctx context.Context, limit int) ([]DownloadJobResult, error) {
	if common.DB == nil {
		return nil, errors.New("list download jobs: nil db")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var rows []struct {
		models.DownloadJob
		DirectoryPath string `gorm:"column:directory_path"`
	}
	if err := common.DB.WithContext(ctx).
		Table("download_job AS download").
		Select("download.*, directory.path AS directory_path").
		Joins("JOIN directory ON directory.id = download.directory_id").
		Order("download.created_at DESC, download.id DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list download jobs: %w", err)
	}
	result := make([]DownloadJobResult, 0, len(rows))
	for _, row := range rows {
		files := []string{}
		_ = json.Unmarshal([]byte(row.LocalFilesJSON), &files)
		result = append(result, DownloadJobResult{
			DownloadJob:   row.DownloadJob,
			DirectoryPath: row.DirectoryPath,
			LocalFiles:    files,
		})
	}
	return result, nil
}

func ResetInterruptedDownloadJobs(ctx context.Context) error {
	if common.DB == nil {
		return errors.New("reset download jobs: nil db")
	}
	active := []string{
		models.DownloadOfflineDownloading,
		models.DownloadResolvingFiles,
		models.DownloadWaitingLocal,
		models.DownloadLocalDownloading,
	}
	if err := common.DB.WithContext(ctx).Model(&models.DownloadJob{}).
		Where("status IN ?", active).
		Updates(map[string]any{"status": models.DownloadQueued, "error_message": ""}).Error; err != nil {
		return fmt.Errorf("reset download jobs: %w", err)
	}
	return nil
}

func ClaimNextQueuedDownloadJob(ctx context.Context, provider string) (*models.DownloadJob, error) {
	if common.DB == nil {
		return nil, errors.New("claim download job: nil db")
	}
	if provider != models.DownloaderProviderCloudDrive2 && provider != models.DownloaderProviderOpenList {
		return nil, errors.New("claim download job: invalid provider")
	}
	for attempts := 0; attempts < 10; attempts++ {
		var job models.DownloadJob
		err := common.DB.WithContext(ctx).
			Where("status = ? AND provider = ?", models.DownloadQueued, provider).
			Order("created_at, id").
			First(&job).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("claim download job: %w", err)
		}
		result := common.DB.WithContext(ctx).Model(&models.DownloadJob{}).
			Where("id = ? AND status = ? AND provider = ?", job.ID, models.DownloadQueued, provider).
			Updates(map[string]any{
				"status": models.DownloadOfflineDownloading, "error_message": "",
			})
		if result.Error != nil {
			return nil, fmt.Errorf("claim download job: %w", result.Error)
		}
		if result.RowsAffected == 1 {
			job.Status = models.DownloadOfflineDownloading
			job.ErrorMessage = ""
			return &job, nil
		}
	}
	return nil, errors.New("claim download job: too much contention")
}

func UpdateDownloadJob(ctx context.Context, id int64, updates map[string]any) error {
	if common.DB == nil {
		return errors.New("update download job: nil db")
	}
	if id <= 0 || len(updates) == 0 {
		return nil
	}
	if err := common.DB.WithContext(ctx).Model(&models.DownloadJob{}).
		Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("update download job: %w", err)
	}
	return nil
}

func RetryDownloadJob(ctx context.Context, id int64) error {
	result := common.DB.WithContext(ctx).Model(&models.DownloadJob{}).
		Where("id = ? AND status IN ?", id, []string{models.DownloadFailed, models.DownloadCanceled}).
		Updates(map[string]any{
			"status": models.DownloadQueued, "error_message": "", "completed_at": nil,
			"remote_task_id": "", "bytes_total": 0, "bytes_downloaded": 0,
		})
	if result.Error != nil {
		return fmt.Errorf("retry download job: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func CancelDownloadJob(ctx context.Context, id int64) error {
	result := common.DB.WithContext(ctx).Model(&models.DownloadJob{}).
		Where("id = ? AND status NOT IN ?", id, []string{models.DownloadCompleted, models.DownloadCanceled}).
		Updates(map[string]any{"status": models.DownloadCanceled, "error_message": ""})
	if result.Error != nil {
		return fmt.Errorf("cancel download job: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func DeleteDownloadJob(ctx context.Context, id int64) error {
	result := common.DB.WithContext(ctx).
		Where("id = ? AND status IN ?", id, []string{
			models.DownloadCompleted, models.DownloadFailed, models.DownloadCanceled,
		}).Delete(&models.DownloadJob{})
	if result.Error != nil {
		return fmt.Errorf("delete download job: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func CompleteDownloadJob(ctx context.Context, id int64, files []string, total int64) error {
	raw, err := json.Marshal(files)
	if err != nil {
		return fmt.Errorf("encode download job local files: %w", err)
	}
	now := time.Now().UTC()
	return UpdateDownloadJob(ctx, id, map[string]any{
		"status":           models.DownloadCompleted,
		"bytes_total":      total,
		"bytes_downloaded": total,
		"local_files_json": string(raw),
		"error_message":    "",
		"completed_at":     &now,
	})
}
