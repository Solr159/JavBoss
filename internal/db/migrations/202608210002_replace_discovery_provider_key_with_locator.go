package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext(
		"202608210002_replace_discovery_provider_key_with_locator.go",
		replaceDiscoveryProviderKeyWithLocator,
		irreversibleMigration,
	)
}

// replaceDiscoveryProviderKeyWithLocator deliberately discards existing
// discovery data. A bare JavBus star key loses the listing namespace (for
// example, /star and /uncensored/star), so existing subscriptions and all
// items associated through them cannot be trusted.
func replaceDiscoveryProviderKeyWithLocator(ctx context.Context, tx *sql.Tx) error {
	return execStatements(ctx, tx,
		`DELETE FROM jav_discovery_subscription_item`,
		`DELETE FROM jav_discovery_item`,
		`DELETE FROM jav_discovery_subscription`,
		`DELETE FROM sqlite_sequence WHERE name IN ("jav_discovery_subscription", "jav_discovery_item")`,
		`DROP INDEX idx_jav_discovery_subscription_kind_provider_key`,
		`ALTER TABLE jav_discovery_subscription RENAME COLUMN provider_key TO provider_locator`,
		`CREATE UNIQUE INDEX idx_jav_discovery_subscription_kind_provider_locator ON jav_discovery_subscription(kind, provider_locator)`,
	)
}
