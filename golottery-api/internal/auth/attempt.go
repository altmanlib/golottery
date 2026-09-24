package auth

import "time"

// LoginAttempt records one failed login. Rate-limit decisions are not made here.
type LoginAttempt struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"`
	Key       string    `gorm:"size:128;index:idx_login_attempts_key_created,priority:1;not null"`
	CreatedAt time.Time `gorm:"index:idx_login_attempts_key_created,priority:2;not null"`
}

// TableName returns the login_attempts table name.
func (LoginAttempt) TableName() string { return "login_attempts" }
