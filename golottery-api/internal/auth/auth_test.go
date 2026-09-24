package auth

import (
	"context"
	"testing"
	"time"

	"golottery/api/internal/store"
)

func TestHashPasswordRoundTrip(t *testing.T) {
	encoded, err := HashPassword("correct horse")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	ok, err := VerifyPassword(encoded, "correct horse")
	if err != nil || !ok {
		t.Fatalf("verify match = %v %v", ok, err)
	}
	ok, err = VerifyPassword(encoded, "wrong")
	if err != nil || ok {
		t.Fatalf("verify mismatch = %v %v", ok, err)
	}
}

func TestTokenTypesAreNotInterchangeable(t *testing.T) {
	db := store.OpenTest(t)
	store.Reset(t, db, &APIToken{}, &LoginAttempt{})
	ctx := context.Background()
	issuer := NewTokenIssuer(db.Gorm, time.Hour, time.Hour, time.Hour)

	issued, err := issuer.IssueConsole(ctx, "user-1", TokenNameLogin)
	if err != nil {
		t.Fatalf("IssueConsole() error = %v", err)
	}
	if _, err := issuer.Lookup(ctx, TokenTypeConsole, issued.Plain); err != nil {
		t.Fatalf("Lookup console error = %v", err)
	}
	if _, err := issuer.Lookup(ctx, TokenTypeHost, issued.Plain); err != ErrTokenNotFound {
		t.Fatalf("host lookup error = %v, want ErrTokenNotFound", err)
	}
	if _, err := issuer.Lookup(ctx, TokenTypePlatform, issued.Plain); err != ErrTokenNotFound {
		t.Fatalf("platform lookup error = %v, want ErrTokenNotFound", err)
	}
}

func TestExpiredTokenRejected(t *testing.T) {
	db := store.OpenTest(t)
	store.Reset(t, db, &APIToken{})
	ctx := context.Background()
	issuer := NewTokenIssuer(db.Gorm, time.Hour, time.Hour, time.Hour)
	issuer.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }

	issued, err := issuer.IssueHost(ctx, "event-1", "")
	if err != nil {
		t.Fatalf("IssueHost() error = %v", err)
	}
	issuer.now = time.Now
	if _, err := issuer.Lookup(ctx, TokenTypeHost, issued.Plain); err != ErrTokenNotFound {
		t.Fatalf("expired lookup error = %v", err)
	}
}

func TestDeleteToken(t *testing.T) {
	db := store.OpenTest(t)
	store.Reset(t, db, &APIToken{})
	ctx := context.Background()
	issuer := NewTokenIssuer(db.Gorm, time.Hour, time.Hour, time.Hour)
	issued, err := issuer.IssuePlatform(ctx, "ops", TokenNameLogin)
	if err != nil {
		t.Fatalf("IssuePlatform() error = %v", err)
	}
	if err := issuer.Delete(ctx, issued.Row.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := issuer.Lookup(ctx, TokenTypePlatform, issued.Plain); err != ErrTokenNotFound {
		t.Fatalf("deleted lookup error = %v", err)
	}
}
