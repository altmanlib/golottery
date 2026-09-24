package settings

import (
	"context"
	"testing"

	"golottery/api/internal/store"
)

func TestSetListUnset(t *testing.T) {
	db := store.OpenTest(t)
	store.Reset(t, db, &Setting{})
	ctx := context.Background()
	s := NewStore(db.Gorm)

	if err := s.Set(ctx, "LOGIN_WINDOW", "10m", "auth", "test"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := s.Set(ctx, "LOGIN_WINDOW", "20m", "auth", "test"); err != nil {
		t.Fatalf("Set() upsert error = %v", err)
	}
	snap, err := s.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if snap["LOGIN_WINDOW"] != "20m" {
		t.Fatalf("snapshot = %v", snap)
	}
	if err := s.Unset(ctx, "LOGIN_WINDOW"); err != nil {
		t.Fatalf("Unset() error = %v", err)
	}
	snap, err = s.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if _, ok := snap["LOGIN_WINDOW"]; ok {
		t.Fatalf("key still present: %v", snap)
	}
}

func TestSetRejectsEmpty(t *testing.T) {
	db := store.OpenTest(t)
	store.Reset(t, db, &Setting{})
	if err := NewStore(db.Gorm).Set(context.Background(), "LOGIN_WINDOW", "", "auth", "test"); err == nil {
		t.Fatal("empty value accepted")
	}
}
