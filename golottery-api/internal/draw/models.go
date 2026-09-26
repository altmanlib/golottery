// Package draw owns host credentials, lottery draws and the on-site SSE hub.
package draw

import (
	"time"

	"github.com/google/uuid"
)

// Result statuses.
const (
	StatusValid = "valid"
	StatusVoid  = "void"
)

// Host is the single host credential of one event.
type Host struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID        uuid.UUID `gorm:"type:uuid;not null"`
	EventID      uuid.UUID `gorm:"type:uuid;not null"`
	PasswordHash string    `gorm:"not null"`
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`
}

// TableName returns the host_users table name.
func (Host) TableName() string { return "host_users" }

// Result is one draw outcome. Voided rows stay; they are never deleted.
type Result struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID      uuid.UUID `gorm:"type:uuid;not null"`
	EventID    uuid.UUID `gorm:"type:uuid;not null"`
	PrizeID    uuid.UUID `gorm:"type:uuid;not null"`
	AttendeeID uuid.UUID `gorm:"type:uuid;not null"`
	RequestID  uuid.UUID `gorm:"type:uuid;not null"`
	Status     string    `gorm:"size:16;not null"`
	Exclusive  bool      `gorm:"not null"`
	VoidReason *string   `gorm:"size:200"`
	VoidedAt   *time.Time
	CreatedAt  time.Time `gorm:"not null"`
}

// TableName returns the draw_results table name.
func (Result) TableName() string { return "draw_results" }

// Log is one draw attempt, keyed by request_id for idempotency.
type Log struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID      uuid.UUID `gorm:"type:uuid;not null"`
	EventID    uuid.UUID `gorm:"type:uuid;not null"`
	PrizeID    uuid.UUID `gorm:"type:uuid;not null"`
	PoolSize   int       `gorm:"not null"`
	DrawCount  int       `gorm:"not null"`
	OperatorID uuid.UUID `gorm:"type:uuid;not null"`
	RequestID  uuid.UUID `gorm:"type:uuid;not null"`
	CreatedAt  time.Time `gorm:"not null"`
}

// TableName returns the draw_logs table name.
func (Log) TableName() string { return "draw_logs" }
