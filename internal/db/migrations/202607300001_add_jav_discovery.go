package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("202607300001_add_jav_discovery.go", addJavDiscovery, irreversibleMigration)
}

func addJavDiscovery(ctx context.Context, tx *sql.Tx) error {
	return execStatements(ctx, tx,
		`CREATE TABLE IF NOT EXISTS "jav_discovery_subscription" (
			id integer PRIMARY KEY AUTOINCREMENT,
			kind text NOT NULL DEFAULT "idol",
			name text NOT NULL,
			reference_code text NOT NULL,
			provider_key text NOT NULL,
			last_synced_at datetime,
			last_error text NOT NULL DEFAULT "",
			created_at datetime,
			updated_at datetime
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_jav_discovery_subscription_kind_provider_key ON jav_discovery_subscription(kind, provider_key)`,
		`CREATE TABLE IF NOT EXISTS "jav_discovery_item" (
			id integer PRIMARY KEY AUTOINCREMENT,
			code text NOT NULL,
			release_unix integer NOT NULL DEFAULT 0,
			metadata_json text NOT NULL DEFAULT "{}",
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
	)
}
