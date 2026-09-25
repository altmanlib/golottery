// Package org owns organizations, their quota and the credit ledger.
package org

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"golottery/api/internal/bizerr"
)

const (
	// StatusActive organizations can sign in and manage events.
	StatusActive = "active"
	// StatusDisabled organizations keep their data but cannot manage events.
	StatusDisabled = "disabled"

	// EventReadyReason is the ledger reason for the credit an event uses when it first becomes ready.
	EventReadyReason = "event ready"

	// LedgerPreview is how many recent entries an organization detail shows.
	LedgerPreview = 20

	maxNameLen    = 100
	maxContactLen = 100
	maxReasonLen  = 200
)

// Org is one tenant.
type Org struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name      string    `gorm:"size:100;not null"`
	Contact   string    `gorm:"size:100;not null"`
	Status    string    `gorm:"size:16;not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

// TableName returns the orgs table name.
func (Org) TableName() string { return "orgs" }

// Quota is the single quota row of an organization.
type Quota struct {
	OrgID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	EventCredits int       `gorm:"not null"`
	MaxAttendees int       `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`
}

// TableName returns the org_quotas table name.
func (Quota) TableName() string { return "org_quotas" }

// LedgerEntry explains one change of event_credits. Rows are never updated or deleted.
type LedgerEntry struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID        uuid.UUID  `gorm:"type:uuid;not null"`
	Delta        int        `gorm:"not null"`
	BalanceAfter int        `gorm:"not null"`
	Reason       string     `gorm:"size:200;not null"`
	EventID      *uuid.UUID `gorm:"type:uuid"`
	OperatorType string     `gorm:"size:32;not null"`
	OperatorID   string     `gorm:"size:64;not null"`
	CreatedAt    time.Time  `gorm:"not null"`
}

// TableName returns the credit_ledger table name.
func (LedgerEntry) TableName() string { return "credit_ledger" }

// Operator identifies who changed a quota: the principal of the calling token.
type Operator struct {
	Type string
	ID   string
}

// Summary is an organization with its quota.
type Summary struct {
	Org
	EventCredits int
	MaxAttendees int
}

// Detail is an organization, its quota and its latest ledger entries.
type Detail struct {
	Summary
	Ledger []LedgerEntry
}

// CreateInput opens an organization.
type CreateInput struct {
	Name         string
	Contact      string
	EventCredits int
	MaxAttendees int
}

// Service implements the platform operations on organizations.
type Service struct {
	db  *gorm.DB
	now func() time.Time
}

// NewService builds a Service.
func NewService(db *gorm.DB) *Service {
	return &Service{db: db, now: time.Now}
}

// Create inserts the organization, its quota and, for a non-zero start, the first ledger entry in one transaction.
func (s *Service) Create(ctx context.Context, in CreateInput, by Operator) (Summary, error) {
	name := strings.TrimSpace(in.Name)
	contact := strings.TrimSpace(in.Contact)
	if name == "" {
		return Summary{}, bizerr.New(bizerr.CodeNameRequired)
	}
	if utf8.RuneCountInString(name) > maxNameLen || utf8.RuneCountInString(contact) > maxContactLen ||
		in.EventCredits < 0 || in.MaxAttendees <= 0 {
		return Summary{}, bizerr.New(bizerr.CodeBadRequest)
	}

	now := s.now().UTC()
	org := Org{ID: uuid.New(), Name: name, Contact: contact, Status: StatusActive, CreatedAt: now, UpdatedAt: now}
	quota := Quota{OrgID: org.ID, EventCredits: in.EventCredits, MaxAttendees: in.MaxAttendees, UpdatedAt: now}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&org).Error; err != nil {
			return fmt.Errorf("org: insert org: %w", err)
		}
		if err := tx.Create(&quota).Error; err != nil {
			return fmt.Errorf("org: insert quota: %w", err)
		}
		if in.EventCredits == 0 {
			return nil
		}
		entry := newEntry(org.ID, in.EventCredits, in.EventCredits, "开通组织", by, now)
		if err := tx.Create(&entry).Error; err != nil {
			return fmt.Errorf("org: insert ledger: %w", err)
		}
		return nil
	})
	if err != nil {
		return Summary{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return Summary{Org: org, EventCredits: quota.EventCredits, MaxAttendees: quota.MaxAttendees}, nil
}

// List returns one page of organizations, newest first, and the total count.
func (s *Service) List(ctx context.Context, offset, limit int) ([]Summary, int64, error) {
	var total int64
	if err := s.db.WithContext(ctx).Model(&Org{}).Count(&total).Error; err != nil {
		return nil, 0, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	var rows []Summary
	err := s.summaries(ctx).
		Order("orgs.created_at DESC, orgs.id").
		Offset(offset).Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return rows, total, nil
}

// Get returns an organization with its latest ledger entries.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (Detail, error) {
	summary, err := s.summary(ctx, id)
	if err != nil {
		return Detail{}, err
	}
	var ledger []LedgerEntry
	err = s.db.WithContext(ctx).Where("org_id = ?", id).
		Order("created_at DESC, id").Limit(LedgerPreview).
		Find(&ledger).Error
	if err != nil {
		return Detail{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return Detail{Summary: summary, Ledger: ledger}, nil
}

// SetStatus moves an organization to status. Setting the current status again is a no-op.
func (s *Service) SetStatus(ctx context.Context, id uuid.UUID, status string) error {
	res := s.db.WithContext(ctx).Model(&Org{}).
		Where("id = ? AND status <> ?", id, status).
		Updates(map[string]any{"status": status, "updated_at": s.now().UTC()})
	if res.Error != nil {
		return bizerr.Wrap(bizerr.CodeInternal, res.Error)
	}
	if res.RowsAffected == 0 {
		return s.mustExist(ctx, id)
	}
	return nil
}

// AdjustCredits changes event_credits by delta and records why.
// The balance never goes below zero, even under concurrent adjustments.
func (s *Service) AdjustCredits(ctx context.Context, id uuid.UUID, delta int, reason string, by Operator) (LedgerEntry, error) {
	reason = strings.TrimSpace(reason)
	if delta == 0 || reason == "" || utf8.RuneCountInString(reason) > maxReasonLen {
		return LedgerEntry{}, bizerr.New(bizerr.CodeBadRequest)
	}

	now := s.now().UTC()
	var entry LedgerEntry
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// A single conditional UPDATE is atomic, so two deductions cannot both pass the check.
		var balances []int
		err := tx.Raw(`UPDATE org_quotas SET event_credits = event_credits + ?, updated_at = ?
			WHERE org_id = ? AND event_credits + ? >= 0 RETURNING event_credits`,
			delta, now, id, delta).Scan(&balances).Error
		if err != nil {
			return fmt.Errorf("org: update credits: %w", err)
		}
		if len(balances) == 0 {
			if err := s.mustExist(ctx, id); err != nil {
				return err
			}
			return bizerr.New(bizerr.CodeConflict)
		}
		entry = newEntry(id, delta, balances[0], reason, by, now)
		if err := tx.Create(&entry).Error; err != nil {
			return fmt.Errorf("org: insert ledger: %w", err)
		}
		return nil
	})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return LedgerEntry{}, err
		}
		return LedgerEntry{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return entry, nil
}

// SetMaxAttendees changes the default limit copied into events created afterwards.
func (s *Service) SetMaxAttendees(ctx context.Context, id uuid.UUID, maxAttendees int) error {
	if maxAttendees <= 0 {
		return bizerr.New(bizerr.CodeBadRequest)
	}
	res := s.db.WithContext(ctx).Model(&Quota{}).Where("org_id = ?", id).
		Updates(map[string]any{"max_attendees": maxAttendees, "updated_at": s.now().UTC()})
	if res.Error != nil {
		return bizerr.Wrap(bizerr.CodeInternal, res.Error)
	}
	if res.RowsAffected == 0 {
		return bizerr.New(bizerr.CodeNotFound)
	}
	return nil
}

func (s *Service) summaries(ctx context.Context) *gorm.DB {
	return s.db.WithContext(ctx).Table("orgs").
		Select("orgs.*, org_quotas.event_credits, org_quotas.max_attendees").
		Joins("JOIN org_quotas ON org_quotas.org_id = orgs.id")
}

func (s *Service) summary(ctx context.Context, id uuid.UUID) (Summary, error) {
	var rows []Summary
	if err := s.summaries(ctx).Where("orgs.id = ?", id).Limit(1).Scan(&rows).Error; err != nil {
		return Summary{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	if len(rows) == 0 {
		return Summary{}, bizerr.New(bizerr.CodeNotFound)
	}
	return rows[0], nil
}

func (s *Service) mustExist(ctx context.Context, id uuid.UUID) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&Org{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return bizerr.Wrap(bizerr.CodeInternal, err)
	}
	if count == 0 {
		return bizerr.New(bizerr.CodeNotFound)
	}
	return nil
}

func newEntry(orgID uuid.UUID, delta, balance int, reason string, by Operator, at time.Time) LedgerEntry {
	return LedgerEntry{
		ID:           uuid.New(),
		OrgID:        orgID,
		Delta:        delta,
		BalanceAfter: balance,
		Reason:       reason,
		OperatorType: by.Type,
		OperatorID:   by.ID,
		CreatedAt:    at,
	}
}

// ConsumeEventCredit takes one credit for an event's first move to ready, inside tx.
// The balance check and decrement are one statement, so concurrent events cannot overdraw.
func ConsumeEventCredit(ctx context.Context, tx *gorm.DB, orgID, eventID uuid.UUID, by Operator, at time.Time) error {
	var balances []int
	err := tx.WithContext(ctx).Raw(`UPDATE org_quotas SET event_credits = event_credits - 1, updated_at = ?
		WHERE org_id = ? AND event_credits > 0 RETURNING event_credits`, at, orgID).Scan(&balances).Error
	if err != nil {
		return bizerr.Wrap(bizerr.CodeInternal, fmt.Errorf("org: consume credit: %w", err))
	}
	if len(balances) == 0 {
		return bizerr.New(bizerr.CodeNoEventCredits)
	}
	entry := newEntry(orgID, -1, balances[0], EventReadyReason, by, at)
	entry.EventID = &eventID
	if err := tx.WithContext(ctx).Create(&entry).Error; err != nil {
		return bizerr.Wrap(bizerr.CodeInternal, fmt.Errorf("org: insert ledger: %w", err))
	}
	return nil
}

// Credits returns the remaining event credits of an organization.
func Credits(ctx context.Context, db *gorm.DB, orgID uuid.UUID) (int, error) {
	var quota Quota
	if err := db.WithContext(ctx).Where("org_id = ?", orgID).Take(&quota).Error; err != nil {
		return 0, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return quota.EventCredits, nil
}
