package store

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"golottery/api/internal/store/migrations"
)

// migrationLockID serializes concurrent migrators on one cluster.
const migrationLockID int64 = 82026092401

// Migrate applies every embedded SQL migration that is not yet recorded.
func (d *DB) Migrate(ctx context.Context) error {
	sqlDB := d.sql
	if _, err := sqlDB.ExecContext(ctx, "SELECT pg_advisory_lock($1)", migrationLockID); err != nil {
		return fmt.Errorf("store: lock migrations: %w", err)
	}
	defer func() {
		_, _ = sqlDB.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLockID)
	}()

	entries, err := migrations.Files.ReadDir(".")
	if err != nil {
		return fmt.Errorf("store: list migrations: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".sql") {
			continue
		}
		versionText := strings.SplitN(name, "_", 2)[0]
		version, err := strconv.Atoi(versionText)
		if err != nil {
			return fmt.Errorf("store: invalid migration name %q", name)
		}

		applied, err := d.migrationApplied(ctx, version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		body, err := migrations.Files.ReadFile(name)
		if err != nil {
			return fmt.Errorf("store: read migration %s: %w", name, err)
		}

		tx, err := sqlDB.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("store: begin migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("store: apply migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2)`,
			version, time.Now().UTC(),
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("store: record migration %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("store: commit migration %s: %w", name, err)
		}
	}
	return nil
}

func (d *DB) migrationApplied(ctx context.Context, version int) (bool, error) {
	var tableExists bool
	if err := d.sql.QueryRowContext(ctx,
		`SELECT to_regclass('public.schema_migrations') IS NOT NULL`,
	).Scan(&tableExists); err != nil {
		return false, fmt.Errorf("store: check schema_migrations: %w", err)
	}
	if !tableExists {
		return false, nil
	}
	var applied bool
	if err := d.sql.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, version,
	).Scan(&applied); err != nil {
		return false, fmt.Errorf("store: check migration %d: %w", version, err)
	}
	return applied, nil
}
