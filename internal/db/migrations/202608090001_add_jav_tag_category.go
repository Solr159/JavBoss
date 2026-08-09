package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext(
		"202608090001_add_jav_tag_category.go",
		addJavTagCategories,
		irreversibleMigration,
	)
}

func addJavTagCategories(ctx context.Context, tx *sql.Tx) error {
	if err := execStatements(ctx, tx,
		`CREATE TABLE IF NOT EXISTS "jav_tag_category" (
			id integer PRIMARY KEY AUTOINCREMENT,
			name text NOT NULL,
			created_at datetime,
			updated_at datetime,
			sort_order integer NOT NULL DEFAULT 0
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS "idx_jav_tag_category_name" ON "jav_tag_category" ("name")`,
	); err != nil {
		return err
	}
	if err := addColumnIfMissing(
		ctx,
		tx,
		"jav_tag",
		"category_id",
		"integer REFERENCES jav_tag_category(id) ON UPDATE CASCADE ON DELETE SET NULL",
	); err != nil {
		return err
	}
	return execDB(ctx, tx, `CREATE INDEX IF NOT EXISTS "idx_jav_tag_category_id" ON "jav_tag" ("category_id")`)
}
