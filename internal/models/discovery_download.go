package models

import "time"

const (
	DownloaderProviderCloudDrive2 = "clouddrive2"
	DownloaderProviderOpenList    = "openlist"

	DownloadSourceDiscovery = "discovery"
	DownloadSourceJav       = "jav"

	DownloadQueued             = "queued"
	DownloadOfflineDownloading = "offline_downloading"
	DownloadResolvingFiles     = "resolving_files"
	DownloadWaitingLocal       = "waiting_local_download"
	DownloadLocalDownloading   = "local_downloading"
	DownloadCompleted          = "completed"
	DownloadFailed             = "failed"
	DownloadCanceled           = "canceled"
)

// DownloaderSettings contains the provider-independent singleton settings.
type DownloaderSettings struct {
	ID               int64     `json:"-" gorm:"primaryKey"`
	ActiveProvider   string    `json:"active_provider" gorm:"not null;default:''"`
	DirectoryID      *int64    `json:"directory_id" gorm:"index"`
	LocalConcurrency int       `json:"local_concurrency" gorm:"not null;default:2"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// DownloaderProviderSettings stores the shared connection fields for one
// provider. Provider-specific validation belongs to the provider adapter.
type DownloaderProviderSettings struct {
	Provider     string    `json:"provider" gorm:"primaryKey"`
	Address      string    `json:"address" gorm:"not null;default:''"`
	APIToken     string    `json:"-" gorm:"type:text;not null;default:''"`
	RemoteFolder string    `json:"remote_folder" gorm:"not null;default:''"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// DownloadJob is a persistent, source-independent download job. Code is an
// immutable execution snapshot; SourceType and SourceID are optional metadata.
// The remote URL stays server-side because magnet URLs may contain tokens.
type DownloadJob struct {
	ID              int64      `json:"id" gorm:"primaryKey"`
	SourceType      *string    `json:"source_type" gorm:"index:idx_download_job_source,priority:1"`
	SourceID        *int64     `json:"source_id" gorm:"index:idx_download_job_source,priority:2"`
	Code            string     `json:"code" gorm:"not null"`
	DirectoryID     int64      `json:"directory_id" gorm:"not null;index;uniqueIndex:idx_download_job_target_hash,priority:1"`
	Directory       Directory  `json:"-" gorm:"foreignKey:DirectoryID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	Provider        string     `json:"provider" gorm:"not null"`
	InfoHash        string     `json:"info_hash" gorm:"not null;uniqueIndex:idx_download_job_target_hash,priority:2"`
	MagnetURL       string     `json:"-" gorm:"type:text;not null"`
	MagnetName      string     `json:"magnet_name" gorm:"not null;default:''"`
	RemoteFolder    string     `json:"remote_folder" gorm:"not null;default:''"`
	RemoteTaskID    string     `json:"remote_task_id" gorm:"not null;default:''"`
	Status          string     `json:"status" gorm:"not null;default:queued;index"`
	BytesTotal      int64      `json:"bytes_total" gorm:"not null;default:0"`
	BytesDownloaded int64      `json:"bytes_downloaded" gorm:"not null;default:0"`
	LocalFilesJSON  string     `json:"-" gorm:"type:text;not null;default:'[]'"`
	ErrorMessage    string     `json:"error_message" gorm:"type:text;not null;default:''"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	CompletedAt     *time.Time `json:"completed_at"`
}
