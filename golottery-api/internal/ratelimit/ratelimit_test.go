package ratelimit

import (
	"context"
	"testing"
	"time"

	"golottery/api/internal/redisx"
)

func TestFixedWindow(t *testing.T) {
	rdb := redisx.OpenTest(t)
	ctx := context.Background()
	a, b := New(rdb, nil), New(rdb, nil)

	for i := range 10 {
		limiter := a
		if i%2 == 1 {
			limiter = b // two instances share one count
		}
		if !limiter.Allow(ctx, "checkin:e:o", 10, time.Second) {
			t.Fatalf("request %d refused", i+1)
		}
	}
	if a.Allow(ctx, "checkin:e:o", 10, time.Second) {
		t.Fatal("11th request allowed")
	}
	if !a.Allow(ctx, "checkin:e:other", 10, time.Second) {
		t.Fatal("another key was limited")
	}
	if !a.Exceeded(ctx, "checkin:e:o", 10) || a.Exceeded(ctx, "checkin:e:other", 10) {
		t.Fatal("Exceeded disagrees with Allow")
	}
	time.Sleep(1100 * time.Millisecond)
	if !a.Allow(ctx, "checkin:e:o", 10, time.Second) {
		t.Fatal("window did not reset")
	}
}

func TestRedisDownAllows(t *testing.T) {
	limiter := New(redisx.Unreachable(), nil)
	for range 3 {
		if !limiter.Allow(context.Background(), "k", 1, time.Minute) {
			t.Fatal("refused while Redis is down")
		}
	}
	if limiter.Exceeded(context.Background(), "k", 1) {
		t.Fatal("Exceeded while Redis is down")
	}
}
