package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	// TokenTypeConsole is the principal type for organization console tokens.
	TokenTypeConsole = "console"
	// TokenTypeHost is the principal type for on-site host tokens.
	TokenTypeHost = "host"
	// TokenTypePlatform is the principal type for platform operator tokens.
	TokenTypePlatform = "platform"

	// TokenNameLogin is the stored name for tokens issued at login.
	TokenNameLogin = "login"

	tokenPlainBytes = 32
)

// ErrTokenNotFound is returned when a hash does not match a live row.
var ErrTokenNotFound = errors.New("auth: token not found")

// APIToken is one stored bearer token. Plaintext is never persisted.
type APIToken struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	PrincipalType string    `gorm:"column:principal_type;size:32;not null"`
	PrincipalID   string    `gorm:"column:principal_id;size:64;not null"`
	Name          string    `gorm:"size:64;not null"`
	TokenHash     []byte    `gorm:"column:token_hash;type:bytea;not null"`
	ExpiresAt     time.Time `gorm:"column:expires_at;not null"`
	LastUsedAt    *time.Time
	CreatedAt     time.Time `gorm:"not null"`
}

// TableName returns the api_tokens table name.
func (APIToken) TableName() string { return "api_tokens" }

// IssuedToken is the result of a successful issue. Plain is shown once.
type IssuedToken struct {
	Plain     string
	ExpiresAt time.Time
	Row       APIToken
}

// TokenIssuer issues and looks up hashed bearer tokens.
type TokenIssuer struct {
	db          *gorm.DB
	consoleTTL  time.Duration
	hostTTL     time.Duration
	platformTTL time.Duration
	now         func() time.Time
}

// NewTokenIssuer builds an issuer. Non-positive TTLs fall back to the defaults.
func NewTokenIssuer(db *gorm.DB, consoleTTL, hostTTL, platformTTL time.Duration) *TokenIssuer {
	if consoleTTL <= 0 {
		consoleTTL = 12 * time.Hour
	}
	if hostTTL <= 0 {
		hostTTL = 12 * time.Hour
	}
	if platformTTL <= 0 {
		platformTTL = 8 * time.Hour
	}
	return &TokenIssuer{
		db:          db,
		consoleTTL:  consoleTTL,
		hostTTL:     hostTTL,
		platformTTL: platformTTL,
		now:         time.Now,
	}
}

// IssueConsole inserts a console token and returns the plaintext once.
func (s *TokenIssuer) IssueConsole(ctx context.Context, principalID, name string) (IssuedToken, error) {
	return s.issue(ctx, TokenTypeConsole, principalID, name, s.consoleTTL)
}

// IssueHost inserts a host token and returns the plaintext once.
func (s *TokenIssuer) IssueHost(ctx context.Context, principalID, name string) (IssuedToken, error) {
	return s.issue(ctx, TokenTypeHost, principalID, name, s.hostTTL)
}

// IssuePlatform inserts a platform token and returns the plaintext once.
func (s *TokenIssuer) IssuePlatform(ctx context.Context, principalID, name string) (IssuedToken, error) {
	return s.issue(ctx, TokenTypePlatform, principalID, name, s.platformTTL)
}

// Lookup returns the live row for plaintext when its type matches typ.
func (s *TokenIssuer) Lookup(ctx context.Context, typ, plain string) (APIToken, error) {
	hash := hashToken(plain)
	var row APIToken
	err := s.db.WithContext(ctx).Where("token_hash = ? AND principal_type = ?", hash, typ).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return APIToken{}, ErrTokenNotFound
	}
	if err != nil {
		return APIToken{}, fmt.Errorf("auth: lookup token: %w", err)
	}
	if !row.ExpiresAt.After(s.now()) {
		return APIToken{}, ErrTokenNotFound
	}
	return row, nil
}

// Delete removes one token row by id.
func (s *TokenIssuer) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.db.WithContext(ctx).Delete(&APIToken{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("auth: delete token: %w", err)
	}
	return nil
}

func (s *TokenIssuer) issue(ctx context.Context, typ, principalID, name string, ttl time.Duration) (IssuedToken, error) {
	if name == "" {
		name = TokenNameLogin
	}
	plain, hash, err := newPlainToken()
	if err != nil {
		return IssuedToken{}, err
	}
	now := s.now().UTC()
	row := APIToken{
		ID:            uuid.New(),
		PrincipalType: typ,
		PrincipalID:   principalID,
		Name:          name,
		TokenHash:     hash,
		ExpiresAt:     now.Add(ttl),
		CreatedAt:     now,
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return IssuedToken{}, fmt.Errorf("auth: insert token: %w", err)
	}
	return IssuedToken{Plain: plain, ExpiresAt: row.ExpiresAt, Row: row}, nil
}

func newPlainToken() (string, []byte, error) {
	buf := make([]byte, tokenPlainBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("auth: random token: %w", err)
	}
	plain := hex.EncodeToString(buf)
	return plain, hashToken(plain), nil
}

func hashToken(plain string) []byte {
	sum := sha256.Sum256([]byte(plain))
	return sum[:]
}
