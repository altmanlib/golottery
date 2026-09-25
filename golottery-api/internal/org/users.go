package org

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"golottery/api/internal/auth"
	"golottery/api/internal/bizerr"
)

const (
	maxEmailLen = 254

	tempPasswordLen = 10
	// tempPasswordAlphabet leaves out characters that are easy to misread: 0 O o 1 l I.
	tempPasswordAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789"

	pgUniqueViolation = "23505"
)

// User is one organization admin. Each email belongs to exactly one organization.
type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID        uuid.UUID `gorm:"type:uuid;not null"`
	Email        string    `gorm:"size:254;not null"`
	Name         string    `gorm:"size:100;not null"`
	PasswordHash string    `gorm:"not null"`
	Status       string    `gorm:"size:16;not null"`
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`
}

// TableName returns the org_users table name.
func (User) TableName() string { return "org_users" }

// Accounts manages organization admins and their console sessions.
type Accounts struct {
	db      *gorm.DB
	tokens  *auth.TokenIssuer
	limiter *auth.LoginLimiter
	now     func() time.Time
}

// NewAccounts builds an Accounts service.
func NewAccounts(db *gorm.DB, tokens *auth.TokenIssuer, limiter *auth.LoginLimiter) *Accounts {
	return &Accounts{db: db, tokens: tokens, limiter: limiter, now: time.Now}
}

// NormalizeEmail trims and lowercases an email the way it is stored.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validEmail(email string) bool {
	if email == "" || len(email) > maxEmailLen {
		return false
	}
	addr, err := mail.ParseAddress(email)
	return err == nil && addr.Name == "" && addr.Address == email
}

func newTempPassword() (string, error) {
	out := make([]byte, tempPasswordLen)
	limit := big.NewInt(int64(len(tempPasswordAlphabet)))
	for i := range out {
		n, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", fmt.Errorf("org: random password: %w", err)
		}
		out[i] = tempPasswordAlphabet[n.Int64()]
	}
	return string(out), nil
}

// ListUsers returns the admins of an organization, oldest first.
func (a *Accounts) ListUsers(ctx context.Context, orgID uuid.UUID) ([]User, error) {
	if err := a.orgExists(ctx, orgID); err != nil {
		return nil, err
	}
	var users []User
	if err := a.db.WithContext(ctx).Where("org_id = ?", orgID).Order("created_at, id").Find(&users).Error; err != nil {
		return nil, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return users, nil
}

// CreateUser adds an admin and returns its one-time password.
func (a *Accounts) CreateUser(ctx context.Context, orgID uuid.UUID, name, email string) (User, string, error) {
	name = strings.TrimSpace(name)
	email = NormalizeEmail(email)
	if name == "" || utf8.RuneCountInString(name) > maxNameLen || !validEmail(email) {
		return User{}, "", bizerr.New(bizerr.CodeBadRequest)
	}
	if err := a.orgExists(ctx, orgID); err != nil {
		return User{}, "", err
	}
	password, hash, err := newPasswordAndHash()
	if err != nil {
		return User{}, "", err
	}
	now := a.now().UTC()
	user := User{ID: uuid.New(), OrgID: orgID, Email: email, Name: name, PasswordHash: hash, Status: StatusActive, CreatedAt: now, UpdatedAt: now}
	if err := a.db.WithContext(ctx).Create(&user).Error; err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return User{}, "", bizerr.New(bizerr.CodeConflict)
		}
		return User{}, "", bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return user, password, nil
}

// ResetPassword gives the admin a new one-time password and revokes its tokens.
func (a *Accounts) ResetPassword(ctx context.Context, orgID, userID uuid.UUID) (string, error) {
	password, hash, err := newPasswordAndHash()
	if err != nil {
		return "", err
	}
	err = a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&User{}).Where("id = ? AND org_id = ?", userID, orgID).
			Updates(map[string]any{"password_hash": hash, "updated_at": a.now().UTC()})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return bizerr.New(bizerr.CodeNotFound)
		}
		return a.tokens.WithDB(tx).DeleteByPrincipal(ctx, auth.TokenTypeConsole, userID.String())
	})
	if err != nil {
		return "", asBizErr(err)
	}
	return password, nil
}

// SetUserStatus enables or disables an admin. Disabling revokes its tokens; repeating a status is a no-op.
func (a *Accounts) SetUserStatus(ctx context.Context, orgID, userID uuid.UUID, status string) error {
	err := a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user User
		err := tx.Where("id = ? AND org_id = ?", userID, orgID).Take(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return bizerr.New(bizerr.CodeNotFound)
		}
		if err != nil {
			return err
		}
		if user.Status != status {
			err := tx.Model(&User{}).Where("id = ?", userID).
				Updates(map[string]any{"status": status, "updated_at": a.now().UTC()}).Error
			if err != nil {
				return err
			}
		}
		if status == StatusDisabled {
			return a.tokens.WithDB(tx).DeleteByPrincipal(ctx, auth.TokenTypeConsole, userID.String())
		}
		return nil
	})
	return asBizErr(err)
}

func (a *Accounts) orgExists(ctx context.Context, orgID uuid.UUID) error {
	var count int64
	if err := a.db.WithContext(ctx).Model(&Org{}).Where("id = ?", orgID).Count(&count).Error; err != nil {
		return bizerr.Wrap(bizerr.CodeInternal, err)
	}
	if count == 0 {
		return bizerr.New(bizerr.CodeNotFound)
	}
	return nil
}

func newPasswordAndHash() (string, string, error) {
	password, err := newTempPassword()
	if err != nil {
		return "", "", bizerr.Wrap(bizerr.CodeInternal, err)
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return "", "", bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return password, hash, nil
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
