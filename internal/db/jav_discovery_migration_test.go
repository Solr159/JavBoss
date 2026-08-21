package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
)

const (
	currentSchemaMigrationVersion = int64(202608100001)
	javDiscoveryMigrationVersion  = int64(202608170001)
)

func TestJavDiscoveryMigrationAppliesAfterCurrentSchema(t *testing.T) {
	driverName := registerSQLiteFunctions()
	sqlDB, err := sql.Open(driverName, filepath.Join(t.TempDir(), "jav-discovery-upgrade.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})
	sqlDB.SetMaxOpenConns(1)

	ctx := context.Background()
	if err := execDB(ctx, sqlDB, "PRAGMA foreign_keys=OFF;"); err != nil {
		t.Fatalf("disable foreign keys: %v", err)
	}
	if err := goose.UpToContext(ctx, sqlDB, migrationDir, currentSchemaMigrationVersion); err != nil {
		t.Fatalf("migrate to current schema: %v", err)
	}
	assertMigrationVersion(t, sqlDB, currentSchemaMigrationVersion)
	assertTablesExist(t, sqlDB, false,
		"jav_discovery_subscription",
		"jav_discovery_item",
		"jav_discovery_subscription_item",
		"downloader_settings",
		"downloader_provider_settings",
		"jav_discovery_download",
		"download_job",
	)

	if err := goose.UpToContext(ctx, sqlDB, migrationDir, javDiscoveryMigrationVersion); err != nil {
		t.Fatalf("apply JAV discovery migration: %v", err)
	}
	assertMigrationVersion(t, sqlDB, javDiscoveryMigrationVersion)
	assertTablesExist(t, sqlDB, true,
		"jav_discovery_subscription",
		"jav_discovery_item",
		"jav_discovery_subscription_item",
		"downloader_settings",
		"downloader_provider_settings",
		"download_job",
	)
	assertTablesExist(t, sqlDB, false, "jav_discovery_download")
	if _, err := sqlDB.ExecContext(ctx, `
		INSERT INTO jav_discovery_subscription (kind, name, reference_code, provider_locator)
		VALUES ("idol", "actress", "ABC-001", "/uncensored/star/abc")
	`); err != nil {
		t.Fatalf("insert subscription with provider locator: %v", err)
	}
	if _, err := sqlDB.ExecContext(ctx, `INSERT INTO jav_discovery_item (code) VALUES (?)`, "ABC-001"); err != nil {
		t.Fatalf("insert discovery item with final defaults: %v", err)
	}
	var magnetLinksJSON string
	if err := sqlDB.QueryRowContext(ctx, `SELECT magnet_links_json FROM jav_discovery_item WHERE code = ?`, "ABC-001").Scan(&magnetLinksJSON); err != nil {
		t.Fatalf("read discovery magnet links: %v", err)
	}
	if magnetLinksJSON != "null" {
		t.Fatalf("magnet links default = %q, want JSON null", magnetLinksJSON)
	}
}

func assertMigrationVersion(t *testing.T, sqlDB *sql.DB, want int64) {
	t.Helper()
	got, err := goose.GetDBVersion(sqlDB)
	if err != nil {
		t.Fatalf("get migration version: %v", err)
	}
	if got != want {
		t.Fatalf("migration version = %d, want %d", got, want)
	}
}

func assertTablesExist(t *testing.T, sqlDB *sql.DB, want bool, tables ...string) {
	t.Helper()
	for _, table := range tables {
		var count int
		if err := sqlDB.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
			table,
		).Scan(&count); err != nil {
			t.Fatalf("check table %s: %v", table, err)
		}
		if got := count == 1; got != want {
			t.Fatalf("table %s exists = %t, want %t", table, got, want)
		}
	}
}
