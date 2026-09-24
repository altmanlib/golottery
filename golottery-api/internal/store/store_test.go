package store

import (
	"context"
	"testing"

	"golottery/api/internal/settings"
)

func TestOpenPingMigrateReset(t *testing.T) {
	db := OpenTest(t)
	ctx := context.Background()
	if err := db.Ping(ctx); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
	Reset(t, db, &settings.Setting{})
	if err := db.Gorm.WithContext(ctx).Create(&settings.Setting{
		Key:   "LOGIN_WINDOW",
		Value: "10m",
	}).Error; err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	Reset(t, db, &settings.Setting{})
	var count int64
	if err := db.Gorm.WithContext(ctx).Model(&settings.Setting{}).Count(&count).Error; err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0 after Reset", count)
	}
}

func TestMigrateIdempotent(t *testing.T) {
	db := OpenTest(t)
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("first Migrate() error = %v", err)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}
}
