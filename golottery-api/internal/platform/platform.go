// Package platform owns platform operator accounts and their sessions.
package platform

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"golottery/api/internal/auth"
	"golottery/api/internal/bizerr"
)

const (
	// MaxUsernameLen matches platform_users.username; longer names cannot exist.
	MaxUsernameLen = 64
)

// User is one platform operator account.
type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	Username     string    `gorm:"size:64;uniqueIndex;not null"`
	PasswordHash string    `gorm:"not null"`
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`
}

// TableName returns the platform_users table name.
func (User) TableName() string { return "platform_users" }

// Seed inserts the first operator when platform_users is empty.
// It never touches existing rows, so a restart cannot undo a password change.
func Seed(ctx context.Context, db *gorm.DB, username, passwordHash string) (bool, error) {
	var count int64
	if err := db.WithContext(ctx).Model(&User{}).Count(&count).Error; err != nil {
		return false, fmt.Errorf("platform: count users: %w", err)
	}
	if count > 0 {
		return false, nil
	}
	if username == "" || passwordHash == "" {
		return false, errors.New("platform_users is empty: set PLATFORM_USER and PLATFORM_PASSWORD_HASH")
	}
	if _, err := auth.VerifyPassword(passwordHash, ""); err != nil {
		return false, fmt.Errorf("PLATFORM_PASSWORD_HASH: %w", err)
	}
	now := time.Now().UTC()
	user := User{ID: uuid.New(), Username: username, PasswordHash: passwordHash, CreatedAt: now, UpdatedAt: now}
	if err := db.WithContext(ctx).Create(&user).Error; err != nil {
		return false, fmt.Errorf("platform: seed user: %w", err)
	}
	return true, nil
}

// Service implements operator login, logout, profile and password change.
type Service struct {
	db      *gorm.DB
	tokens  *auth.TokenIssuer
	limiter *auth.LoginLimiter
}

// NewService builds a Service.
func NewService(db *gorm.DB, tokens *auth.TokenIssuer, limiter *auth.LoginLimiter) *Service {
	return &Service{db: db, tokens: tokens, limiter: limiter}
}

// Login checks credentials and issues a platform token.
func (s *Service) Login(ctx context.Context, username, password string) (auth.IssuedToken, error) {
	// Such a name has no account and would not fit the limiter key; answer like any unknown user.
	if utf8.RuneCountInString(username) > MaxUsernameLen {
		auth.VerifyDummy(password)
		return auth.IssuedToken{}, bizerr.New(bizerr.CodeInvalidCredentials)
	}
	key := "platform:" + username
	wait, err := s.limiter.Check(ctx, key)
	if err != nil {
		return auth.IssuedToken{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	if wait > 0 {
		return auth.IssuedToken{}, bizerr.New(bizerr.CodeTooManyAttempts, wait)
	}

	user, err := s.findByUsername(ctx, username)
	if err != nil {
		return auth.IssuedToken{}, err
	}
	ok := false
	if user == nil {
		auth.VerifyDummy(password)
	} else {
		ok, err = auth.VerifyPassword(user.PasswordHash, password)
		if err != nil {
			return auth.IssuedToken{}, bizerr.Wrap(bizerr.CodeInternal, err)
		}
	}
	if !ok {
		if err := s.limiter.Fail(ctx, key); err != nil {
			return auth.IssuedToken{}, bizerr.Wrap(bizerr.CodeInternal, err)
		}
		return auth.IssuedToken{}, bizerr.New(bizerr.CodeInvalidCredentials)
	}

	if err := s.limiter.Reset(ctx, key); err != nil {
		return auth.IssuedToken{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	issued, err := s.tokens.IssuePlatform(ctx, user.ID.String(), auth.TokenNameLogin)
	if err != nil {
		return auth.IssuedToken{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return issued, nil
}

// Logout revokes one token.
func (s *Service) Logout(ctx context.Context, token auth.APIToken) error {
	if err := s.tokens.Delete(ctx, token.ID); err != nil {
		return bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return nil
}

// Me returns the operator that owns token.
func (s *Service) Me(ctx context.Context, token auth.APIToken) (User, error) {
	return s.findByToken(ctx, token)
}

// ChangePassword replaces the password, revokes every token of the operator and issues a new one.
func (s *Service) ChangePassword(ctx context.Context, token auth.APIToken, current, next string) (auth.IssuedToken, error) {
	if utf8.RuneCountInString(next) < auth.MinPasswordLen {
		return auth.IssuedToken{}, bizerr.New(bizerr.CodePasswordTooShort)
	}
	user, err := s.findByToken(ctx, token)
	if err != nil {
		return auth.IssuedToken{}, err
	}
	ok, err := auth.VerifyPassword(user.PasswordHash, current)
	if err != nil {
		return auth.IssuedToken{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	if !ok {
		return auth.IssuedToken{}, bizerr.New(bizerr.CodeCurrentPasswordWrong)
	}
	if next == current {
		return auth.IssuedToken{}, bizerr.New(bizerr.CodePasswordUnchanged)
	}
	hash, err := auth.HashPassword(next)
	if err != nil {
		return auth.IssuedToken{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}

	var issued auth.IssuedToken
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Model(&User{}).Where("id = ?", user.ID).
			Updates(map[string]any{"password_hash": hash, "updated_at": time.Now().UTC()}).Error
		if err != nil {
			return fmt.Errorf("platform: update password: %w", err)
		}
		tokens := s.tokens.WithDB(tx)
		if err := tokens.DeleteByPrincipal(ctx, auth.TokenTypePlatform, user.ID.String()); err != nil {
			return err
		}
		issued, err = tokens.IssuePlatform(ctx, user.ID.String(), auth.TokenNameLogin)
		return err
	})
	if err != nil {
		return auth.IssuedToken{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return issued, nil
}

func (s *Service) findByUsername(ctx context.Context, username string) (*User, error) {
	var user User
	err := s.db.WithContext(ctx).Where("username = ?", username).Take(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return &user, nil
}

// findByToken treats a token whose operator no longer exists as unauthorized.
func (s *Service) findByToken(ctx context.Context, token auth.APIToken) (User, error) {
	var user User
	err := s.db.WithContext(ctx).Where("id = ?", token.PrincipalID).Take(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return User{}, bizerr.New(bizerr.CodeUnauthorized)
	}
	if err != nil {
		return User{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return user, nil
}
