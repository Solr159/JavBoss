package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("202609250001_add_jav_zh_title.go", addJavZhTitle, irreversibleMigration)
}

func addJavZhTitle(ctx context.Context, tx *sql.Tx) error {
	return addColumnIfMissing(ctx, tx, "jav", "zh_title", `text NOT NULL DEFAULT ""`)
}
