package platform

import (
	"context"
	"strings"
	"testing"

	"golottery/api/internal/auth"
	"golottery/api/internal/store"
)

func TestSeed(t *testing.T) {
	db := store.OpenTest(t)
	store.Reset(t, db, &User{})
	ctx := context.Background()

	if _, err := Seed(ctx, db.Gorm, "", ""); err == nil || !strings.Contains(err.Error(), "PLATFORM_USER") {
		t.Fatalf("Seed() without config error = %v, want missing PLATFORM_USER", err)
	}
	if _, err := Seed(ctx, db.Gorm, "ops", "not-a-hash"); err == nil {
		t.Fatal("Seed() accepted an invalid hash")
	}

	first, err := auth.HashPassword("first-password")
	if err != nil {
		t.Fatal(err)
	}
	seeded, err := Seed(ctx, db.Gorm, "ops", first)
	if err != nil || !seeded {
		t.Fatalf("Seed() = %v, %v; want seeded", seeded, err)
	}

	second, err := auth.HashPassword("second-password")
	if err != nil {
		t.Fatal(err)
	}
	seeded, err = Seed(ctx, db.Gorm, "ops", second)
	if err != nil || seeded {
		t.Fatalf("second Seed() = %v, %v; want no-op", seeded, err)
	}
	if seeded, err = Seed(ctx, db.Gorm, "", ""); err != nil || seeded {
		t.Fatalf("Seed() on non-empty table without config = %v, %v; want no-op", seeded, err)
	}

	var user User
	if err := db.Gorm.Where("username = ?", "ops").Take(&user).Error; err != nil {
		t.Fatal(err)
	}
	if ok, _ := auth.VerifyPassword(user.PasswordHash, "first-password"); !ok {
		t.Fatal("existing password was overwritten")
	}
}
