package auth

import (
	"context"
	"testing"
	"time"

	"golottery/api/internal/store"
)

func TestLoginLimiter(t *testing.T) {
	db := store.OpenTest(t)
	store.Reset(t, db, &LoginAttempt{})
	ctx := context.Background()
	limiter := NewLoginLimiter(db.Gorm, 3, 10*time.Minute)
	now := time.Now()
	limiter.now = func() time.Time { return now }

	for i := range 2 {
		if err := limiter.Fail(ctx, "platform:ops"); err != nil {
			t.Fatalf("Fail() error = %v", err)
		}
		if wait, err := limiter.Check(ctx, "platform:ops"); err != nil || wait != 0 {
			t.Fatalf("after %d failures Check() = %d, %v; want 0", i+1, wait, err)
		}
	}

	if err := limiter.Fail(ctx, "platform:ops"); err != nil {
		t.Fatalf("Fail() error = %v", err)
	}
	if wait, err := limiter.Check(ctx, "platform:ops"); err != nil || wait != 10 {
		t.Fatalf("at threshold Check() = %d, %v; want 10", wait, err)
	}
	if wait, _ := limiter.Check(ctx, "platform:other"); wait != 0 {
		t.Fatalf("other key Check() = %d, want 0", wait)
	}

	now = now.Add(9*time.Minute + 30*time.Second)
	if wait, _ := limiter.Check(ctx, "platform:ops"); wait != 1 {
		t.Fatalf("near window end Check() = %d, want at least 1", wait)
	}
	now = now.Add(time.Minute)
	if wait, _ := limiter.Check(ctx, "platform:ops"); wait != 0 {
		t.Fatalf("after window Check() = %d, want 0", wait)
	}

	now = now.Add(-time.Minute)
	if err := limiter.Reset(ctx, "platform:ops"); err != nil {
		t.Fatalf("Reset() error = %v", err)
	}
	if wait, _ := limiter.Check(ctx, "platform:ops"); wait != 0 {
		t.Fatalf("after Reset Check() = %d, want 0", wait)
	}
}
