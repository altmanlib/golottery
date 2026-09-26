package draw

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"golottery/api/internal/auth"
	"golottery/api/internal/bizerr"
	"golottery/api/internal/event"
	"golottery/api/internal/org"
)

const hostLoginKeyPrefix = "host:"

// Credential is the one-time host password for an event.
type Credential struct {
	Password string
	PublicID string
	Path     string
}

// UpsertHost creates or resets the host password of an event and revokes every host token.
func (s *Service) UpsertHost(ctx context.Context, orgID, eventID uuid.UUID) (Credential, error) {
	var ev event.Event
	if err := s.db.WithContext(ctx).Where("id = ? AND org_id = ?", eventID, orgID).Take(&ev).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Credential{}, bizerr.New(bizerr.CodeNotFound)
		}
		return Credential{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	password, err := org.NewTempPassword()
	if err != nil {
		return Credential{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return Credential{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	now := s.now().UTC()
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var host Host
		err := tx.Where("event_id = ?", eventID).Take(&host).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			host = Host{
				ID: uuid.New(), OrgID: orgID, EventID: eventID,
				PasswordHash: hash, CreatedAt: now, UpdatedAt: now,
			}
			if err := tx.Create(&host).Error; err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			if err := tx.Model(&host).Updates(map[string]any{
				"password_hash": hash, "updated_at": now,
			}).Error; err != nil {
				return err
			}
		}
		return s.tokens.WithDB(tx).DeleteByPrincipal(ctx, auth.TokenTypeHost, host.ID.String())
	})
	if err != nil {
		return Credential{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return Credential{Password: password, PublicID: ev.PublicID, Path: "/host/" + ev.PublicID}, nil
}

// Login checks the host password of an event and returns a host token.
func (s *Service) Login(ctx context.Context, publicID, password string) (auth.IssuedToken, error) {
	publicID = trimPublicID(publicID)
	key := hostLoginKeyPrefix + publicID
	if wait, err := s.limiter.Check(ctx, key); err != nil {
		return auth.IssuedToken{}, bizerr.Wrap(bizerr.CodeInternal, err)
	} else if wait > 0 {
		return auth.IssuedToken{}, bizerr.New(bizerr.CodeTooManyAttempts, wait)
	}
	var host Host
	err := s.db.WithContext(ctx).
		Joins("JOIN events ON events.id = host_users.event_id").
		Where("events.public_id = ?", publicID).
		Take(&host).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		_ = s.limiter.Fail(ctx, key)
		return auth.IssuedToken{}, bizerr.New(bizerr.CodeInvalidCredentials)
	}
	if err != nil {
		return auth.IssuedToken{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	ok, err := auth.VerifyPassword(host.PasswordHash, password)
	if err != nil {
		return auth.IssuedToken{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	if !ok {
		_ = s.limiter.Fail(ctx, key)
		return auth.IssuedToken{}, bizerr.New(bizerr.CodeInvalidCredentials)
	}
	if err := s.limiter.Reset(ctx, key); err != nil {
		return auth.IssuedToken{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	issued, err := s.tokens.IssueHost(ctx, host.ID.String(), auth.TokenNameLogin)
	if err != nil {
		return auth.IssuedToken{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return issued, nil
}

// HostSession is the authenticated host bound to one event.
type HostSession struct {
	HostID  uuid.UUID
	OrgID   uuid.UUID
	EventID uuid.UUID
}

// ResolveHost loads the host and its event from a host token.
func (s *Service) ResolveHost(ctx context.Context, token auth.APIToken) (HostSession, error) {
	id, err := uuid.Parse(token.PrincipalID)
	if err != nil {
		return HostSession{}, bizerr.New(bizerr.CodeUnauthorized)
	}
	var host Host
	if err := s.db.WithContext(ctx).Where("id = ?", id).Take(&host).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return HostSession{}, bizerr.New(bizerr.CodeUnauthorized)
		}
		return HostSession{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return HostSession{HostID: host.ID, OrgID: host.OrgID, EventID: host.EventID}, nil
}

func trimPublicID(v string) string {
	return strings.TrimSpace(v)
}
