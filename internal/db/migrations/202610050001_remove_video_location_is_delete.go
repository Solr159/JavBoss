package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("202610050001_remove_video_location_is_delete.go", removeVideoLocationIsDelete, irreversibleMigration)
}

func removeVideoLocationIsDelete(ctx context.Context, tx *sql.Tx) error {
	hasColumn, err := columnExists(ctx, tx, "video_location", "is_delete")
	if err != nil || !hasColumn {
		return err
	}
	// Keep video metadata and history even when its last location is removed.
	for _, statement := range []string{
		`DELETE FROM video_location WHERE COALESCE(is_delete, 0) <> 0`,
		`DROP INDEX IF EXISTS idx_video_location_is_delete`,
		`DROP INDEX IF EXISTS idx_video_location_jav_id_is_delete`,
		`DROP INDEX IF EXISTS idx_video_location_visible_path`,
		`DROP INDEX IF EXISTS idx_video_location_visible_filename`,
		`ALTER TABLE video_location DROP COLUMN is_delete`,
		`CREATE INDEX idx_video_location_visible_path ON video_location(jav_id, relative_path)`,
		`CREATE INDEX idx_video_location_visible_filename ON video_location(jav_id, filename COLLATE NOCASE)`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("remove video location deletion flag: %w", err)
		}
	}
	return nil
}
