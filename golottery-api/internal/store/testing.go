package store

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"gorm.io/gorm"
)

// DefaultTestURL is the local convention for the shared test database.
const DefaultTestURL = "postgres://postgres:secret@127.0.0.1:15436/golottery_test?sslmode=disable"

// OpenTest opens the shared PostgreSQL test database.
// DATABASE_URL overrides the default URL when set.
func OpenTest(tb testing.TB) *DB {
	tb.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = DefaultTestURL
	}
	db, err := Open(context.Background(), Config{URL: url})
	if err != nil {
		tb.Fatalf("store.OpenTest() error = %v (need a reachable PostgreSQL; default %s)", err, DefaultTestURL)
	}
	tb.Cleanup(func() { _ = db.Close() })
	return db
}

// Reset applies SQL migrations and truncates the listed model tables.
func Reset(tb testing.TB, db *DB, models ...any) {
	tb.Helper()
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		tb.Fatalf("store.Reset migrate: %v", err)
	}
	if len(models) == 0 {
		return
	}
	tables := make([]string, 0, len(models))
	for _, model := range models {
		stmt := &gorm.Statement{DB: db.Gorm}
		if err := stmt.Parse(model); err != nil {
			tb.Fatalf("store.Reset parse: %v", err)
		}
		tables = append(tables, stmt.Schema.Table)
	}
	sql := fmt.Sprintf("TRUNCATE %s RESTART IDENTITY CASCADE", quoteTables(tables))
	if err := db.Gorm.WithContext(ctx).Exec(sql).Error; err != nil {
		tb.Fatalf("store.Reset truncate: %v", err)
	}
}

func quoteTables(tables []string) string {
	quoted := make([]string, len(tables))
	for i, table := range tables {
		quoted[i] = `"` + strings.ReplaceAll(table, `"`, `""`) + `"`
	}
	return strings.Join(quoted, ", ")
}
