package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("202610240001_add_jav_cover_crop_position.go", addJavCoverCropPosition, irreversibleMigration)
}

func addJavCoverCropPosition(ctx context.Context, tx *sql.Tx) error {
	return addColumnIfMissing(ctx, tx, "jav", "cover_crop_position", "real NOT NULL DEFAULT 1")
}
