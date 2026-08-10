package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext(
		"202608100002_add_western_match_status.go",
		addWesternMatchStatus,
		irreversibleMigration,
	)
}

func addWesternMatchStatus(ctx context.Context, tx *sql.Tx) error {
	return execStatements(ctx, tx,
		`ALTER TABLE "western_metadata" ADD COLUMN "match_status" text NOT NULL DEFAULT "matched"`,
		`CREATE INDEX IF NOT EXISTS "idx_western_metadata_match_status" ON "western_metadata" ("match_status")`,
	)
}
