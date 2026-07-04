package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("202607040001_add_video_cover_screenshot_name.go", addVideoCoverScreenshotName, irreversibleMigration)
}

func addVideoCoverScreenshotName(ctx context.Context, tx *sql.Tx) error {
	return addColumnIfMissing(ctx, tx, "video", "cover_screenshot_name", "text")
}
