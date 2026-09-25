// Package event owns events, their roster and prizes, for organization admins.
package event

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"golottery/api/internal/bizerr"
	"golottery/api/internal/org"
)

// Event statuses.
const (
	StatusDraft  = "draft"
	StatusReady  = "ready"
	StatusClosed = "closed"
)

// Check-in modes.
const (
	ModeGeo    = "geo"
	ModeDirect = "direct"
)

const (
	// DefaultRadius is the geofence radius of a new event, in meters.
	DefaultRadius = 400
	minRadius     = 100
	maxRadius     = 1000

	maxNameLen = 100

	publicIDLen      = 21
	publicIDAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-"

	// EntryPage is the mini program page guests open; the event code travels as ?e= or scene.
	EntryPage = "pages/index/index"

	pgUniqueViolation = "23505"
)

// Event is one check-in and lottery occasion of an organization.
type Event struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID            uuid.UUID `gorm:"type:uuid;not null"`
	PublicID         string    `gorm:"size:21;not null"`
	Name             string    `gorm:"size:100;not null"`
	Status           string    `gorm:"size:16;not null"`
	CheckinMode      string    `gorm:"size:16;not null"`
	CenterLat        *float64  `gorm:"type:numeric(9,6)"`
	CenterLng        *float64  `gorm:"type:numeric(9,6)"`
	RadiusM          int       `gorm:"not null"`
	CheckinStart     *time.Time
	CheckinEnd       *time.Time
	AllowMultiWin    bool `gorm:"not null"`
	MaxAttendees     int  `gorm:"not null"`
	CreditConsumedAt *time.Time
	CreatedAt        time.Time `gorm:"not null"`
	UpdatedAt        time.Time `gorm:"not null"`
}

// TableName returns the events table name.
func (Event) TableName() string { return "events" }

// View is an event with the counts shown next to it.
type View struct {
	Event
	AttendeeCount int
	PrizeCount    int
}

// Patch lists the event fields to change; nil fields stay as they are.
type Patch struct {
	Name          *string
	CheckinMode   *string
	CenterLat     *float64
	CenterLng     *float64
	RadiusM       *int
	CheckinStart  *time.Time
	CheckinEnd    *time.Time
	AllowMultiWin *bool
	Status        *string
}

// Service implements event setup for organization admins.
type Service struct {
	db  *gorm.DB
	now func() time.Time
}

// NewService builds a Service.
func NewService(db *gorm.DB) *Service {
	return &Service{db: db, now: time.Now}
}

// EntryPath is the mini program path that opens an event.
func EntryPath(publicID string) string {
	return EntryPage + "?e=" + publicID
}

func newPublicID() (string, error) {
	buf := make([]byte, publicIDLen)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("event: random public id: %w", err)
	}
	for i, b := range buf {
		buf[i] = publicIDAlphabet[int(b)&63]
	}
	return string(buf), nil
}

func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", bizerr.New(bizerr.CodeNameRequired)
	}
	if utf8.RuneCountInString(name) > maxNameLen {
		return "", bizerr.New(bizerr.CodeBadRequest)
	}
	return name, nil
}

// Create adds a draft event. It copies the organization's default attendee limit and uses no credit.
func (s *Service) Create(ctx context.Context, orgID uuid.UUID, name string) (View, error) {
	name, err := cleanName(name)
	if err != nil {
		return View{}, err
	}
	var quota org.Quota
	if err := s.db.WithContext(ctx).Where("org_id = ?", orgID).Take(&quota).Error; err != nil {
		return View{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	publicID, err := newPublicID()
	if err != nil {
		return View{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	now := s.now().UTC()
	ev := Event{
		ID: uuid.New(), OrgID: orgID, PublicID: publicID, Name: name,
		Status: StatusDraft, CheckinMode: ModeGeo, RadiusM: DefaultRadius,
		MaxAttendees: quota.MaxAttendees, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.db.WithContext(ctx).Create(&ev).Error; err != nil {
		return View{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return View{Event: ev}, nil
}

// List returns one page of the organization's events, newest first.
func (s *Service) List(ctx context.Context, orgID uuid.UUID, offset, limit int) ([]View, int64, error) {
	var total int64
	if err := s.db.WithContext(ctx).Model(&Event{}).Where("org_id = ?", orgID).Count(&total).Error; err != nil {
		return nil, 0, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	var rows []View
	err := s.views(ctx).Where("events.org_id = ?", orgID).
		Order("events.created_at DESC, events.id").Offset(offset).Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return rows, total, nil
}

// Get returns one event of the organization.
func (s *Service) Get(ctx context.Context, orgID, id uuid.UUID) (View, error) {
	var rows []View
	if err := s.views(ctx).Where("events.org_id = ? AND events.id = ?", orgID, id).Limit(1).Scan(&rows).Error; err != nil {
		return View{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	if len(rows) == 0 {
		return View{}, bizerr.New(bizerr.CodeNotFound)
	}
	return rows[0], nil
}

// Update applies p and any status change in one transaction.
// The first move to ready uses one credit of the organization; later moves never refund or charge.
func (s *Service) Update(ctx context.Context, orgID, id uuid.UUID, p Patch, by org.Operator) (View, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ev, err := lockEvent(ctx, tx, orgID, id)
		if err != nil {
			return err
		}
		if ev.Status == StatusClosed {
			return bizerr.New(bizerr.CodeConflict)
		}
		from := ev.Status
		if err := applyPatch(&ev, p); err != nil {
			return err
		}
		to := from
		if p.Status != nil {
			to = *p.Status
		}
		if err := checkTransition(from, to); err != nil {
			return err
		}
		if to == StatusReady {
			if err := checkComplete(ctx, tx, ev); err != nil {
				return err
			}
		}
		now := s.now().UTC()
		if from == StatusDraft && to == StatusReady && ev.CreditConsumedAt == nil {
			if err := org.ConsumeEventCredit(ctx, tx, orgID, ev.ID, by, now); err != nil {
				return err
			}
			ev.CreditConsumedAt = &now
		}
		ev.Status = to
		ev.UpdatedAt = now
		if err := tx.Save(&ev).Error; err != nil {
			return fmt.Errorf("event: save: %w", err)
		}
		return nil
	})
	if err != nil {
		return View{}, asBizErr(err)
	}
	return s.Get(ctx, orgID, id)
}

func applyPatch(ev *Event, p Patch) error {
	if p.Name != nil {
		name, err := cleanName(*p.Name)
		if err != nil {
			return err
		}
		ev.Name = name
	}
	if p.CheckinMode != nil {
		if *p.CheckinMode != ModeGeo && *p.CheckinMode != ModeDirect {
			return bizerr.New(bizerr.CodeBadRequest)
		}
		ev.CheckinMode = *p.CheckinMode
	}
	if p.CenterLat != nil {
		if *p.CenterLat < -90 || *p.CenterLat > 90 {
			return bizerr.New(bizerr.CodeBadRequest)
		}
		ev.CenterLat = p.CenterLat
	}
	if p.CenterLng != nil {
		if *p.CenterLng < -180 || *p.CenterLng > 180 {
			return bizerr.New(bizerr.CodeBadRequest)
		}
		ev.CenterLng = p.CenterLng
	}
	if p.RadiusM != nil {
		if *p.RadiusM < minRadius || *p.RadiusM > maxRadius {
			return bizerr.New(bizerr.CodeBadRequest)
		}
		ev.RadiusM = *p.RadiusM
	}
	if p.CheckinStart != nil {
		start := p.CheckinStart.UTC()
		ev.CheckinStart = &start
	}
	if p.CheckinEnd != nil {
		end := p.CheckinEnd.UTC()
		ev.CheckinEnd = &end
	}
	if ev.CheckinStart != nil && ev.CheckinEnd != nil && !ev.CheckinEnd.After(*ev.CheckinStart) {
		return bizerr.New(bizerr.CodeBadRequest)
	}
	if p.AllowMultiWin != nil {
		ev.AllowMultiWin = *p.AllowMultiWin
	}
	return nil
}

// allowedTransitions lists the status changes an admin may make; closed is terminal.
var allowedTransitions = map[string][]string{
	StatusDraft: {StatusReady},
	StatusReady: {StatusDraft, StatusClosed},
}

func checkTransition(from, to string) error {
	if from == to {
		return nil
	}
	for _, next := range allowedTransitions[from] {
		if next == to {
			return nil
		}
	}
	return bizerr.New(bizerr.CodeConflict)
}

// checkComplete lists what a ready event still lacks.
func checkComplete(ctx context.Context, tx *gorm.DB, ev Event) error {
	var missing []string
	if ev.CheckinStart == nil || ev.CheckinEnd == nil {
		missing = append(missing, "签到时间窗")
	}
	if ev.CheckinMode == ModeGeo && (ev.CenterLat == nil || ev.CenterLng == nil) {
		missing = append(missing, "签到中心点")
	}
	var attendees int64
	if err := tx.WithContext(ctx).Model(&Attendee{}).Where("event_id = ?", ev.ID).Count(&attendees).Error; err != nil {
		return fmt.Errorf("event: count attendees: %w", err)
	}
	if attendees == 0 {
		missing = append(missing, "名单")
	}
	if len(missing) > 0 {
		return bizerr.New(bizerr.CodeEventIncomplete, strings.Join(missing, "、"))
	}
	return nil
}

func (s *Service) views(ctx context.Context) *gorm.DB {
	return s.db.WithContext(ctx).Table("events").Select(`events.*,
		(SELECT count(*) FROM attendees WHERE attendees.event_id = events.id) AS attendee_count,
		(SELECT count(*) FROM prizes WHERE prizes.event_id = events.id) AS prize_count`)
}

// lockEvent loads an event of the organization and locks it for the rest of tx.
func lockEvent(ctx context.Context, tx *gorm.DB, orgID, id uuid.UUID) (Event, error) {
	var ev Event
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND org_id = ?", id, orgID).Take(&ev).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Event{}, bizerr.New(bizerr.CodeNotFound)
	}
	if err != nil {
		return Event{}, fmt.Errorf("event: load: %w", err)
	}
	return ev, nil
}

// editableEvent locks an event for a roster or prize change; closed events are read-only.
func editableEvent(ctx context.Context, tx *gorm.DB, orgID, id uuid.UUID) (Event, error) {
	ev, err := lockEvent(ctx, tx, orgID, id)
	if err != nil {
		return Event{}, err
	}
	if ev.Status == StatusClosed {
		return Event{}, bizerr.New(bizerr.CodeConflict)
	}
	return ev, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation
}

// asBizErr keeps business errors and wraps anything else as internal.
func asBizErr(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := bizerr.As(err); ok {
		return err
	}
	return bizerr.Wrap(bizerr.CodeInternal, err)
}

// SetMaxAttendees lets an operator change one event's limit. It never drops below the
// current roster and does not touch credits.
func (s *Service) SetMaxAttendees(ctx context.Context, orgID, id uuid.UUID, maxAttendees int) (View, error) {
	if maxAttendees <= 0 {
		return View{}, bizerr.New(bizerr.CodeBadRequest)
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ev, err := lockEvent(ctx, tx, orgID, id)
		if err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&Attendee{}).Where("event_id = ?", id).Count(&count).Error; err != nil {
			return fmt.Errorf("event: count attendees: %w", err)
		}
		if int64(maxAttendees) < count {
			return bizerr.New(bizerr.CodeConflict)
		}
		return tx.Model(&ev).Updates(map[string]any{"max_attendees": maxAttendees, "updated_at": s.now().UTC()}).Error
	})
	if err != nil {
		return View{}, asBizErr(err)
	}
	return s.Get(ctx, orgID, id)
}
