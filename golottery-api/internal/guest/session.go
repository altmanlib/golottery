// Package guest serves the people at an event: sign-in through the mini program or
// its web stand-in, roster binding, check-in, help requests and on-site staff.
package guest

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"golottery/api/internal/auth"
	"golottery/api/internal/bizerr"
	"golottery/api/internal/event"
	"golottery/api/internal/ratelimit"
	"golottery/api/internal/wechat"
)

// Login modes; only one is active per deployment.
const (
	ModeWechat = "wechat"
	ModeWeb    = "web"
)

// webOpenIDPrefix marks identities that come from a browser, not WeChat.
const webOpenIDPrefix = "web:"

var deviceIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,60}$`)

// ErrSessionNotFound is returned for unknown or expired guest tokens.
var ErrSessionNotFound = errors.New("guest: session not found")

// Session is one guest's token for one event. Guest tokens live apart from api_tokens.
type Session struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	EventID   uuid.UUID `gorm:"type:uuid;not null"`
	OpenID    string    `gorm:"column:openid;size:64;not null"`
	TokenHash []byte    `gorm:"type:bytea;not null"`
	ExpiresAt time.Time `gorm:"not null"`
	CreatedAt time.Time `gorm:"not null"`
}

// TableName returns the guest_sessions table name.
func (Session) TableName() string { return "guest_sessions" }

// Guest is the authenticated caller of a guest request.
type Guest struct {
	EventID uuid.UUID
	OpenID  string
}

// Config configures a Service.
type Config struct {
	Mode       string
	SessionTTL time.Duration
	Wechat     *wechat.Client
	Limiter    *ratelimit.Limiter
}

// Service implements the guest API.
type Service struct {
	db      *gorm.DB
	cfg     Config
	binding *auth.LoginLimiter
	now     func() time.Time
}

// Binding failures lock one identity for ten minutes after five misses.
const (
	bindMaxFailures = 5
	bindWindow      = 10 * time.Minute
	// Web identities are free to create, so web mode also limits failures per client IP.
	bindIPMaxFailures = 20
	checkinPerMinute  = 10
)

// NewService builds a Service.
func NewService(db *gorm.DB, cfg Config) *Service {
	if cfg.Mode == "" {
		cfg.Mode = ModeWechat
	}
	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = 24 * time.Hour
	}
	return &Service{db: db, cfg: cfg, binding: auth.NewLoginLimiter(db, bindMaxFailures, bindWindow), now: time.Now}
}

// Mode returns the login mode in use.
func (s *Service) Mode() string { return s.cfg.Mode }

// LoginInput carries what the client sends: a WeChat code, or a browser device id.
type LoginInput struct {
	PublicID string
	Code     string
	DeviceID string
}

// Login resolves the guest identity and issues a token for the event.
// Logging in again for the same event rotates the token.
func (s *Service) Login(ctx context.Context, in LoginInput) (auth.IssuedToken, event.Event, error) {
	var ev event.Event
	err := s.db.WithContext(ctx).Where("public_id = ?", in.PublicID).Take(&ev).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return auth.IssuedToken{}, event.Event{}, bizerr.New(bizerr.CodeNotFound)
	}
	if err != nil {
		return auth.IssuedToken{}, event.Event{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}

	openID, err := s.identify(ctx, in)
	if err != nil {
		return auth.IssuedToken{}, event.Event{}, err
	}
	plain, hash, err := auth.NewPlainToken()
	if err != nil {
		return auth.IssuedToken{}, event.Event{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	now := s.now().UTC()
	session := Session{ID: uuid.New(), EventID: ev.ID, OpenID: openID, TokenHash: hash, ExpiresAt: now.Add(s.cfg.SessionTTL), CreatedAt: now}
	err = s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "event_id"}, {Name: "openid"}},
		DoUpdates: clause.AssignmentColumns([]string{"token_hash", "expires_at", "created_at"}),
	}).Create(&session).Error
	if err != nil {
		return auth.IssuedToken{}, event.Event{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return auth.IssuedToken{Plain: plain, ExpiresAt: session.ExpiresAt}, ev, nil
}

func (s *Service) identify(ctx context.Context, in LoginInput) (string, error) {
	switch s.cfg.Mode {
	case ModeWeb:
		if !deviceIDPattern.MatchString(in.DeviceID) {
			return "", bizerr.New(bizerr.CodeBadRequest)
		}
		return webOpenIDPrefix + in.DeviceID, nil
	default:
		if in.Code == "" || s.cfg.Wechat == nil {
			return "", bizerr.New(bizerr.CodeBadRequest)
		}
		openID, err := s.cfg.Wechat.Code2Session(ctx, in.Code)
		if errors.Is(err, wechat.ErrNotConfigured) {
			return "", bizerr.New(bizerr.CodeWechatNotConfigured)
		}
		var apiErr *wechat.APIError
		if errors.As(err, &apiErr) {
			return "", bizerr.Wrap(bizerr.CodeUnauthorized, err)
		}
		if err != nil {
			return "", bizerr.Wrap(bizerr.CodeInternal, err)
		}
		return openID, nil
	}
}

// Lookup resolves a guest token; expired tokens do not exist.
func (s *Service) Lookup(ctx context.Context, plain string) (Guest, error) {
	var session Session
	err := s.db.WithContext(ctx).Where("token_hash = ?", auth.HashToken(plain)).Take(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Guest{}, ErrSessionNotFound
	}
	if err != nil {
		return Guest{}, fmt.Errorf("guest: lookup session: %w", err)
	}
	if !session.ExpiresAt.After(s.now()) {
		return Guest{}, ErrSessionNotFound
	}
	return Guest{EventID: session.EventID, OpenID: session.OpenID}, nil
}
