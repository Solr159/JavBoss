package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext(
		"202607260001_add_directory_last_scan_summary.go",
		addDirectoryLastScanSummary,
		irreversibleMigration,
	)
}

func addDirectoryLastScanSummary(ctx context.Context, tx *sql.Tx) error {
	return addColumnIfMissing(
		ctx,
		tx,
		"directory",
		"last_scan_summary",
		`text NOT NULL DEFAULT "{}"`,
	)
}
