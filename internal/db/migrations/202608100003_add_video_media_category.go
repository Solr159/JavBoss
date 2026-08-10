package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext(
		"202608100003_add_video_media_category.go",
		addVideoMediaCategory,
		irreversibleMigration,
	)
}

func addVideoMediaCategory(ctx context.Context, tx *sql.Tx) error {
	return execStatements(ctx, tx,
		`ALTER TABLE "video" ADD COLUMN "media_category" text NOT NULL DEFAULT "auto"`,
		`CREATE INDEX IF NOT EXISTS "idx_video_media_category" ON "video" ("media_category")`,
	)
}
