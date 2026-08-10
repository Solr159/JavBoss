package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext(
		"202608100001_add_western_metadata.go",
		addWesternMetadata,
		irreversibleMigration,
	)
}

func addWesternMetadata(ctx context.Context, tx *sql.Tx) error {
	return execStatements(ctx, tx,
		`CREATE TABLE IF NOT EXISTS "western_metadata" (
			video_id integer PRIMARY KEY,
			title text,
			original_title text,
			content_type text NOT NULL DEFAULT "scene",
			studio text,
			description text,
			release_date text,
			source text NOT NULL,
			source_id text,
			source_url text,
			cover_url text,
			performers text NOT NULL DEFAULT "[]",
			genres text NOT NULL DEFAULT "[]",
			labels text NOT NULL DEFAULT "[]",
			fetched_at datetime,
			created_at datetime,
			updated_at datetime,
			CONSTRAINT fk_western_metadata_video FOREIGN KEY (video_id) REFERENCES video(id) ON UPDATE CASCADE ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS "idx_western_metadata_content_type" ON "western_metadata" ("content_type")`,
		`CREATE INDEX IF NOT EXISTS "idx_western_metadata_studio" ON "western_metadata" ("studio")`,
		`CREATE INDEX IF NOT EXISTS "idx_western_metadata_source" ON "western_metadata" ("source")`,
	)
}
