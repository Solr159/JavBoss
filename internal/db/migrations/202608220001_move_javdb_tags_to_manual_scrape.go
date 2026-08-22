package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext(
		"202608220001_move_javdb_tags_to_manual_scrape.go",
		moveJavDBTagsToManualScrape,
		irreversibleMigration,
	)
}

func moveJavDBTagsToManualScrape(ctx context.Context, tx *sql.Tx) error {
	if err := execDB(
		ctx,
		tx,
		`DELETE FROM jav_tag_map AS javdb
		 WHERE javdb.provider = ?
		   AND EXISTS (
			SELECT 1
			FROM jav_tag_map AS other
			WHERE other.jav_id = javdb.jav_id
			  AND other.jav_tag_id = javdb.jav_tag_id
			  AND other.provider BETWEEN ? AND ?
			  AND other.provider NOT IN (?, ?)
		   )`,
		providerJavDB,
		providerJavBus,
		providerManualScrape,
		providerUser,
		providerJavDB,
	); err != nil {
		return err
	}
	return execDB(
		ctx,
		tx,
		`UPDATE jav_tag_map SET provider = ? WHERE provider = ?`,
		providerManualScrape,
		providerJavDB,
	)
}
