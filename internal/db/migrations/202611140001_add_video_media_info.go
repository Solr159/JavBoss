package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("202611140001_add_video_media_info.go", addVideoMediaInfo, irreversibleMigration)
}

func addVideoMediaInfo(ctx context.Context, tx *sql.Tx) error {
	return addColumnIfMissing(ctx, tx, "video", "media_info", `text NOT NULL DEFAULT '{}'`)
}
