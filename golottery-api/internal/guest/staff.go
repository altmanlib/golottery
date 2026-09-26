package guest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"golottery/api/internal/auth"
	"golottery/api/internal/bizerr"
	"golottery/api/internal/event"
	"golottery/api/internal/geo"
	"golottery/api/internal/org"
)

// Staff roles. Admins may also change the check-in mode and fence on site.
const (
	RoleStaff = "staff"
	RoleAdmin = "admin"
)

// Staff grants one guest identity on-site powers for one event.
type Staff struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	EventID   uuid.UUID `gorm:"type:uuid;not null"`
	OpenID    string    `gorm:"column:openid;size:64;not null"`
	Role      string    `gorm:"size:16;not null"`
	CreatedAt time.Time `gorm:"not null"`
}

// TableName returns the event_staff table name.
func (Staff) TableName() string { return "event_staff" }

func (s *Service) staffRole(ctx context.Context, g Guest) (string, error) {
	var staff Staff
	err := s.db.WithContext(ctx).Where("event_id = ? AND openid = ?", g.EventID, g.OpenID).Take(&staff).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return staff.Role, nil
}

// inviteTTL is how long a staff invite link works; each link works once.
const inviteTTL = 24 * time.Hour

// Invite hands out one on-site role. Only the SHA-256 of the code is stored.
type Invite struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	EventID   uuid.UUID `gorm:"type:uuid;not null"`
	Role      string    `gorm:"size:16;not null"`
	CodeHash  []byte    `gorm:"type:bytea;not null"`
	ExpiresAt time.Time `gorm:"not null"`
	UsedBy    *string   `gorm:"size:64"`
	UsedAt    *time.Time
	CreatedAt time.Time `gorm:"not null"`
}

// TableName returns the staff_invites table name.
func (Invite) TableName() string { return "staff_invites" }

// StaffMember is a staff row with the roster name its identity is bound to, if any.
type StaffMember struct {
	Staff
	AttendeeName *string
}

// Summary is the on-site check-in progress.
type Summary struct {
	Total, CheckedIn, PendingRequests int64
}

// PendingRequest is a help request with the person it concerns, when known.
type PendingRequest struct {
	ManualRequest
	Attendee *event.Attendee
}

// Approval chooses how staff resolve a request from a guest who is not bound yet.
type Approval struct {
	AttendeeID *uuid.UUID
	Create     *event.AttendeeInput
}

// SettingsPatch is what an on-site admin may change.
type SettingsPatch struct {
	CheckinMode          *string
	CenterLat, CenterLng *float64
	RadiusM              *int
	// CoordType says which datum the center is in; wgs84 (a browser fix) is converted to the fence's gcj02.
	CoordType string
}

// ownedEvent checks that the event belongs to the organization.
func (s *Service) ownedEvent(ctx context.Context, orgID, eventID uuid.UUID) (event.Event, error) {
	var ev event.Event
	err := s.db.WithContext(ctx).Where("id = ? AND org_id = ?", eventID, orgID).Take(&ev).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return event.Event{}, bizerr.New(bizerr.CodeNotFound)
	}
	if err != nil {
		return event.Event{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return ev, nil
}

// CreateInvite issues a one-time invite code for a role.
func (s *Service) CreateInvite(ctx context.Context, orgID, eventID uuid.UUID, role string) (string, Invite, error) {
	if role != RoleStaff && role != RoleAdmin {
		return "", Invite{}, bizerr.New(bizerr.CodeBadRequest)
	}
	ev, err := s.ownedEvent(ctx, orgID, eventID)
	if err != nil {
		return "", Invite{}, err
	}
	if ev.Status == event.StatusClosed {
		return "", Invite{}, bizerr.New(bizerr.CodeConflict)
	}
	plain, hash, err := auth.NewPlainToken()
	if err != nil {
		return "", Invite{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	now := s.now().UTC()
	inv := Invite{ID: uuid.New(), EventID: eventID, Role: role, CodeHash: hash, ExpiresAt: now.Add(inviteTTL), CreatedAt: now}
	if err := s.db.WithContext(ctx).Create(&inv).Error; err != nil {
		return "", Invite{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return plain, inv, nil
}

// ListStaff returns the event's staff with the roster names they are bound to.
func (s *Service) ListStaff(ctx context.Context, orgID, eventID uuid.UUID) ([]StaffMember, error) {
	if _, err := s.ownedEvent(ctx, orgID, eventID); err != nil {
		return nil, err
	}
	var rows []StaffMember
	err := s.db.WithContext(ctx).Table("event_staff").
		Select("event_staff.*, attendees.name AS attendee_name").
		Joins("LEFT JOIN attendees ON attendees.event_id = event_staff.event_id AND attendees.openid = event_staff.openid").
		Where("event_staff.event_id = ?", eventID).Order("event_staff.created_at").Scan(&rows).Error
	if err != nil {
		return nil, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return rows, nil
}

// RemoveStaff revokes one staff member.
func (s *Service) RemoveStaff(ctx context.Context, orgID, eventID, staffID uuid.UUID) error {
	if _, err := s.ownedEvent(ctx, orgID, eventID); err != nil {
		return err
	}
	res := s.db.WithContext(ctx).Where("id = ? AND event_id = ?", staffID, eventID).Delete(&Staff{})
	if res.Error != nil {
		return bizerr.Wrap(bizerr.CodeInternal, res.Error)
	}
	if res.RowsAffected == 0 {
		return bizerr.New(bizerr.CodeNotFound)
	}
	return nil
}

// Join turns the guest into staff of its event using an invite code.
func (s *Service) Join(ctx context.Context, g Guest, code string) (Status, error) {
	now := s.now().UTC()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var inv Invite
		// Claiming the invite in one conditional update makes it single-use under races.
		res := tx.Model(&inv).Clauses(clause.Returning{}).
			Where("code_hash = ? AND event_id = ? AND used_at IS NULL AND expires_at > ?", auth.HashToken(code), g.EventID, now).
			Updates(map[string]any{"used_by": g.OpenID, "used_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return bizerr.New(bizerr.CodeInviteInvalid)
		}
		staff := Staff{ID: uuid.New(), EventID: g.EventID, OpenID: g.OpenID, Role: inv.Role, CreatedAt: now}
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "event_id"}, {Name: "openid"}},
			DoUpdates: clause.AssignmentColumns([]string{"role"}),
		}).Create(&staff).Error
	})
	if err != nil {
		return Status{}, asBizErr(err)
	}
	return s.Status(ctx, g)
}

// requireStaff returns the caller's role; guests without one see 404, as if nothing were there.
func (s *Service) requireStaff(ctx context.Context, g Guest, admin bool) (string, error) {
	role, err := s.staffRole(ctx, g)
	if err != nil {
		return "", err
	}
	if role == "" || (admin && role != RoleAdmin) {
		return "", bizerr.New(bizerr.CodeNotFound)
	}
	return role, nil
}

// Summary counts the roster, check-ins and open help requests.
func (s *Service) Summary(ctx context.Context, g Guest) (Summary, error) {
	if _, err := s.requireStaff(ctx, g, false); err != nil {
		return Summary{}, err
	}
	var out Summary
	db := s.db.WithContext(ctx)
	if err := db.Model(&event.Attendee{}).Where("event_id = ?", g.EventID).Count(&out.Total).Error; err != nil {
		return Summary{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	if err := db.Model(&event.Attendee{}).Where("event_id = ? AND status = ?", g.EventID, event.AttendeeCheckedIn).Count(&out.CheckedIn).Error; err != nil {
		return Summary{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	if err := db.Model(&ManualRequest{}).Where("event_id = ? AND status = ?", g.EventID, RequestPending).Count(&out.PendingRequests).Error; err != nil {
		return Summary{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return out, nil
}

// PendingRequests lists open help requests, oldest first.
func (s *Service) PendingRequests(ctx context.Context, g Guest) ([]PendingRequest, error) {
	if _, err := s.requireStaff(ctx, g, false); err != nil {
		return nil, err
	}
	var reqs []ManualRequest
	if err := s.db.WithContext(ctx).Where("event_id = ? AND status = ?", g.EventID, RequestPending).Order("created_at").Find(&reqs).Error; err != nil {
		return nil, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	out := make([]PendingRequest, len(reqs))
	for i, r := range reqs {
		out[i].ManualRequest = r
		if r.AttendeeID != nil {
			var a event.Attendee
			if err := s.db.WithContext(ctx).Where("id = ?", *r.AttendeeID).Take(&a).Error; err == nil {
				out[i].Attendee = &a
			}
		}
	}
	return out, nil
}

// SearchAttendees finds roster people by name for proxy check-in and request handling.
func (s *Service) SearchAttendees(ctx context.Context, g Guest, q string) ([]event.Attendee, error) {
	if _, err := s.requireStaff(ctx, g, false); err != nil {
		return nil, err
	}
	q = strings.TrimSpace(q)
	var rows []event.Attendee
	query := s.db.WithContext(ctx).Where("event_id = ?", g.EventID)
	if q != "" {
		query = query.Where("name LIKE ?", "%"+escapeLike(q)+"%")
	}
	if err := query.Order("name, id").Limit(20).Find(&rows).Error; err != nil {
		return nil, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return rows, nil
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// ApproveRequest resolves a help request in one step: bind when needed, then check in.
func (s *Service) ApproveRequest(ctx context.Context, g Guest, requestID uuid.UUID, how Approval) error {
	if _, err := s.requireStaff(ctx, g, false); err != nil {
		return err
	}
	now := s.now().UTC()
	var requester string
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ev, err := s.readyEvent(ctx, tx, g.EventID)
		if err != nil {
			return err
		}
		var req ManualRequest
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND event_id = ?", requestID, g.EventID).Take(&req).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return bizerr.New(bizerr.CodeNotFound)
		}
		if err != nil {
			return err
		}
		if req.Status != RequestPending {
			return bizerr.New(bizerr.CodeConflict)
		}
		requester = req.OpenID

		attendeeID := req.AttendeeID
		if attendeeID == nil {
			switch {
			case how.AttendeeID != nil:
				attendeeID = how.AttendeeID
			case how.Create != nil:
				created, err := event.AddAttendeeTx(ctx, tx, ev.OrgID, ev.ID, *how.Create, now)
				if err != nil {
					return err
				}
				attendeeID = &created.ID
			default:
				return bizerr.New(bizerr.CodeBadRequest)
			}
			// Bind the requester unless the person is already bound to someone else.
			res := tx.Model(&event.Attendee{}).
				Where("id = ? AND event_id = ? AND (openid IS NULL OR openid = ?)", *attendeeID, g.EventID, req.OpenID).
				Update("openid", req.OpenID)
			if res.Error != nil {
				if isUniqueViolation(res.Error) {
					return bizerr.New(bizerr.CodeConflict)
				}
				return res.Error
			}
			if res.RowsAffected == 0 {
				return bizerr.New(bizerr.CodeConflict)
			}
		}
		if err := checkInTx(tx, *attendeeID, MethodManual, g.OpenID, now); err != nil {
			return err
		}
		return tx.Model(&req).Updates(map[string]any{"status": RequestApproved, "attendee_id": *attendeeID, "handled_by": g.OpenID, "handled_at": now}).Error
	})
	if err != nil {
		return asBizErr(err)
	}
	// Staff vouched for the person, so their earlier binding misses no longer count.
	if err := s.binding.Reset(ctx, fmt.Sprintf("bind:%s:%s", g.EventID, requester)); err != nil {
		return bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return nil
}

// RejectRequest closes a help request without checking anyone in.
func (s *Service) RejectRequest(ctx context.Context, g Guest, requestID uuid.UUID) error {
	if _, err := s.requireStaff(ctx, g, false); err != nil {
		return err
	}
	res := s.db.WithContext(ctx).Model(&ManualRequest{}).
		Where("id = ? AND event_id = ? AND status = ?", requestID, g.EventID, RequestPending).
		Updates(map[string]any{"status": RequestRejected, "handled_by": g.OpenID, "handled_at": s.now().UTC()})
	if res.Error != nil {
		return bizerr.Wrap(bizerr.CodeInternal, res.Error)
	}
	if res.RowsAffected == 0 {
		return bizerr.New(bizerr.CodeNotFound)
	}
	return nil
}

// ProxyCheckin checks a roster person in on their behalf. Repeating it changes nothing.
func (s *Service) ProxyCheckin(ctx context.Context, g Guest, attendeeID uuid.UUID) (event.Attendee, error) {
	if _, err := s.requireStaff(ctx, g, false); err != nil {
		return event.Attendee{}, err
	}
	var a event.Attendee
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := s.readyEvent(ctx, tx, g.EventID); err != nil {
			return err
		}
		err := tx.Where("id = ? AND event_id = ?", attendeeID, g.EventID).Take(&a).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return bizerr.New(bizerr.CodeNotFound)
		}
		if err != nil {
			return err
		}
		if err := checkInTx(tx, a.ID, MethodProxy, g.OpenID, s.now().UTC()); err != nil {
			return err
		}
		return tx.Where("id = ?", a.ID).Take(&a).Error
	})
	if err != nil {
		return event.Attendee{}, asBizErr(err)
	}
	return a, nil
}

// UpdateSettings lets an on-site admin switch the check-in mode or move the fence.
func (s *Service) UpdateSettings(ctx context.Context, g Guest, p SettingsPatch) (event.View, error) {
	if _, err := s.requireStaff(ctx, g, true); err != nil {
		return event.View{}, err
	}
	lat, lng := p.CenterLat, p.CenterLng
	switch p.CoordType {
	case "", CoordGCJ02:
	case CoordWGS84:
		if lat != nil && lng != nil && geo.Valid(*lat, *lng) {
			glat, glng := geo.WGS84ToGCJ02(*lat, *lng)
			lat, lng = &glat, &glng
		}
	default:
		return event.View{}, bizerr.New(bizerr.CodeBadRequest)
	}
	var ev event.Event
	if err := s.db.WithContext(ctx).Where("id = ?", g.EventID).Take(&ev).Error; err != nil {
		return event.View{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return event.NewService(s.db).Update(ctx, ev.OrgID, ev.ID, event.Patch{
		CheckinMode: p.CheckinMode, CenterLat: lat, CenterLng: lng, RadiusM: p.RadiusM,
	}, org.Operator{Type: "guest", ID: g.OpenID})
}

func (s *Service) readyEvent(ctx context.Context, tx *gorm.DB, eventID uuid.UUID) (event.Event, error) {
	var ev event.Event
	if err := tx.WithContext(ctx).Where("id = ?", eventID).Take(&ev).Error; err != nil {
		return event.Event{}, err
	}
	if ev.Status != event.StatusReady {
		return event.Event{}, bizerr.New(bizerr.CodeEventNotOpen)
	}
	return ev, nil
}

// checkInTx marks a pending person checked in; someone already checked in keeps their record.
func checkInTx(tx *gorm.DB, attendeeID uuid.UUID, method, by string, at time.Time) error {
	return tx.Model(&event.Attendee{}).Where("id = ? AND status = ?", attendeeID, event.AttendeePending).
		Updates(map[string]any{"status": event.AttendeeCheckedIn, "checkin_at": at, "checkin_method": method, "checkin_by": by}).Error
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
