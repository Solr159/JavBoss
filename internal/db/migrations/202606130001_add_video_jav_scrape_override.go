package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("202606130001_add_video_jav_scrape_override.go", addVideoJavScrapeOverride, irreversibleMigration)
}

func addVideoJavScrapeOverride(ctx context.Context, tx *sql.Tx) error {
	return addColumnIfMissing(ctx, tx, "video", "jav_scrape_override", "text")
}
