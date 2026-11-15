package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("202611150001_add_jav_tag_map_tag_index.go", addJavTagMapTagIndex, irreversibleMigration)
}

func addJavTagMapTagIndex(ctx context.Context, tx *sql.Tx) error {
	// Tag counts and the unused-tag check both look up mappings by tag ID.
	return execDB(ctx, tx, `CREATE INDEX IF NOT EXISTS idx_jav_tag_map_tag_provider ON jav_tag_map(jav_tag_id, provider)`)
}
