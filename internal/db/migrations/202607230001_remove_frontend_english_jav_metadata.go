package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("202607230001_remove_frontend_english_jav_metadata.go", removeFrontendEnglishJavMetadata, irreversibleMigration)
}

func removeFrontendEnglishJavMetadata(ctx context.Context, tx *sql.Tx) error {
	if err := execStatements(ctx, tx,
		`DELETE FROM jav_favorite_map
		 WHERE entity_type = 'idol'
		   AND entity_id IN (SELECT id FROM jav_idol WHERE COALESCE(is_english, 0) <> 0)`,
		`DELETE FROM jav_favorite_map
		 WHERE entity_type = 'series'
		   AND entity_id IN (SELECT id FROM jav_series WHERE COALESCE(is_english, 0) <> 0)`,
		`DELETE FROM jav_idol_alias
		 WHERE jav_idol_id IN (SELECT id FROM jav_idol WHERE COALESCE(is_english, 0) <> 0)`,
		`DELETE FROM jav_idol_map
		 WHERE jav_idol_id IN (SELECT id FROM jav_idol WHERE COALESCE(is_english, 0) <> 0)`,
		`DELETE FROM jav_idol WHERE COALESCE(is_english, 0) <> 0`,
		`DELETE FROM jav_tag_map WHERE provider IN (2, 6)`,
		`DELETE FROM jav_tag
		 WHERE COALESCE(is_user, 0) = 0
		   AND NOT EXISTS (
			SELECT 1 FROM jav_tag_map WHERE jav_tag_map.jav_tag_id = jav_tag.id
		   )`,
		`DELETE FROM config WHERE key = 'jav_metadata_language'`,
	); err != nil {
		return err
	}
	if err := rebuildJavWithoutEnglishTitle(ctx, tx); err != nil {
		return err
	}
	if err := rebuildJavIdolWithoutLanguage(ctx, tx); err != nil {
		return err
	}
	return rebuildJavIdolAliasWithoutLanguage(ctx, tx)
}

func rebuildJavWithoutEnglishTitle(ctx context.Context, tx *sql.Tx) error {
	const columns = `"id", "code", "title", "studio_id", "series_id", "series_en_id", "release_unix", "duration_min", "fetched_at", "created_at", "updated_at", "is_uncensored"`
	return execStatements(ctx, tx,
		`DROP TABLE IF EXISTS "__new_jav"`,
		`CREATE TABLE "__new_jav" (
			id integer PRIMARY KEY AUTOINCREMENT,
			code text,
			title text,
			studio_id integer,
			series_id integer,
			series_en_id integer,
			release_unix integer,
			duration_min integer,
			fetched_at datetime,
			created_at datetime,
			updated_at datetime,
			is_uncensored numeric,
			CONSTRAINT fk_jav_studio FOREIGN KEY (studio_id) REFERENCES jav_studio(id) ON UPDATE CASCADE ON DELETE SET NULL,
			CONSTRAINT fk_jav_series FOREIGN KEY (series_id) REFERENCES jav_series(id) ON UPDATE CASCADE ON DELETE SET NULL,
			CONSTRAINT fk_jav_series_en FOREIGN KEY (series_en_id) REFERENCES jav_series(id) ON UPDATE CASCADE ON DELETE SET NULL
		)`,
		`INSERT INTO "__new_jav" (`+columns+`) SELECT `+columns+` FROM "jav"`,
		`DROP TABLE "jav"`,
		`ALTER TABLE "__new_jav" RENAME TO "jav"`,
		`CREATE UNIQUE INDEX idx_jav_code ON jav(code)`,
		`CREATE INDEX idx_jav_studio_id ON jav(studio_id)`,
		`CREATE INDEX idx_jav_series_id ON jav(series_id)`,
		`CREATE INDEX idx_jav_series_en_id ON jav(series_en_id)`,
	)
}

func rebuildJavIdolWithoutLanguage(ctx context.Context, tx *sql.Tx) error {
	const columns = `"id", "name", "roman_name", "japanese_name", "chinese_name", "height_cm", "birth_date", "bust", "waist", "hips", "cup", "created_at", "updated_at", "cover_jav_id", "cover_crop_left"`
	return execStatements(ctx, tx,
		`DROP TABLE IF EXISTS "__new_jav_idol"`,
		`CREATE TABLE "__new_jav_idol" (
			id integer PRIMARY KEY AUTOINCREMENT,
			name text,
			roman_name text,
			japanese_name text,
			chinese_name text,
			height_cm integer,
			birth_date datetime,
			bust integer,
			waist integer,
			hips integer,
			cup integer,
			created_at datetime,
			updated_at datetime,
			cover_jav_id integer,
			cover_crop_left real NOT NULL DEFAULT 0.53
		)`,
		`INSERT INTO "__new_jav_idol" (`+columns+`) SELECT `+columns+` FROM "jav_idol"`,
		`DROP TABLE "jav_idol"`,
		`ALTER TABLE "__new_jav_idol" RENAME TO "jav_idol"`,
		`CREATE UNIQUE INDEX idx_jav_idol_name ON jav_idol(name)`,
		`CREATE INDEX idx_jav_idol_cover_jav_id ON jav_idol(cover_jav_id)`,
	)
}

func rebuildJavIdolAliasWithoutLanguage(ctx context.Context, tx *sql.Tx) error {
	const columns = `"id", "jav_idol_id", "alias", "created_at"`
	return execStatements(ctx, tx,
		`DROP TABLE IF EXISTS "__new_jav_idol_alias"`,
		`CREATE TABLE "__new_jav_idol_alias" (
			id integer PRIMARY KEY AUTOINCREMENT,
			jav_idol_id integer NOT NULL,
			alias text NOT NULL,
			created_at datetime,
			CONSTRAINT fk_jav_idol_alias_jav_idol FOREIGN KEY (jav_idol_id) REFERENCES jav_idol(id) ON UPDATE CASCADE ON DELETE CASCADE
		)`,
		`INSERT INTO "__new_jav_idol_alias" (`+columns+`) SELECT `+columns+` FROM "jav_idol_alias"`,
		`DROP TABLE "jav_idol_alias"`,
		`ALTER TABLE "__new_jav_idol_alias" RENAME TO "jav_idol_alias"`,
		`CREATE INDEX idx_jav_idol_alias_jav_idol_id ON jav_idol_alias(jav_idol_id)`,
		`CREATE UNIQUE INDEX idx_jav_idol_alias_alias ON jav_idol_alias(alias)`,
	)
}
