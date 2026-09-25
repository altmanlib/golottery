package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

// MinPasswordLen is the minimum length, in characters, of a password a user chooses.
const MinPasswordLen = 8

const (
	passwordMemory      uint32 = 64 * 1024
	passwordIterations  uint32 = 3
	passwordParallelism uint8  = 2
	passwordSaltBytes          = 16
	passwordKeyBytes           = 32
)

// HashPassword returns an argon2id PHC string.
func HashPassword(password string) (string, error) {
	salt := make([]byte, passwordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, passwordIterations, passwordMemory, passwordParallelism, passwordKeyBytes)
	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		passwordMemory,
		passwordIterations,
		passwordParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword reports whether password matches an argon2id PHC string.
func VerifyPassword(encoded, password string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false, fmt.Errorf("invalid argon2id password hash")
	}
	var memory uint32
	var iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil || memory == 0 || iterations == 0 || parallelism == 0 {
		return false, fmt.Errorf("invalid argon2id parameters")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("invalid argon2id salt: %w", err)
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("invalid argon2id key: %w", err)
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

var (
	dummyHashOnce sync.Once
	dummyHash     string
)

// VerifyDummy spends the cost of one real VerifyPassword so unknown accounts
// cannot be told apart from wrong passwords by response time.
func VerifyDummy(password string) {
	dummyHashOnce.Do(func() {
		dummyHash, _ = HashPassword("golottery-dummy-password")
	})
	if dummyHash != "" {
		_, _ = VerifyPassword(dummyHash, password)
	}
}
