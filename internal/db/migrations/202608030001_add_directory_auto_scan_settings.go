package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext(
		"202608030001_add_directory_auto_scan_settings.go",
		addDirectoryAutoScanSettings,
		irreversibleMigration,
	)
}

func addDirectoryAutoScanSettings(ctx context.Context, tx *sql.Tx) error {
	if err := addColumnIfMissing(
		ctx,
		tx,
		"directory",
		"auto_scan_enabled",
		`numeric NOT NULL DEFAULT 1`,
	); err != nil {
		return err
	}
	return addColumnIfMissing(
		ctx,
		tx,
		"directory",
		"auto_scan_interval_minutes",
		`integer NOT NULL DEFAULT 1`,
	)
}
