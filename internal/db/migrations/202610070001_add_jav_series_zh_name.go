package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("202610070001_add_jav_series_zh_name.go", addJavSeriesZhName, irreversibleMigration)
}

func addJavSeriesZhName(ctx context.Context, tx *sql.Tx) error {
	return addColumnIfMissing(ctx, tx, "jav_series", "zh_name", `text NOT NULL DEFAULT ""`)
}
