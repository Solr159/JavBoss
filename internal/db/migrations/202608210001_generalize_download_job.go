package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext(
		"202608210001_generalize_download_job.go",
		generalizeDownloadJob,
		irreversibleMigration,
	)
}

func generalizeDownloadJob(ctx context.Context, tx *sql.Tx) error {
	return execStatements(ctx, tx,
		`CREATE TABLE "download_job" (
			id integer PRIMARY KEY AUTOINCREMENT,
			source_type text,
			source_id integer,
			code text NOT NULL,
			directory_id integer NOT NULL,
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
			completed_at datetime,
			CONSTRAINT fk_download_job_directory FOREIGN KEY (directory_id) REFERENCES directory(id) ON UPDATE CASCADE ON DELETE RESTRICT
		)`,
		`INSERT INTO download_job (
			id, source_type, source_id, code, directory_id, provider, info_hash,
			magnet_url, magnet_name, remote_folder, remote_task_id, status,
			bytes_total, bytes_downloaded, local_files_json, error_message,
			created_at, updated_at, completed_at
		)
		SELECT download.id, "discovery", download.jav_discovery_item_id, item.code,
			download.directory_id, download.provider, download.info_hash,
			download.magnet_url, download.magnet_name, download.remote_folder,
			download.remote_task_id, download.status, download.bytes_total,
			download.bytes_downloaded, download.local_files_json,
			download.error_message, download.created_at, download.updated_at,
			download.completed_at
		FROM jav_discovery_download AS download
		JOIN jav_discovery_item AS item ON item.id = download.jav_discovery_item_id`,
		`DROP TABLE jav_discovery_download`,
		`DELETE FROM download_job
		WHERE id IN (
			SELECT id FROM (
				SELECT id, ROW_NUMBER() OVER (
					PARTITION BY directory_id, info_hash
					ORDER BY
						CASE WHEN status IN ("completed", "failed", "canceled") THEN 1 ELSE 0 END,
						id DESC
				) AS duplicate_order
				FROM download_job
			) WHERE duplicate_order > 1
		)`,
		`CREATE UNIQUE INDEX idx_download_job_target_hash ON download_job(directory_id, info_hash)`,
		`CREATE INDEX idx_download_job_directory_id ON download_job(directory_id)`,
		`CREATE INDEX idx_download_job_source ON download_job(source_type, source_id)`,
		`CREATE INDEX idx_download_job_status ON download_job(status)`,
	)
}
