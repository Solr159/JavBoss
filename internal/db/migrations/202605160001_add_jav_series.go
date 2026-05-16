package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("202605160001_add_jav_series.go", addJavSeries, irreversibleMigration)
}

func addJavSeries(ctx context.Context, tx *sql.Tx) error {
	if err := execDB(ctx, tx, `CREATE TABLE IF NOT EXISTS "jav_series" (
		id integer PRIMARY KEY AUTOINCREMENT,
		name text,
		is_english numeric NOT NULL DEFAULT 0,
		studio_id integer,
		created_at datetime,
		updated_at datetime,
		CONSTRAINT fk_jav_series_studio FOREIGN KEY (studio_id) REFERENCES jav_studio(id) ON UPDATE CASCADE ON DELETE SET NULL
	)`); err != nil {
		return err
	}
	if err := execDB(ctx, tx, `CREATE UNIQUE INDEX IF NOT EXISTS idx_jav_series_name_language ON jav_series(name, is_english)`); err != nil {
		return err
	}
	if err := execDB(ctx, tx, `CREATE INDEX IF NOT EXISTS idx_jav_series_studio_id ON jav_series(studio_id)`); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, tx, "jav", "series_id", "integer"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, tx, "jav", "series_en_id", "integer"); err != nil {
		return err
	}
	return rebuildJavTableWithSeries(ctx, tx)
}

func rebuildJavTableWithSeries(ctx context.Context, tx *sql.Tx) error {
	const columns = `"id", "code", "title", "title_en", "studio_id", "series_id", "series_en_id", "release_unix", "duration_min", "provider", "fetched_at", "created_at", "updated_at"`
	if err := execStatements(ctx, tx,
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
			provider integer NOT NULL DEFAULT 0,
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
	); err != nil {
		return err
	}
	return nil
}
