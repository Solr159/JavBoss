package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("202607250001_add_jav_sample_images.go", addJavSampleImages, irreversibleMigration)
}

func addJavSampleImages(ctx context.Context, tx *sql.Tx) error {
	return addColumnIfMissing(ctx, tx, "jav", "sample_images", `text NOT NULL DEFAULT "[]"`)
}
