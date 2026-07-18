package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("202607180002_add_auth_session.go", addAuthSession, irreversibleMigration)
}

func addAuthSession(ctx context.Context, tx *sql.Tx) error {
	return execStatements(ctx, tx,
		`CREATE TABLE auth_session (
			token_hash text PRIMARY KEY,
			session_version integer NOT NULL,
			expires_at datetime NOT NULL,
			created_at datetime NOT NULL
		)`,
		`CREATE INDEX idx_auth_session_expires_at ON auth_session (expires_at)`,
	)
}
