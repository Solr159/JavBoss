package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext(
		"202611010001_add_directory_scan_scope.go",
		addDirectoryScanScope,
		irreversibleMigration,
	)
}

// addDirectoryScanScope stores the optional list of subdirectories a directory
// scans. The default "{}" keeps the historic behaviour of scanning every
// subdirectory below the directory path.
func addDirectoryScanScope(ctx context.Context, tx *sql.Tx) error {
	return addColumnIfMissing(
		ctx,
		tx,
		"directory",
		"scan_scope",
		`text NOT NULL DEFAULT "{}"`,
	)
}
