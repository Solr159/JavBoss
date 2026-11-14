package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("202611070001_add_video_subtitle_settings.go", addVideoSubtitleSettings, irreversibleMigration)
}

func addVideoSubtitleSettings(ctx context.Context, tx *sql.Tx) error {
	return addColumnIfMissing(ctx, tx, "video", "subtitle_settings", `text NOT NULL DEFAULT '{}'`)
}
