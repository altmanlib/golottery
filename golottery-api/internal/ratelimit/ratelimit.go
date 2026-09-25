// Package ratelimit counts requests in fixed windows kept in Redis, shared by every
// instance. When Redis fails the limiter lets requests through and logs the error.
package ratelimit

import (
	"context"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"golottery/api/internal/redisx"
)

// hit increments the counter and starts its window on the first hit, atomically.
var hit = redis.NewScript(`
local n = redis.call("INCR", KEYS[1])
if n == 1 then redis.call("PEXPIRE", KEYS[1], ARGV[1]) end
return n`)

// Limiter is safe for concurrent use.
type Limiter struct {
	rdb    *redis.Client
	logger *slog.Logger
}

// New builds a Limiter.
func New(rdb *redis.Client, logger *slog.Logger) *Limiter {
	if logger == nil {
		logger = slog.Default()
	}
	return &Limiter{rdb: rdb, logger: logger}
}

func fullKey(key string) string { return redisx.KeyPrefix + "rl:" + key }

// Allow counts one request under key and reports whether it is within limit for window.
func (l *Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration) bool {
	n, err := hit.Run(ctx, l.rdb, []string{fullKey(key)}, window.Milliseconds()).Int()
	if err != nil {
		l.logger.Error("rate limit unavailable, allowing request", "key", key, "error", err)
		return true
	}
	return n <= limit
}

// Exceeded reports whether key has already reached limit, without counting a request.
func (l *Limiter) Exceeded(ctx context.Context, key string, limit int) bool {
	n, err := l.rdb.Get(ctx, fullKey(key)).Int()
	if err != nil {
		if err != redis.Nil {
			l.logger.Error("rate limit unavailable, allowing request", "key", key, "error", err)
		}
		return false
	}
	return n >= limit
}
