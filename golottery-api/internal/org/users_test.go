package org

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"golottery/api/internal/auth"
	"golottery/api/internal/bizerr"
	"golottery/api/internal/store"
)

type accountsEnv struct {
	orgs     *Service
	accounts *Accounts
	tokens   *auth.TokenIssuer
	db       *store.DB
}

func newAccountsEnv(t *testing.T) accountsEnv {
	t.Helper()
	db := store.OpenTest(t)
	store.Reset(t, db, &User{}, &LedgerEntry{}, &Quota{}, &Org{}, &auth.APIToken{}, &auth.LoginAttempt{})
	tokens := auth.NewTokenIssuer(db.Gorm, time.Hour, time.Hour, time.Hour)
	limiter := auth.NewLoginLimiter(db.Gorm, 3, 15*time.Minute)
	return accountsEnv{orgs: NewService(db.Gorm), accounts: NewAccounts(db.Gorm, tokens, limiter), tokens: tokens, db: db}
}

func (e accountsEnv) mustCreateUser(t *testing.T, orgID uuid.UUID, email string) (User, string) {
	t.Helper()
	user, password, err := e.accounts.CreateUser(context.Background(), orgID, "管理员", email)
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	return user, password
}

func TestCreateUserReturnsPasswordOnce(t *testing.T) {
	env := newAccountsEnv(t)
	ctx := context.Background()
	acme := mustCreate(t, env.orgs, 1)

	user, password, err := env.accounts.CreateUser(ctx, acme.ID, " 李雷 ", "  Admin@Example.COM ")
	if err != nil {
		t.Fatal(err)
	}
	if user.Email != "admin@example.com" || user.Name != "李雷" || user.Status != StatusActive {
		t.Fatalf("user = %+v", user)
	}
	if len(password) != tempPasswordLen || strings.ContainsAny(password, "0Oo1lI") {
		t.Fatalf("password = %q", password)
	}
	var stored User
	if err := env.db.Gorm.Where("id = ?", user.ID).Take(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored.PasswordHash, password) {
		t.Fatal("password stored in clear")
	}
	if ok, _ := auth.VerifyPassword(stored.PasswordHash, password); !ok {
		t.Fatal("stored hash does not match the returned password")
	}

	other := mustCreate(t, env.orgs, 1)
	_, _, err = env.accounts.CreateUser(ctx, other.ID, "韩梅梅", "ADMIN@example.com")
	expectCode(t, err, bizerr.CodeConflict)
	_, _, err = env.accounts.CreateUser(ctx, uuid.New(), "x", "x@example.com")
	expectCode(t, err, bizerr.CodeNotFound)
	for _, bad := range []struct{ name, email string }{
		{" ", "a@example.com"},
		{"a", "not-an-email"},
		{"a", "Li <li@example.com>"},
		{"a", ""},
	} {
		_, _, err = env.accounts.CreateUser(ctx, acme.ID, bad.name, bad.email)
		expectCode(t, err, bizerr.CodeBadRequest)
	}
}

func TestUserOperationsStayInsideTheirOrg(t *testing.T) {
	env := newAccountsEnv(t)
	ctx := context.Background()
	acme := mustCreate(t, env.orgs, 1)
	other := mustCreate(t, env.orgs, 1)
	user, _ := env.mustCreateUser(t, acme.ID, "a@example.com")

	_, err := env.accounts.ResetPassword(ctx, other.ID, user.ID)
	expectCode(t, err, bizerr.CodeNotFound)
	expectCode(t, env.accounts.SetUserStatus(ctx, other.ID, user.ID, StatusDisabled), bizerr.CodeNotFound)

	users, err := env.accounts.ListUsers(ctx, other.ID)
	if err != nil || len(users) != 0 {
		t.Fatalf("other org users = %v, %v", users, err)
	}
	users, _ = env.accounts.ListUsers(ctx, acme.ID)
	if len(users) != 1 || users[0].ID != user.ID {
		t.Fatalf("acme users = %+v", users)
	}
	_, err = env.accounts.ListUsers(ctx, uuid.New())
	expectCode(t, err, bizerr.CodeNotFound)
}

func TestLoginAndPrincipal(t *testing.T) {
	env := newAccountsEnv(t)
	ctx := context.Background()
	acme := mustCreate(t, env.orgs, 1)
	user, password := env.mustCreateUser(t, acme.ID, "a@example.com")

	issued, got, err := env.accounts.Login(ctx, " A@Example.com ", password)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if got.OrgID != acme.ID {
		t.Fatalf("login org = %s, want %s", got.OrgID, acme.ID)
	}
	token, err := env.tokens.Lookup(ctx, auth.TokenTypeConsole, issued.Plain)
	if err != nil {
		t.Fatalf("issued token is not a console token: %v", err)
	}
	if _, err := env.tokens.Lookup(ctx, auth.TokenTypePlatform, issued.Plain); err == nil {
		t.Fatal("console token accepted as platform token")
	}
	admin, err := env.accounts.Principal(ctx, token)
	if err != nil || admin.OrgID != acme.ID || admin.OrgName != "Acme" || admin.ID != user.ID {
		t.Fatalf("Principal() = %+v, %v", admin, err)
	}

	_, _, err = env.accounts.Login(ctx, "a@example.com", "wrong-password")
	expectCode(t, err, bizerr.CodeInvalidCredentials)
	_, _, err = env.accounts.Login(ctx, "nobody@example.com", password)
	expectCode(t, err, bizerr.CodeInvalidCredentials)
}

func TestDisablingUserOrOrgRevokesAccess(t *testing.T) {
	env := newAccountsEnv(t)
	ctx := context.Background()
	acme := mustCreate(t, env.orgs, 1)
	user, password := env.mustCreateUser(t, acme.ID, "a@example.com")
	login := func() (auth.APIToken, error) {
		issued, _, err := env.accounts.Login(ctx, "a@example.com", password)
		if err != nil {
			return auth.APIToken{}, err
		}
		return env.tokens.Lookup(ctx, auth.TokenTypeConsole, issued.Plain)
	}

	token, err := login()
	if err != nil {
		t.Fatal(err)
	}
	if err := env.accounts.SetUserStatus(ctx, acme.ID, user.ID, StatusDisabled); err != nil {
		t.Fatal(err)
	}
	// Disabling deletes the token rows, so they stay dead after the admin is enabled again.
	var remaining int64
	env.db.Gorm.Model(&auth.APIToken{}).Where("principal_type = ? AND principal_id = ?", auth.TokenTypeConsole, user.ID.String()).Count(&remaining)
	if remaining != 0 {
		t.Fatalf("tokens after disable = %d, want 0", remaining)
	}
	_, err = env.accounts.Principal(ctx, token)
	expectCode(t, err, bizerr.CodeUnauthorized)
	_, err = login()
	expectCode(t, err, bizerr.CodeInvalidCredentials)

	if err := env.accounts.SetUserStatus(ctx, acme.ID, user.ID, StatusActive); err != nil {
		t.Fatal(err)
	}
	token, err = login()
	if err != nil {
		t.Fatalf("login after enable: %v", err)
	}

	// Disabling the organization keeps the token row but the principal check rejects it.
	if err := env.orgs.SetStatus(ctx, acme.ID, StatusDisabled); err != nil {
		t.Fatal(err)
	}
	_, err = env.accounts.Principal(ctx, token)
	expectCode(t, err, bizerr.CodeUnauthorized)
	_, err = login()
	expectCode(t, err, bizerr.CodeInvalidCredentials)
	if err := env.orgs.SetStatus(ctx, acme.ID, StatusActive); err != nil {
		t.Fatal(err)
	}
	if _, err := env.accounts.Principal(ctx, token); err != nil {
		t.Fatalf("token after re-enabling org: %v", err)
	}
}

func TestResetAndChangePassword(t *testing.T) {
	env := newAccountsEnv(t)
	ctx := context.Background()
	acme := mustCreate(t, env.orgs, 1)
	user, password := env.mustCreateUser(t, acme.ID, "a@example.com")
	issued, _, err := env.accounts.Login(ctx, "a@example.com", password)
	if err != nil {
		t.Fatal(err)
	}

	fresh, err := env.accounts.ResetPassword(ctx, acme.ID, user.ID)
	if err != nil || fresh == password {
		t.Fatalf("ResetPassword() = %q, %v", fresh, err)
	}
	if _, err := env.tokens.Lookup(ctx, auth.TokenTypeConsole, issued.Plain); err == nil {
		t.Fatal("token survived password reset")
	}
	_, _, err = env.accounts.Login(ctx, "a@example.com", password)
	expectCode(t, err, bizerr.CodeInvalidCredentials)
	issued, _, err = env.accounts.Login(ctx, "a@example.com", fresh)
	if err != nil {
		t.Fatalf("login with reset password: %v", err)
	}

	token, _ := env.tokens.Lookup(ctx, auth.TokenTypeConsole, issued.Plain)
	admin, _ := env.accounts.Principal(ctx, token)
	_, err = env.accounts.ChangePassword(ctx, admin, fresh, "short")
	expectCode(t, err, bizerr.CodePasswordTooShort)
	_, err = env.accounts.ChangePassword(ctx, admin, "wrong-password", "a-new-password")
	expectCode(t, err, bizerr.CodeCurrentPasswordWrong)
	_, err = env.accounts.ChangePassword(ctx, admin, fresh, fresh)
	expectCode(t, err, bizerr.CodePasswordUnchanged)

	changed, err := env.accounts.ChangePassword(ctx, admin, fresh, "a-new-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.tokens.Lookup(ctx, auth.TokenTypeConsole, issued.Plain); err == nil {
		t.Fatal("old token survived password change")
	}
	if _, err := env.tokens.Lookup(ctx, auth.TokenTypeConsole, changed.Plain); err != nil {
		t.Fatalf("new token rejected: %v", err)
	}
}

func TestOrgLoginRateLimited(t *testing.T) {
	env := newAccountsEnv(t)
	ctx := context.Background()
	acme := mustCreate(t, env.orgs, 1)
	_, password := env.mustCreateUser(t, acme.ID, "a@example.com")
	for range 3 {
		_, _, err := env.accounts.Login(ctx, "a@example.com", "wrong-password")
		expectCode(t, err, bizerr.CodeInvalidCredentials)
	}
	_, _, err := env.accounts.Login(ctx, "A@EXAMPLE.COM", password)
	expectCode(t, err, bizerr.CodeTooManyAttempts)
}
