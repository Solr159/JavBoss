package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext(
		"202608050001_add_jav_favorite_rating.go",
		addJavFavoriteRating,
		irreversibleMigration,
	)
}

func addJavFavoriteRating(ctx context.Context, tx *sql.Tx) error {
	return addColumnIfMissing(ctx, tx, "jav", "favorite_rating", "real NOT NULL DEFAULT 0")
}
