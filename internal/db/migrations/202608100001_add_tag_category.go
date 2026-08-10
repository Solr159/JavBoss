package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext(
		"202608100001_add_tag_category.go",
		addTagCategories,
		irreversibleMigration,
	)
}

func addTagCategories(ctx context.Context, tx *sql.Tx) error {
	if err := execStatements(ctx, tx,
		`CREATE TABLE IF NOT EXISTS "tag_category" (
			id integer PRIMARY KEY AUTOINCREMENT,
			name text NOT NULL,
			created_at datetime,
			updated_at datetime,
			sort_order integer NOT NULL DEFAULT 0
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS "idx_tag_category_name" ON "tag_category" ("name")`,
	); err != nil {
		return err
	}
	if err := addColumnIfMissing(
		ctx,
		tx,
		"tag",
		"category_id",
		"integer REFERENCES tag_category(id) ON UPDATE CASCADE ON DELETE SET NULL",
	); err != nil {
		return err
	}
	return execDB(ctx, tx, `CREATE INDEX IF NOT EXISTS "idx_tag_category_id" ON "tag" ("category_id")`)
}
