package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("202605210001_drop_jav_provider.go", dropJavProvider, irreversibleMigration)
}

func dropJavProvider(ctx context.Context, tx *sql.Tx) error {
	hasProvider, err := columnExists(ctx, tx, "jav", "provider")
	if err != nil {
		return err
	}
	if !hasProvider {
		return nil
	}
	return rebuildJavTableWithoutProvider(ctx, tx)
}

func rebuildJavTableWithoutProvider(ctx context.Context, tx *sql.Tx) error {
	const columns = `"id", "code", "title", "title_en", "studio_id", "series_id", "series_en_id", "release_unix", "duration_min", "fetched_at", "created_at", "updated_at"`
	return execStatements(ctx, tx,
		`DROP TABLE IF EXISTS "__new_jav"`,
		`CREATE TABLE "__new_jav" (
			id integer PRIMARY KEY AUTOINCREMENT,
			code text,
			title text,
			title_en text,
			studio_id integer,
			series_id integer,
			series_en_id integer,
			release_unix integer,
			duration_min integer,
			fetched_at datetime,
			created_at datetime,
			updated_at datetime,
			CONSTRAINT fk_jav_studio FOREIGN KEY (studio_id) REFERENCES jav_studio(id) ON UPDATE CASCADE ON DELETE SET NULL,
			CONSTRAINT fk_jav_series FOREIGN KEY (series_id) REFERENCES jav_series(id) ON UPDATE CASCADE ON DELETE SET NULL,
			CONSTRAINT fk_jav_series_en FOREIGN KEY (series_en_id) REFERENCES jav_series(id) ON UPDATE CASCADE ON DELETE SET NULL
		)`,
		`INSERT INTO "__new_jav" (`+columns+`)
		 SELECT `+columns+` FROM "jav"`,
		`DROP TABLE "jav"`,
		`ALTER TABLE "__new_jav" RENAME TO "jav"`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_jav_code ON jav(code)`,
		`CREATE INDEX IF NOT EXISTS idx_jav_studio_id ON jav(studio_id)`,
		`CREATE INDEX IF NOT EXISTS idx_jav_series_id ON jav(series_id)`,
		`CREATE INDEX IF NOT EXISTS idx_jav_series_en_id ON jav(series_en_id)`,
	)
}
