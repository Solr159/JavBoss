package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("202610210001_add_jav_en_title.go", addJavEnTitle, irreversibleMigration)
}

func addJavEnTitle(ctx context.Context, tx *sql.Tx) error {
	return addColumnIfMissing(ctx, tx, "jav", "en_title", `text NOT NULL DEFAULT ""`)
}
