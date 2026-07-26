package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("202607260002_add_jav_studio_alias.go", addJavStudioAlias, irreversibleMigration)
}

func addJavStudioAlias(ctx context.Context, tx *sql.Tx) error {
	return execStatements(ctx, tx,
		`CREATE TABLE IF NOT EXISTS "jav_studio_alias" (
			id integer PRIMARY KEY AUTOINCREMENT,
			jav_studio_id integer NOT NULL,
			alias text NOT NULL,
			created_at datetime,
			CONSTRAINT fk_jav_studio_alias_jav_studio FOREIGN KEY (jav_studio_id) REFERENCES jav_studio(id) ON UPDATE CASCADE ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_jav_studio_alias_jav_studio_id ON jav_studio_alias(jav_studio_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_jav_studio_alias_alias ON jav_studio_alias(alias)`,
	)
}
