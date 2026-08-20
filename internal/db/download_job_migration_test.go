package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
)

const downloadJobMigrationVersion = int64(202608210001)

func TestDownloadJobMigrationPreservesDiscoveryJobsAndAllowsNoSource(t *testing.T) {
	driverName := registerSQLiteFunctions()
	sqlDB, err := sql.Open(driverName, filepath.Join(t.TempDir(), "download-job-upgrade.db"))
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
	if err := goose.UpToContext(ctx, sqlDB, migrationDir, javDiscoveryMigrationVersion); err != nil {
		t.Fatalf("migrate to discovery schema: %v", err)
	}
	directoryResult, err := sqlDB.ExecContext(ctx, `INSERT INTO directory (path) VALUES (?)`, t.TempDir())
	if err != nil {
		t.Fatalf("insert directory: %v", err)
	}
	directoryID, err := directoryResult.LastInsertId()
	if err != nil {
		t.Fatalf("read directory id: %v", err)
	}
	itemResult, err := sqlDB.ExecContext(ctx, `INSERT INTO jav_discovery_item (code) VALUES (?)`, "ABC-001")
	if err != nil {
		t.Fatalf("insert discovery item: %v", err)
	}
	itemID, err := itemResult.LastInsertId()
	if err != nil {
		t.Fatalf("read discovery item id: %v", err)
	}
	if _, err := sqlDB.ExecContext(ctx, `
		INSERT INTO jav_discovery_download (
			jav_discovery_item_id, directory_id, provider, info_hash, magnet_url
		) VALUES (?, ?, ?, ?, ?)`,
		itemID, directoryID, "openlist", "legacy-hash", "magnet:?xt=urn:btih:legacy-hash",
	); err != nil {
		t.Fatalf("insert legacy discovery download: %v", err)
	}
	duplicateItemResult, err := sqlDB.ExecContext(ctx, `INSERT INTO jav_discovery_item (code) VALUES (?)`, "DUP-001")
	if err != nil {
		t.Fatalf("insert duplicate discovery item: %v", err)
	}
	duplicateItemID, err := duplicateItemResult.LastInsertId()
	if err != nil {
		t.Fatalf("read duplicate discovery item id: %v", err)
	}
	if _, err := sqlDB.ExecContext(ctx, `
		INSERT INTO jav_discovery_download (
			jav_discovery_item_id, directory_id, provider, info_hash, magnet_url, status
		) VALUES (?, ?, ?, ?, ?, ?)`,
		duplicateItemID, directoryID, "openlist", "legacy-hash",
		"magnet:?xt=urn:btih:legacy-hash", "completed",
	); err != nil {
		t.Fatalf("insert duplicate legacy discovery download: %v", err)
	}

	if err := goose.UpToContext(ctx, sqlDB, migrationDir, downloadJobMigrationVersion); err != nil {
		t.Fatalf("apply download job migration: %v", err)
	}
	assertMigrationVersion(t, sqlDB, downloadJobMigrationVersion)
	assertTablesExist(t, sqlDB, false, "jav_discovery_download")
	assertTablesExist(t, sqlDB, true, "download_job")

	var sourceType, code string
	var sourceID int64
	if err := sqlDB.QueryRowContext(ctx, `
		SELECT source_type, source_id, code
		FROM download_job
		WHERE info_hash = ?`, "legacy-hash").Scan(&sourceType, &sourceID, &code); err != nil {
		t.Fatalf("read migrated download job: %v", err)
	}
	if sourceType != "discovery" || sourceID != itemID || code != "ABC-001" {
		t.Fatalf("migrated source = (%q, %d, %q)", sourceType, sourceID, code)
	}
	var migratedCount int
	if err := sqlDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM download_job
		WHERE directory_id = ? AND info_hash = ?`, directoryID, "legacy-hash").Scan(&migratedCount); err != nil {
		t.Fatalf("count migrated duplicate downloads: %v", err)
	}
	if migratedCount != 1 {
		t.Fatalf("migrated duplicate count = %d, want 1", migratedCount)
	}

	if _, err := sqlDB.ExecContext(ctx, `
		INSERT INTO download_job (
			source_type, source_id, code, directory_id, provider, info_hash, magnet_url
		) VALUES (NULL, NULL, ?, ?, ?, ?, ?)`,
		"NO-SOURCE-001", directoryID, "openlist", "source-free-hash",
		"magnet:?xt=urn:btih:source-free-hash",
	); err != nil {
		t.Fatalf("insert source-free download job: %v", err)
	}
	var nullSourceType sql.NullString
	var nullSourceID sql.NullInt64
	if err := sqlDB.QueryRowContext(ctx, `
		SELECT source_type, source_id FROM download_job WHERE info_hash = ?`, "source-free-hash").Scan(&nullSourceType, &nullSourceID); err != nil {
		t.Fatalf("read source-free download job: %v", err)
	}
	if nullSourceType.Valid || nullSourceID.Valid {
		t.Fatalf("source-free source = (%q, %d), want (NULL, NULL)", nullSourceType.String, nullSourceID.Int64)
	}
}
