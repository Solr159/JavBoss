package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("202608170001_add_jav_discovery.go", addJavDiscovery, irreversibleMigration)
}

func addJavDiscovery(ctx context.Context, tx *sql.Tx) error {
	return execStatements(ctx, tx,
		`CREATE TABLE IF NOT EXISTS "jav_discovery_subscription" (
			id integer PRIMARY KEY AUTOINCREMENT,
			kind text NOT NULL DEFAULT "idol",
			name text NOT NULL,
			reference_code text NOT NULL,
			provider_locator text NOT NULL,
			last_synced_at datetime,
			last_error text NOT NULL DEFAULT "",
			created_at datetime,
			updated_at datetime
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_jav_discovery_subscription_kind_provider_locator ON jav_discovery_subscription(kind, provider_locator)`,
		`CREATE TABLE IF NOT EXISTS "jav_discovery_item" (
			id integer PRIMARY KEY AUTOINCREMENT,
			code text NOT NULL,
			release_unix integer NOT NULL DEFAULT 0,
			metadata_json text NOT NULL DEFAULT "{}",
			magnet_links_json text NOT NULL DEFAULT "null",
			wanted numeric NOT NULL DEFAULT 0,
			created_at datetime,
			updated_at datetime
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_jav_discovery_item_code ON jav_discovery_item(code)`,
		`CREATE INDEX IF NOT EXISTS idx_jav_discovery_item_release_unix ON jav_discovery_item(release_unix)`,
		`CREATE INDEX IF NOT EXISTS idx_jav_discovery_item_wanted ON jav_discovery_item(wanted)`,
		`CREATE TABLE IF NOT EXISTS "jav_discovery_subscription_item" (
			jav_discovery_subscription_id integer,
			jav_discovery_item_id integer,
			created_at datetime,
			PRIMARY KEY (jav_discovery_subscription_id, jav_discovery_item_id),
			CONSTRAINT fk_jav_discovery_subscription_item_jav_discovery_subscription FOREIGN KEY (jav_discovery_subscription_id) REFERENCES jav_discovery_subscription(id) ON UPDATE CASCADE ON DELETE CASCADE,
			CONSTRAINT fk_jav_discovery_subscription_item_jav_discovery_item FOREIGN KEY (jav_discovery_item_id) REFERENCES jav_discovery_item(id) ON UPDATE CASCADE ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_jav_discovery_subscription_item_jav_discovery_item_id ON jav_discovery_subscription_item(jav_discovery_item_id)`,
		`CREATE TABLE IF NOT EXISTS "downloader_settings" (
			id integer PRIMARY KEY AUTOINCREMENT,
			active_provider text NOT NULL DEFAULT "",
			directory_id integer,
			local_concurrency integer NOT NULL DEFAULT 2,
			created_at datetime,
			updated_at datetime
		)`,
		`CREATE INDEX IF NOT EXISTS idx_downloader_settings_directory_id ON downloader_settings(directory_id)`,
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
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_download_job_target_hash ON download_job(directory_id, info_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_download_job_directory_id ON download_job(directory_id)`,
		`CREATE INDEX IF NOT EXISTS idx_download_job_source ON download_job(source_type, source_id)`,
		`CREATE INDEX IF NOT EXISTS idx_download_job_status ON download_job(status)`,
	)
}
