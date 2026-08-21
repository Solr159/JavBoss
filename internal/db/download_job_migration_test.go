package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestDownloadJobSchemaAllowsDiscoveryAndNoSource(t *testing.T) {
	driverName := registerSQLiteFunctions()
	sqlDB, err := sql.Open(driverName, filepath.Join(t.TempDir(), "download-job-schema.db"))
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
	if err := execDB(ctx, sqlDB, "PRAGMA foreign_keys=ON;"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	if err := goose.UpToContext(ctx, sqlDB, migrationDir, javDiscoveryMigrationVersion); err != nil {
		t.Fatalf("migrate to discovery and download schema: %v", err)
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
		INSERT INTO download_job (
			source_type, source_id, code, directory_id, provider, info_hash, magnet_url
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"discovery", itemID, "ABC-001", directoryID, "openlist", "discovery-hash",
		"magnet:?xt=urn:btih:discovery-hash",
	); err != nil {
		t.Fatalf("insert discovery download job: %v", err)
	}

	var sourceType, code string
	var sourceID int64
	if err := sqlDB.QueryRowContext(ctx, `
		SELECT source_type, source_id, code
		FROM download_job
		WHERE info_hash = ?`, "discovery-hash").Scan(&sourceType, &sourceID, &code); err != nil {
		t.Fatalf("read discovery download job: %v", err)
	}
	if sourceType != "discovery" || sourceID != itemID || code != "ABC-001" {
		t.Fatalf("discovery source = (%q, %d, %q)", sourceType, sourceID, code)
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
