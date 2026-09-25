package redisx

import (
	"context"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"
)

// DefaultTestURL is the local convention for tests: db 15 of the compose Redis.
const DefaultTestURL = "redis://127.0.0.1:57379/15"

// OpenTest connects to the test database and empties it. REDIS_TEST_URL overrides the URL.
func OpenTest(tb testing.TB) *redis.Client {
	tb.Helper()
	url := os.Getenv("REDIS_TEST_URL")
	if url == "" {
		url = DefaultTestURL
	}
	client, err := Open(context.Background(), url)
	if err != nil {
		tb.Fatalf("redisx.OpenTest() error = %v (need a reachable Redis; default %s)", err, DefaultTestURL)
	}
	if err := client.FlushDB(context.Background()).Err(); err != nil {
		tb.Fatalf("redisx.OpenTest() flush: %v", err)
	}
	tb.Cleanup(func() { _ = client.Close() })
	return client
}

// Unreachable returns a client pointing at a closed port, for testing degradation.
func Unreachable() *redis.Client {
	return redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", DialTimeout: 100e6, MaxRetries: -1})
}
