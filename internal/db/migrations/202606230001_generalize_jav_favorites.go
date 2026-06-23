package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("202606230001_generalize_jav_favorites.go", generalizeJavFavorites, irreversibleMigration)
}

func generalizeJavFavorites(ctx context.Context, tx *sql.Tx) error {
	return execStatements(ctx, tx,
		`CREATE TABLE IF NOT EXISTS "jav_favorite_group" (
			id integer PRIMARY KEY AUTOINCREMENT,
			entity_type text NOT NULL DEFAULT "idol",
			name text,
			created_at datetime,
			updated_at datetime,
			sort_order integer NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS "jav_favorite_map" (
			jav_favorite_group_id integer,
			entity_type text NOT NULL DEFAULT "idol",
			entity_id integer,
			created_at datetime,
			sort_order integer NOT NULL DEFAULT 0,
			PRIMARY KEY (jav_favorite_group_id, entity_id),
			CONSTRAINT fk_jav_favorite_map_jav_favorite_group FOREIGN KEY (jav_favorite_group_id) REFERENCES jav_favorite_group(id) ON UPDATE CASCADE ON DELETE CASCADE
		)`,
		`INSERT OR IGNORE INTO "jav_favorite_group" (id, entity_type, name, created_at, updated_at, sort_order)
			SELECT id, 'idol', name, created_at, updated_at, COALESCE(sort_order, 0)
			FROM "jav_idol_favorite_group"
			WHERE EXISTS (SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = 'jav_idol_favorite_group')`,
		`INSERT OR IGNORE INTO "jav_favorite_map" (jav_favorite_group_id, entity_type, entity_id, created_at, sort_order)
			SELECT jav_idol_favorite_group_id, 'idol', jav_idol_id, created_at, COALESCE(sort_order, 0)
			FROM "jav_idol_favorite_map"
			WHERE EXISTS (SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = 'jav_idol_favorite_map')`,
		`DROP TABLE IF EXISTS "jav_idol_favorite_map"`,
		`DROP TABLE IF EXISTS "jav_idol_favorite_group"`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_jav_favorite_group_type_name ON jav_favorite_group(entity_type, name)`,
		`CREATE INDEX IF NOT EXISTS idx_jav_favorite_group_type_sort ON jav_favorite_group(entity_type, sort_order)`,
		`CREATE INDEX IF NOT EXISTS idx_jav_favorite_map_entity_type_entity_id_group_id ON jav_favorite_map(entity_type, entity_id, jav_favorite_group_id)`,
		`CREATE INDEX IF NOT EXISTS idx_jav_favorite_map_sort_order ON jav_favorite_map(sort_order)`,
		`CREATE VIEW IF NOT EXISTS "jav_idol_favorite_group" AS
			SELECT id, name, created_at, updated_at, sort_order
			FROM "jav_favorite_group"
			WHERE entity_type = 'idol'`,
		`CREATE VIEW IF NOT EXISTS "jav_idol_favorite_map" AS
			SELECT jav_favorite_group_id AS jav_idol_favorite_group_id,
				entity_id AS jav_idol_id,
				created_at,
				sort_order
			FROM "jav_favorite_map"
			WHERE entity_type = 'idol'`,
		`CREATE TRIGGER IF NOT EXISTS jav_idol_favorite_map_insert
			INSTEAD OF INSERT ON jav_idol_favorite_map
			BEGIN
				INSERT OR IGNORE INTO jav_favorite_map (jav_favorite_group_id, entity_type, entity_id, created_at, sort_order)
				VALUES (NEW.jav_idol_favorite_group_id, 'idol', NEW.jav_idol_id, NEW.created_at, COALESCE(NEW.sort_order, 0));
			END`,
		`CREATE TRIGGER IF NOT EXISTS jav_idol_favorite_map_delete
			INSTEAD OF DELETE ON jav_idol_favorite_map
			BEGIN
				DELETE FROM jav_favorite_map
				WHERE jav_favorite_group_id = OLD.jav_idol_favorite_group_id
					AND entity_type = 'idol'
					AND entity_id = OLD.jav_idol_id;
			END`,
		`CREATE TRIGGER IF NOT EXISTS jav_idol_favorite_map_update
			INSTEAD OF UPDATE ON jav_idol_favorite_map
			BEGIN
				UPDATE jav_favorite_map
				SET sort_order = COALESCE(NEW.sort_order, sort_order),
					created_at = NEW.created_at
				WHERE jav_favorite_group_id = OLD.jav_idol_favorite_group_id
					AND entity_type = 'idol'
					AND entity_id = OLD.jav_idol_id;
			END`,
	)
}
