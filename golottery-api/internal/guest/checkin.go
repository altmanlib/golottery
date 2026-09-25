package guest

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"golottery/api/internal/bizerr"
	"golottery/api/internal/event"
	"golottery/api/internal/geo"
)

// Check-in methods stored on attendees.
const (
	MethodGeo    = "geo"
	MethodDirect = "direct"
	MethodManual = "manual"
	MethodProxy  = "proxy"
)

// Help request statuses.
const (
	RequestPending  = "pending"
	RequestApproved = "approved"
	RequestRejected = "rejected"
)

// Coordinate systems a client may report.
const (
	CoordGCJ02 = "gcj02"
	CoordWGS84 = "wgs84"
)

const (
	// accuracyAllowance is the most GPS inaccuracy credited toward the fence.
	accuracyAllowance = 200.0
	// maxAccuracy is the worst accuracy that may check in automatically.
	maxAccuracy  = 500.0
	maxReasonLen = 200
)

// Attempt records one check-in tap by a guest, passed or not. Rows are never changed.
type Attempt struct {
	ID         int64      `gorm:"primaryKey;autoIncrement"`
	EventID    uuid.UUID  `gorm:"type:uuid;not null"`
	AttendeeID *uuid.UUID `gorm:"type:uuid"`
	OpenID     string     `gorm:"column:openid;size:64;not null"`
	Lat        *float64   `gorm:"type:numeric(9,6)"`
	Lng        *float64   `gorm:"type:numeric(9,6)"`
	AccuracyM  *float64   `gorm:"column:accuracy_m;type:numeric(8,1)"`
	DistanceM  *float64   `gorm:"column:distance_m;type:numeric(10,1)"`
	Result     string     `gorm:"size:16;not null"`
	Reason     string     `gorm:"size:32;not null"`
	CreatedAt  time.Time  `gorm:"not null"`
}

// TableName returns the checkin_attempts table name.
func (Attempt) TableName() string { return "checkin_attempts" }

// ManualRequest asks on-site staff for help: a failed location, or a name the roster misses.
type ManualRequest struct {
	ID                uuid.UUID  `gorm:"type:uuid;primaryKey"`
	EventID           uuid.UUID  `gorm:"type:uuid;not null"`
	OpenID            string     `gorm:"column:openid;size:64;not null"`
	AttendeeID        *uuid.UUID `gorm:"type:uuid"`
	ClaimedName       string     `gorm:"size:100;not null"`
	ClaimedPhoneLast4 string     `gorm:"column:claimed_phone_last4;size:4;not null"`
	Reason            string     `gorm:"size:200;not null"`
	Status            string     `gorm:"size:16;not null"`
	HandledBy         *string    `gorm:"size:64"`
	HandledAt         *time.Time
	CreatedAt         time.Time `gorm:"not null"`
}

// TableName returns the manual_requests table name.
func (ManualRequest) TableName() string { return "manual_requests" }

// Status is everything the guest page shows.
type Status struct {
	Event     event.Event
	Attendee  *event.Attendee
	Request   *ManualRequest
	StaffRole string
}

// CheckinInput is a check-in tap; coordinates are only read in geo mode.
type CheckinInput struct {
	Lat, Lng, Accuracy *float64
	CoordType          string
}

// RequestInput is a help request; name and last four are needed only before binding.
type RequestInput struct {
	Name       string
	PhoneLast4 string
	Reason     string
}

// Status returns the guest's current view of the event.
func (s *Service) Status(ctx context.Context, g Guest) (Status, error) {
	var st Status
	if err := s.db.WithContext(ctx).Where("id = ?", g.EventID).Take(&st.Event).Error; err != nil {
		return Status{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	var attendee event.Attendee
	err := s.db.WithContext(ctx).Where("event_id = ? AND openid = ?", g.EventID, g.OpenID).Take(&attendee).Error
	switch {
	case err == nil:
		st.Attendee = &attendee
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return Status{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	var req ManualRequest
	err = s.db.WithContext(ctx).Where("event_id = ? AND openid = ?", g.EventID, g.OpenID).Order("created_at DESC").Take(&req).Error
	switch {
	case err == nil:
		st.Request = &req
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return Status{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	role, err := s.staffRole(ctx, g)
	if err != nil {
		return Status{}, err
	}
	st.StaffRole = role
	return st, nil
}

func validLast4(v string) bool {
	if len(v) != 4 {
		return false
	}
	for _, r := range v {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// Bind ties the guest to a roster person by name and the last four phone digits.
// A person binds to one identity only; repeated misses lock the identity, and in web
// mode also the client IP.
func (s *Service) Bind(ctx context.Context, g Guest, name, last4, clientIP string) (Status, error) {
	st, err := s.Status(ctx, g)
	if err != nil {
		return Status{}, err
	}
	if st.Event.Status == event.StatusClosed {
		return Status{}, bizerr.New(bizerr.CodeEventNotOpen)
	}
	if st.Attendee != nil {
		return st, nil
	}
	name = strings.TrimSpace(name)
	if name == "" || !validLast4(last4) {
		return Status{}, bizerr.New(bizerr.CodeBadRequest)
	}

	key := fmt.Sprintf("bind:%s:%s", g.EventID, g.OpenID)
	wait, err := s.binding.Check(ctx, key)
	if err != nil {
		return Status{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	if wait > 0 {
		return Status{}, bizerr.New(bizerr.CodeTooManyAttempts, wait)
	}
	ipKey := fmt.Sprintf("bind-ip:%s:%s", g.EventID, clientIP)
	webMode := s.cfg.Mode == ModeWeb && s.cfg.Limiter != nil
	if webMode && s.cfg.Limiter.Exceeded(ctx, ipKey, bindIPMaxFailures) {
		return Status{}, bizerr.New(bizerr.CodeTooManyAttempts, int(bindWindow/time.Minute))
	}

	var attendee event.Attendee
	err = s.db.WithContext(ctx).Where("event_id = ? AND name = ? AND phone_last4 = ?", g.EventID, name, last4).Take(&attendee).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := s.binding.Fail(ctx, key); err != nil {
			return Status{}, bizerr.Wrap(bizerr.CodeInternal, err)
		}
		if webMode {
			s.cfg.Limiter.Allow(ctx, ipKey, bindIPMaxFailures, bindWindow)
		}
		return Status{}, bizerr.New(bizerr.CodeAttendeeNotMatched)
	}
	if err != nil {
		return Status{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	res := s.db.WithContext(ctx).Model(&event.Attendee{}).
		Where("id = ? AND openid IS NULL", attendee.ID).Update("openid", g.OpenID)
	if res.Error != nil {
		if isUniqueViolation(res.Error) {
			return s.Status(ctx, g)
		}
		return Status{}, bizerr.Wrap(bizerr.CodeInternal, res.Error)
	}
	if res.RowsAffected == 0 {
		return Status{}, bizerr.New(bizerr.CodeAttendeeTaken)
	}
	if err := s.binding.Reset(ctx, key); err != nil {
		return Status{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return s.Status(ctx, g)
}

// Checkin decides a check-in tap on the server. The order of checks is fixed: bound,
// event open, already checked in, time window, then (geo only) coordinates, accuracy
// and distance. Every tap is recorded.
func (s *Service) Checkin(ctx context.Context, g Guest, in CheckinInput) (Status, error) {
	if s.cfg.Limiter != nil && !s.cfg.Limiter.Allow(ctx, fmt.Sprintf("checkin:%s:%s", g.EventID, g.OpenID), checkinPerMinute, time.Minute) {
		return Status{}, bizerr.New(bizerr.CodeTooManyAttempts, 1)
	}
	st, err := s.Status(ctx, g)
	if err != nil {
		return Status{}, err
	}
	now := s.now().UTC()
	attempt := Attempt{EventID: g.EventID, OpenID: g.OpenID, CreatedAt: now}
	if st.Attendee != nil {
		attempt.AttendeeID = &st.Attendee.ID
	}
	reject := func(reason string, err error) (Status, error) {
		attempt.Result, attempt.Reason = "rejected", reason
		if dbErr := s.db.WithContext(ctx).Create(&attempt).Error; dbErr != nil {
			return Status{}, bizerr.Wrap(bizerr.CodeInternal, dbErr)
		}
		return Status{}, err
	}
	ev := st.Event

	switch {
	case st.Attendee == nil:
		return reject("not_bound", bizerr.New(bizerr.CodeNotBound))
	case ev.Status != event.StatusReady:
		return reject("event_not_open", bizerr.New(bizerr.CodeEventNotOpen))
	case st.Attendee.Status == event.AttendeeCheckedIn:
		attempt.Result, attempt.Reason = "passed", "already"
		if err := s.db.WithContext(ctx).Create(&attempt).Error; err != nil {
			return Status{}, bizerr.Wrap(bizerr.CodeInternal, err)
		}
		return st, nil
	case ev.CheckinStart == nil || ev.CheckinEnd == nil || now.Before(*ev.CheckinStart) || now.After(*ev.CheckinEnd):
		return reject("window_closed", bizerr.New(bizerr.CodeWindowClosed))
	}

	method := MethodDirect
	if ev.CheckinMode == event.ModeGeo {
		method = MethodGeo
		if in.Lat == nil || in.Lng == nil || in.Accuracy == nil || !geo.Valid(*in.Lat, *in.Lng) || *in.Accuracy < 0 ||
			(in.CoordType != "" && in.CoordType != CoordGCJ02 && in.CoordType != CoordWGS84) {
			return reject("bad_coordinates", bizerr.New(bizerr.CodeBadRequest))
		}
		lat, lng := *in.Lat, *in.Lng
		if in.CoordType == CoordWGS84 {
			lat, lng = geo.WGS84ToGCJ02(lat, lng)
		}
		accuracy := *in.Accuracy
		attempt.Lat, attempt.Lng, attempt.AccuracyM = &lat, &lng, &accuracy
		if ev.CenterLat == nil || ev.CenterLng == nil {
			return reject("no_fence", bizerr.New(bizerr.CodeEventNotOpen))
		}
		distance := geo.DistanceM(lat, lng, *ev.CenterLat, *ev.CenterLng)
		attempt.DistanceM = &distance
		if accuracy > maxAccuracy {
			return reject("low_accuracy", bizerr.New(bizerr.CodeLowAccuracy))
		}
		if distance-math.Min(accuracy, accuracyAllowance) > float64(ev.RadiusM) {
			return reject("out_of_range", bizerr.New(bizerr.CodeOutOfRange, int(math.Round(distance))))
		}
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		attempt.Result = "passed"
		if err := tx.Create(&attempt).Error; err != nil {
			return err
		}
		// Only the first successful tap sets the time; a concurrent second one changes nothing.
		return tx.Model(&event.Attendee{}).Where("id = ? AND status = ?", st.Attendee.ID, event.AttendeePending).
			Updates(map[string]any{"status": event.AttendeeCheckedIn, "checkin_at": now, "checkin_method": method}).Error
	})
	if err != nil {
		return Status{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return s.Status(ctx, g)
}

// SubmitRequest asks staff for help. One pending request per guest at a time.
func (s *Service) SubmitRequest(ctx context.Context, g Guest, in RequestInput) (Status, error) {
	st, err := s.Status(ctx, g)
	if err != nil {
		return Status{}, err
	}
	if st.Event.Status == event.StatusClosed {
		return Status{}, bizerr.New(bizerr.CodeEventNotOpen)
	}
	reason := strings.TrimSpace(in.Reason)
	if utf8.RuneCountInString(reason) > maxReasonLen {
		return Status{}, bizerr.New(bizerr.CodeBadRequest)
	}
	req := ManualRequest{ID: uuid.New(), EventID: g.EventID, OpenID: g.OpenID, Reason: reason, Status: RequestPending, CreatedAt: s.now().UTC()}
	if st.Attendee != nil {
		if st.Attendee.Status == event.AttendeeCheckedIn {
			return Status{}, bizerr.New(bizerr.CodeConflict)
		}
		req.AttendeeID = &st.Attendee.ID
	} else {
		name := strings.TrimSpace(in.Name)
		if name == "" || utf8.RuneCountInString(name) > 100 || !validLast4(in.PhoneLast4) {
			return Status{}, bizerr.New(bizerr.CodeBadRequest)
		}
		req.ClaimedName, req.ClaimedPhoneLast4 = name, in.PhoneLast4
	}
	if err := s.db.WithContext(ctx).Create(&req).Error; err != nil {
		if isUniqueViolation(err) {
			return Status{}, bizerr.New(bizerr.CodeConflict)
		}
		return Status{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return s.Status(ctx, g)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
