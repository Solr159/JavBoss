package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("202606060001_add_jav_idol_cover_selection.go", addJavIdolCoverSelection, irreversibleMigration)
}

func addJavIdolCoverSelection(ctx context.Context, tx *sql.Tx) error {
	if err := addColumnIfMissing(ctx, tx, "jav_idol", "cover_jav_id", "integer"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, tx, "jav_idol", "cover_crop_left", "real NOT NULL DEFAULT 0.53"); err != nil {
		return err
	}
	return execStatements(ctx, tx,
		`CREATE INDEX IF NOT EXISTS idx_jav_idol_cover_jav_id ON jav_idol(cover_jav_id)`,
	)
}
