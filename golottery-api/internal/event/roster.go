package event

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"golottery/api/internal/bizerr"
)

const (
	maxDeptLen = 100
	maxGiftLen = 200

	// AttendeePending has not checked in yet.
	AttendeePending = "pending"
	// AttendeeCheckedIn is in the draw pool.
	AttendeeCheckedIn = "checked_in"
)

// Attendee is one person on an event roster. Only the last four phone digits are kept.
type Attendee struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID      uuid.UUID `gorm:"type:uuid;not null"`
	EventID    uuid.UUID `gorm:"type:uuid;not null"`
	Name       string    `gorm:"size:100;not null"`
	Dept       string    `gorm:"size:100;not null"`
	PhoneLast4 string    `gorm:"column:phone_last4;type:char(4);not null"`
	Status     string    `gorm:"size:16;not null"`
	CreatedAt  time.Time `gorm:"not null"`
	// Set by check-in (phase 6): the bound guest and how the person checked in.
	OpenID        *string `gorm:"column:openid;size:64"`
	CheckinAt     *time.Time
	CheckinMethod *string `gorm:"size:16"`
	CheckinBy     *string `gorm:"size:64"`
}

// TableName returns the attendees table name.
func (Attendee) TableName() string { return "attendees" }

// Prize is one prize tier of an event, drawn in sort_no order.
type Prize struct {
	ID      uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID   uuid.UUID `gorm:"type:uuid;not null"`
	EventID uuid.UUID `gorm:"type:uuid;not null"`
	Name    string    `gorm:"size:100;not null"`
	Gift    string    `gorm:"size:200;not null"`
	Quota   int       `gorm:"not null"`
	SortNo  int       `gorm:"not null"`
}

// TableName returns the prizes table name.
func (Prize) TableName() string { return "prizes" }

// AttendeeInput is a roster row as typed or imported.
type AttendeeInput struct {
	Name  string
	Dept  string
	Phone string
}

// AttendeePatch lists the attendee fields to change.
type AttendeePatch struct {
	Name  *string
	Dept  *string
	Phone *string
}

// PrizeInput describes a new prize.
type PrizeInput struct {
	Name   string
	Gift   string
	Quota  int
	SortNo int
}

// PrizePatch lists the prize fields to change.
type PrizePatch struct {
	Name   *string
	Gift   *string
	Quota  *int
	SortNo *int
}

// PhoneLast4 keeps the last four digits of a phone number; it needs at least four digits.
func PhoneLast4(phone string) (string, bool) {
	var digits strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	d := digits.String()
	if len(d) < 4 {
		return "", false
	}
	return d[len(d)-4:], true
}

// cleanAttendee validates one roster row and reduces the phone to its last four digits.
func cleanAttendee(in AttendeeInput) (AttendeeInput, error) {
	name := strings.TrimSpace(in.Name)
	dept := strings.TrimSpace(in.Dept)
	if name == "" {
		return AttendeeInput{}, bizerr.New(bizerr.CodeNameRequired)
	}
	if utf8.RuneCountInString(name) > maxNameLen || utf8.RuneCountInString(dept) > maxDeptLen {
		return AttendeeInput{}, bizerr.New(bizerr.CodeBadRequest)
	}
	last4, ok := PhoneLast4(in.Phone)
	if !ok {
		return AttendeeInput{}, bizerr.New(bizerr.CodeBadRequest)
	}
	return AttendeeInput{Name: name, Dept: dept, Phone: last4}, nil
}

// ListAttendees returns one page of the roster in the order it was added.
func (s *Service) ListAttendees(ctx context.Context, orgID, eventID uuid.UUID, offset, limit int) ([]Attendee, int64, error) {
	if _, err := s.Get(ctx, orgID, eventID); err != nil {
		return nil, 0, err
	}
	var total int64
	if err := s.db.WithContext(ctx).Model(&Attendee{}).Where("event_id = ?", eventID).Count(&total).Error; err != nil {
		return nil, 0, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	var rows []Attendee
	err := s.db.WithContext(ctx).Where("event_id = ?", eventID).
		Order("created_at, id").Offset(offset).Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, 0, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return rows, total, nil
}

// AddAttendee appends one person; the roster may not grow beyond the event limit.
func (s *Service) AddAttendee(ctx context.Context, orgID, eventID uuid.UUID, in AttendeeInput) (Attendee, error) {
	var row Attendee
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		row, err = AddAttendeeTx(ctx, tx, orgID, eventID, in, s.now().UTC())
		return err
	})
	if err != nil {
		return Attendee{}, err
	}
	return row, nil
}

// AddAttendeeTx adds one person inside tx with the same checks as AddAttendee, for callers
// that must add and act on the person atomically (on-site staff adding a walk-in).
func AddAttendeeTx(ctx context.Context, tx *gorm.DB, orgID, eventID uuid.UUID, in AttendeeInput, at time.Time) (Attendee, error) {
	in, err := cleanAttendee(in)
	if err != nil {
		return Attendee{}, err
	}
	ev, err := editableEvent(ctx, tx, orgID, eventID)
	if err != nil {
		return Attendee{}, asBizErr(err)
	}
	if err := checkRoom(ctx, tx, ev, 1); err != nil {
		return Attendee{}, asBizErr(err)
	}
	row := Attendee{
		ID: uuid.New(), OrgID: orgID, EventID: eventID,
		Name: in.Name, Dept: in.Dept, PhoneLast4: in.Phone,
		Status: AttendeePending, CreatedAt: at,
	}
	// A savepoint keeps a duplicate from aborting the caller's transaction.
	if err := tx.SavePoint("add_attendee").Error; err != nil {
		return Attendee{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	if err := tx.Create(&row).Error; err != nil {
		if isUniqueViolation(err) {
			_ = tx.RollbackTo("add_attendee").Error
			return Attendee{}, bizerr.New(bizerr.CodeConflict)
		}
		return Attendee{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return row, nil
}

// UpdateAttendee changes a roster row; a bound WeChat account stays bound.
func (s *Service) UpdateAttendee(ctx context.Context, orgID, eventID, attendeeID uuid.UUID, p AttendeePatch) (Attendee, error) {
	var row Attendee
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := editableEvent(ctx, tx, orgID, eventID); err != nil {
			return err
		}
		if err := findIn(ctx, tx, &row, eventID, attendeeID); err != nil {
			return err
		}
		next := AttendeeInput{Name: row.Name, Dept: row.Dept, Phone: row.PhoneLast4}
		if p.Name != nil {
			next.Name = *p.Name
		}
		if p.Dept != nil {
			next.Dept = *p.Dept
		}
		if p.Phone != nil {
			next.Phone = *p.Phone
		}
		clean, err := cleanAttendee(next)
		if err != nil {
			return err
		}
		row.Name, row.Dept, row.PhoneLast4 = clean.Name, clean.Dept, clean.Phone
		return tx.Save(&row).Error
	})
	if isUniqueViolation(err) {
		return Attendee{}, bizerr.New(bizerr.CodeConflict)
	}
	return row, asBizErr(err)
}

// DeleteAttendee removes a person who has not bound or checked in. Phase 7 adds the winner check.
func (s *Service) DeleteAttendee(ctx context.Context, orgID, eventID, attendeeID uuid.UUID) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := editableEvent(ctx, tx, orgID, eventID); err != nil {
			return err
		}
		var row Attendee
		if err := findIn(ctx, tx, &row, eventID, attendeeID); err != nil {
			return err
		}
		if row.OpenID != nil || row.Status != AttendeePending {
			return bizerr.New(bizerr.CodeConflict)
		}
		var wins int64
		if err := tx.Table("draw_results").Where("attendee_id = ?", attendeeID).Count(&wins).Error; err != nil {
			return err
		}
		if wins > 0 {
			return bizerr.New(bizerr.CodeConflict)
		}
		return tx.Delete(&row).Error
	})
	return asBizErr(err)
}

// checkRoom rejects adding n people when the roster would exceed the event limit.
func checkRoom(ctx context.Context, tx *gorm.DB, ev Event, n int) error {
	var count int64
	if err := tx.WithContext(ctx).Model(&Attendee{}).Where("event_id = ?", ev.ID).Count(&count).Error; err != nil {
		return fmt.Errorf("event: count attendees: %w", err)
	}
	if int(count)+n > ev.MaxAttendees {
		return bizerr.New(bizerr.CodeRosterFull, ev.MaxAttendees)
	}
	return nil
}

// ListPrizes returns the prizes of an event by sort_no.
func (s *Service) ListPrizes(ctx context.Context, orgID, eventID uuid.UUID) ([]Prize, error) {
	if _, err := s.Get(ctx, orgID, eventID); err != nil {
		return nil, err
	}
	var rows []Prize
	if err := s.db.WithContext(ctx).Where("event_id = ?", eventID).Order("sort_no, id").Find(&rows).Error; err != nil {
		return nil, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return rows, nil
}

func cleanPrize(in PrizeInput) (PrizeInput, error) {
	name := strings.TrimSpace(in.Name)
	gift := strings.TrimSpace(in.Gift)
	if name == "" {
		return PrizeInput{}, bizerr.New(bizerr.CodeNameRequired)
	}
	if utf8.RuneCountInString(name) > maxNameLen || utf8.RuneCountInString(gift) > maxGiftLen || in.Quota <= 0 {
		return PrizeInput{}, bizerr.New(bizerr.CodeBadRequest)
	}
	return PrizeInput{Name: name, Gift: gift, Quota: in.Quota, SortNo: in.SortNo}, nil
}

// AddPrize adds a prize; sort_no is unique within the event.
func (s *Service) AddPrize(ctx context.Context, orgID, eventID uuid.UUID, in PrizeInput) (Prize, error) {
	in, err := cleanPrize(in)
	if err != nil {
		return Prize{}, err
	}
	var row Prize
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := editableEvent(ctx, tx, orgID, eventID); err != nil {
			return err
		}
		row = Prize{ID: uuid.New(), OrgID: orgID, EventID: eventID, Name: in.Name, Gift: in.Gift, Quota: in.Quota, SortNo: in.SortNo}
		return tx.Create(&row).Error
	})
	if isUniqueViolation(err) {
		return Prize{}, bizerr.New(bizerr.CodeConflict)
	}
	return row, asBizErr(err)
}

// UpdatePrize changes a prize. Phase 7 adds the limits that apply once winners exist.
func (s *Service) UpdatePrize(ctx context.Context, orgID, eventID, prizeID uuid.UUID, p PrizePatch) (Prize, error) {
	var row Prize
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := editableEvent(ctx, tx, orgID, eventID); err != nil {
			return err
		}
		if err := findIn(ctx, tx, &row, eventID, prizeID); err != nil {
			return err
		}
		next := PrizeInput{Name: row.Name, Gift: row.Gift, Quota: row.Quota, SortNo: row.SortNo}
		if p.Name != nil {
			next.Name = *p.Name
		}
		if p.Gift != nil {
			next.Gift = *p.Gift
		}
		if p.Quota != nil {
			next.Quota = *p.Quota
		}
		if p.SortNo != nil {
			next.SortNo = *p.SortNo
		}
		clean, err := cleanPrize(next)
		if err != nil {
			return err
		}
		if clean.Quota < row.Quota {
			var won int64
			if err := tx.Table("draw_results").Where("prize_id = ? AND status = ?", prizeID, "valid").Count(&won).Error; err != nil {
				return err
			}
			if int64(clean.Quota) < won {
				return bizerr.New(bizerr.CodeConflict)
			}
		}
		row.Name, row.Gift, row.Quota, row.SortNo = clean.Name, clean.Gift, clean.Quota, clean.SortNo
		return tx.Save(&row).Error
	})
	if isUniqueViolation(err) {
		return Prize{}, bizerr.New(bizerr.CodeConflict)
	}
	return row, asBizErr(err)
}

// DeletePrize removes a prize. Phase 7 forbids it once the prize has winners.
func (s *Service) DeletePrize(ctx context.Context, orgID, eventID, prizeID uuid.UUID) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := editableEvent(ctx, tx, orgID, eventID); err != nil {
			return err
		}
		var wins int64
		if err := tx.Table("draw_results").Where("prize_id = ?", prizeID).Count(&wins).Error; err != nil {
			return err
		}
		if wins > 0 {
			return bizerr.New(bizerr.CodeConflict)
		}
		res := tx.Where("id = ? AND event_id = ?", prizeID, eventID).Delete(&Prize{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return bizerr.New(bizerr.CodeNotFound)
		}
		return nil
	})
	return asBizErr(err)
}

// findIn loads a row of the event by id into dst, or reports not found.
func findIn(ctx context.Context, tx *gorm.DB, dst any, eventID, id uuid.UUID) error {
	err := tx.WithContext(ctx).Where("id = ? AND event_id = ?", id, eventID).Take(dst).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return bizerr.New(bizerr.CodeNotFound)
	}
	return err
}
