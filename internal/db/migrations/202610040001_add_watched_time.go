package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("202610040001_add_watched_time.go", addWatchedTime, irreversibleMigration)
}

func addWatchedTime(ctx context.Context, tx *sql.Tx) error {
	for _, table := range []string{"video", "jav"} {
		if err := addColumnIfMissing(ctx, tx, table, "watched_ms", "integer NOT NULL DEFAULT 0"); err != nil {
			return err
		}
	}
	return nil
}
