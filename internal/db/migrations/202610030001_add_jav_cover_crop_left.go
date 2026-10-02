package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("202610030001_add_jav_cover_crop_left.go", addJavCoverCropLeft, irreversibleMigration)
}

func addJavCoverCropLeft(ctx context.Context, tx *sql.Tx) error {
	return addColumnIfMissing(ctx, tx, "jav", "cover_crop_left", `real NOT NULL DEFAULT 0.53`)
}
