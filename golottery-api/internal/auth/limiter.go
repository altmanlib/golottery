package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// LoginLimiter decides whether a login key has failed too often within the window.
type LoginLimiter struct {
	db          *gorm.DB
	maxFailures int
	window      time.Duration
	now         func() time.Time
}

// NewLoginLimiter builds a limiter. Non-positive values fall back to 5 failures in 15 minutes.
func NewLoginLimiter(db *gorm.DB, maxFailures int, window time.Duration) *LoginLimiter {
	if maxFailures <= 0 {
		maxFailures = 5
	}
	if window <= 0 {
		window = 15 * time.Minute
	}
	return &LoginLimiter{db: db, maxFailures: maxFailures, window: window, now: time.Now}
}

// Check returns the whole minutes key must wait, or 0 when a login may be attempted.
func (l *LoginLimiter) Check(ctx context.Context, key string) (int, error) {
	now := l.now().UTC()
	// The maxFailures-th most recent failure inside the window unblocks the key once it ages out.
	var row LoginAttempt
	err := l.db.WithContext(ctx).
		Where("key = ? AND created_at > ?", key, now.Add(-l.window)).
		Order("created_at DESC").
		Offset(l.maxFailures - 1).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("auth: check login attempts: %w", err)
	}
	wait := row.CreatedAt.Add(l.window).Sub(now)
	minutes := int((wait + time.Minute - 1) / time.Minute)
	return max(minutes, 1), nil
}

// Fail records one failed login for key.
func (l *LoginLimiter) Fail(ctx context.Context, key string) error {
	row := LoginAttempt{Key: key, CreatedAt: l.now().UTC()}
	if err := l.db.WithContext(ctx).Create(&row).Error; err != nil {
		return fmt.Errorf("auth: record login attempt: %w", err)
	}
	return nil
}

// Reset clears the failures recorded for key.
func (l *LoginLimiter) Reset(ctx context.Context, key string) error {
	if err := l.db.WithContext(ctx).Where("key = ?", key).Delete(&LoginAttempt{}).Error; err != nil {
		return fmt.Errorf("auth: reset login attempts: %w", err)
	}
	return nil
}
