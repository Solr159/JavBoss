package db

import (
	"context"
	"database/sql"
	"fmt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// Open initialises a GORM-backed SQLite database and applies goose migrations.
func Open(path string) (*gorm.DB, error) {
	driverName := registerSQLiteFunctions()
	sqlDB, err := sql.Open(driverName, path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	closeOnErr := true
	defer func() {
		if closeOnErr {
			_ = sqlDB.Close()
		}
	}()

	if err := execDB(context.Background(), sqlDB, "PRAGMA journal_mode=WAL;"); err != nil {
		return nil, fmt.Errorf("enable WAL: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := execDB(context.Background(), sqlDB, "PRAGMA foreign_keys=OFF;"); err != nil {
		return nil, fmt.Errorf("disable foreign keys for migration: %w", err)
	}
	if err := runMigrations(context.Background(), sqlDB); err != nil {
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	if err := execDB(context.Background(), sqlDB, "PRAGMA foreign_keys=ON;"); err != nil {
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	sqlDB.SetMaxOpenConns(0)

	db, err := gorm.Open(sqlite.New(sqlite.Config{Conn: sqlDB}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("open gorm database: %w", err)
	}
	closeOnErr = false
	return db, nil
}
