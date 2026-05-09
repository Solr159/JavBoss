package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("202605090001_add_jav_studio.go", addJavStudio, irreversibleMigration)
}

func addJavStudio(ctx context.Context, tx *sql.Tx) error {
	if err := execDB(ctx, tx, `CREATE TABLE IF NOT EXISTS "jav_studio" (
		id integer PRIMARY KEY AUTOINCREMENT,
		name text,
		created_at datetime,
		updated_at datetime
	)`); err != nil {
		return err
	}
	if err := execDB(ctx, tx, `CREATE UNIQUE INDEX IF NOT EXISTS idx_jav_studio_name ON jav_studio(name)`); err != nil {
		return err
	}
	hasLegacyStudio, err := columnExists(ctx, tx, "jav", "studio")
	if err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, tx, "jav", "studio_id", "integer"); err != nil {
		return err
	}
	if hasLegacyStudio {
		if err := backfillStudiosFromLegacyJavColumn(ctx, tx); err != nil {
			return err
		}
	}
	return rebuildJavTableWithStudio(ctx, tx)
}

func backfillStudiosFromLegacyJavColumn(ctx context.Context, tx *sql.Tx) error {
	if err := execDB(ctx, tx, `
		INSERT OR IGNORE INTO jav_studio (name, created_at, updated_at)
		SELECT DISTINCT TRIM(studio), CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
		FROM jav
		WHERE COALESCE(TRIM(studio), '') <> ''
	`); err != nil {
		return err
	}
	return execDB(ctx, tx, `
		UPDATE jav
		SET studio_id = (
			SELECT jav_studio.id
			FROM jav_studio
			WHERE jav_studio.name = TRIM(jav.studio)
		)
		WHERE studio_id IS NULL
		  AND COALESCE(TRIM(studio), '') <> ''
	`)
}

func rebuildJavTableWithStudio(ctx context.Context, tx *sql.Tx) error {
	const columns = `"id", "code", "title", "title_en", "studio_id", "release_unix", "duration_min", "provider", "fetched_at", "created_at", "updated_at"`
	if err := execStatements(ctx, tx,
		`DROP TABLE IF EXISTS "__new_jav"`,
		`CREATE TABLE "__new_jav" (
			id integer PRIMARY KEY AUTOINCREMENT,
			code text,
			title text,
			title_en text,
			studio_id integer,
			release_unix integer,
			duration_min integer,
			provider integer NOT NULL DEFAULT 0,
			fetched_at datetime,
			created_at datetime,
			updated_at datetime,
			CONSTRAINT fk_jav_studio FOREIGN KEY (studio_id) REFERENCES jav_studio(id) ON UPDATE CASCADE ON DELETE SET NULL
		)`,
		`INSERT INTO "__new_jav" (`+columns+`)
		 SELECT `+columns+` FROM "jav"`,
		`DROP TABLE "jav"`,
		`ALTER TABLE "__new_jav" RENAME TO "jav"`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_jav_code ON jav(code)`,
		`CREATE INDEX IF NOT EXISTS idx_jav_studio_id ON jav(studio_id)`,
	); err != nil {
		return err
	}
	return nil
}
