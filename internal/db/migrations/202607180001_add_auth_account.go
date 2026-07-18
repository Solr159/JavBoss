package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
	"golang.org/x/crypto/bcrypt"
)

func init() {
	goose.AddNamedMigrationContext("202607180001_add_auth_account.go", addAuthAccount, irreversibleMigration)
}

func addAuthAccount(ctx context.Context, tx *sql.Tx) error {
	if err := execDB(ctx, tx, `
		CREATE TABLE auth_account (
			id integer PRIMARY KEY AUTOINCREMENT,
			password_hash text NOT NULL,
			session_version integer NOT NULL DEFAULT 1,
			created_at datetime NOT NULL,
			updated_at datetime NOT NULL
		)
	`); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash default password: %w", err)
	}
	return execDB(ctx, tx, `
		INSERT INTO auth_account (id, password_hash, session_version, created_at, updated_at)
		VALUES (1, ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, string(hash))
}
