package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("202608290001_add_downloads.go", addDownloads, irreversibleMigration)
}

func addDownloads(ctx context.Context, tx *sql.Tx) error {
	return execStatements(ctx, tx,
		`CREATE TABLE IF NOT EXISTS "downloader_settings" (
			id integer PRIMARY KEY AUTOINCREMENT,
			active_provider text NOT NULL DEFAULT "",
			download_directory text NOT NULL DEFAULT "",
			local_concurrency integer NOT NULL DEFAULT 2,
			min_video_size_bytes integer NOT NULL DEFAULT 52428800,
			created_at datetime,
			updated_at datetime
		)`,
		`CREATE TABLE IF NOT EXISTS "downloader_provider_settings" (
			provider text PRIMARY KEY,
			address text NOT NULL DEFAULT "",
			api_token text NOT NULL DEFAULT "",
			remote_folder text NOT NULL DEFAULT "",
			created_at datetime,
			updated_at datetime
		)`,
		`CREATE TABLE IF NOT EXISTS "download_job" (
			id integer PRIMARY KEY AUTOINCREMENT,
			download_directory text NOT NULL,
			provider text NOT NULL,
			info_hash text NOT NULL,
			magnet_url text NOT NULL,
			magnet_name text NOT NULL DEFAULT "",
			remote_folder text NOT NULL DEFAULT "",
			remote_task_id text NOT NULL DEFAULT "",
			status text NOT NULL DEFAULT "queued",
			bytes_total integer NOT NULL DEFAULT 0,
			bytes_downloaded integer NOT NULL DEFAULT 0,
			local_files_json text NOT NULL DEFAULT "[]",
			error_message text NOT NULL DEFAULT "",
			created_at datetime,
			updated_at datetime,
			completed_at datetime
		)`,
		`CREATE INDEX IF NOT EXISTS idx_download_job_target_hash ON download_job(download_directory, info_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_download_job_download_directory ON download_job(download_directory)`,
		`CREATE INDEX IF NOT EXISTS idx_download_job_status ON download_job(status)`,
	)
}
