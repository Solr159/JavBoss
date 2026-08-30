package downloader

import (
	"context"
	"net/http"
)

type OfflineState string

const (
	OfflineNotFound OfflineState = "not_found"
	// OfflineUntracked means the provider accepted or already owns the remote
	// task, but its task record can no longer be used for status polling.
	OfflineUntracked OfflineState = "untracked"
	OfflineRunning   OfflineState = "running"
	OfflineComplete  OfflineState = "complete"
	OfflineFailed    OfflineState = "failed"
	OfflineCanceled  OfflineState = "canceled"
)

type OfflineStatus struct {
	State   OfflineState
	Message string
}

type RemoteFile struct {
	Path string
	Name string
	Size int64
}

type DownloadSource struct {
	URL     string
	Headers http.Header
}

type TestResult struct {
	Provider string         `json:"provider"`
	UserName string         `json:"user_name,omitempty"`
	Folder   string         `json:"folder"`
	Details  map[string]any `json:"details,omitempty"`
}

type Client interface {
	Test(ctx context.Context, folder string) (*TestResult, error)
	EnsureFolder(ctx context.Context, parent, name string) (string, error)
	StartOffline(ctx context.Context, magnet, folder, infoHash string) (string, error)
	OfflineStatus(ctx context.Context, taskID, folder, infoHash string) (*OfflineStatus, error)
	WalkFiles(ctx context.Context, folder string) ([]RemoteFile, error)
	DownloadSource(ctx context.Context, remotePath string) (*DownloadSource, error)
	CancelOffline(ctx context.Context, taskID string) error
	Close() error
}
