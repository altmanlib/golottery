// Package redisx wires the shared Redis client. Redis holds only short-lived state
// that can be rebuilt: rate-limit counters, SSE fan-out and the WeChat access token.
package redisx

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// KeyPrefix starts every key this service writes.
const KeyPrefix = "gl:"

// Open parses url, connects and pings once so a bad address fails at startup.
func Open(ctx context.Context, url string) (*redis.Client, error) {
	if url == "" {
		return nil, fmt.Errorf("redisx: REDIS_URL is required")
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("redisx: parse REDIS_URL: %w", err)
	}
	client := redis.NewClient(opts)
	if err := Ping(ctx, client); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

// Ping checks the connection with a short timeout.
func Ping(ctx context.Context, client *redis.Client) error {
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		return fmt.Errorf("redisx: ping: %w", err)
	}
	return nil
}
