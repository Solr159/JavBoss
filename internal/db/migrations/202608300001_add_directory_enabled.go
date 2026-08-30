package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext(
		"202608300001_add_directory_enabled.go",
		addDirectoryEnabled,
		irreversibleMigration,
	)
}

func addDirectoryEnabled(ctx context.Context, tx *sql.Tx) error {
	return addColumnIfMissing(
		ctx,
		tx,
		"directory",
		"enabled",
		`numeric NOT NULL DEFAULT 1`,
	)
}
