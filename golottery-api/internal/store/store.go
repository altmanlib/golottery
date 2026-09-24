// Package store owns the database handle: connection opening, pool sizing
// and schema migration for PostgreSQL.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config configures the PostgreSQL database handle.
type Config struct {
	URL    string
	Logger *slog.Logger
}

// DB wraps the GORM handle together with the underlying *sql.DB.
type DB struct {
	Gorm *gorm.DB
	sql  *sql.DB
}

// Open opens GORM over PostgreSQL and sizes the connection pool.
func Open(ctx context.Context, cfg Config) (*DB, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("store: DATABASE_URL is required")
	}

	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}

	gormDB, err := gorm.Open(postgres.Open(cfg.URL), &gorm.Config{
		Logger: logger.NewSlogLogger(log, logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("store: sql db: %w", err)
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)

	db := &DB{Gorm: gormDB, sql: sqlDB}
	if err := db.Ping(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// Ping verifies the database connection is usable.
func (d *DB) Ping(ctx context.Context) error {
	if err := d.sql.PingContext(ctx); err != nil {
		return fmt.Errorf("store: ping: %w", err)
	}
	return nil
}

// Close releases the underlying database handle.
func (d *DB) Close() error {
	if d == nil || d.sql == nil {
		return nil
	}
	if err := d.sql.Close(); err != nil {
		return fmt.Errorf("store: close: %w", err)
	}
	return nil
}
