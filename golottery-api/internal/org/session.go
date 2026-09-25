package org

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"golottery/api/internal/auth"
	"golottery/api/internal/bizerr"
)

// Admin is an active admin of an active organization, resolved from a console token.
type Admin struct {
	User
	OrgName string
}

// loginRow is an admin joined with the state of its organization.
type loginRow struct {
	User
	OrgName   string
	OrgStatus string
}

// Login checks admin credentials and issues a console token.
// Unknown emails, wrong passwords, disabled admins and disabled organizations all look the same.
func (a *Accounts) Login(ctx context.Context, email, password string) (auth.IssuedToken, User, error) {
	email = NormalizeEmail(email)
	if utf8.RuneCountInString(email) > maxEmailLen {
		auth.VerifyDummy(password)
		return auth.IssuedToken{}, User{}, bizerr.New(bizerr.CodeInvalidCredentials)
	}
	key := "org:" + email
	wait, err := a.limiter.Check(ctx, key)
	if err != nil {
		return auth.IssuedToken{}, User{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	if wait > 0 {
		return auth.IssuedToken{}, User{}, bizerr.New(bizerr.CodeTooManyAttempts, wait)
	}

	row, found, err := a.loginRow(ctx, "org_users.email = ?", email)
	if err != nil {
		return auth.IssuedToken{}, User{}, err
	}
	ok := false
	if !found {
		auth.VerifyDummy(password)
	} else {
		ok, err = auth.VerifyPassword(row.PasswordHash, password)
		if err != nil {
			return auth.IssuedToken{}, User{}, bizerr.Wrap(bizerr.CodeInternal, err)
		}
		ok = ok && row.Status == StatusActive && row.OrgStatus == StatusActive
	}
	if !ok {
		if err := a.limiter.Fail(ctx, key); err != nil {
			return auth.IssuedToken{}, User{}, bizerr.Wrap(bizerr.CodeInternal, err)
		}
		return auth.IssuedToken{}, User{}, bizerr.New(bizerr.CodeInvalidCredentials)
	}

	if err := a.limiter.Reset(ctx, key); err != nil {
		return auth.IssuedToken{}, User{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	issued, err := a.tokens.IssueConsole(ctx, row.ID.String(), auth.TokenNameLogin)
	if err != nil {
		return auth.IssuedToken{}, User{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return issued, row.User, nil
}

// Principal resolves a console token on every request. The admin and its
// organization must both still be active, so disabling either takes effect at once.
func (a *Accounts) Principal(ctx context.Context, token auth.APIToken) (Admin, error) {
	id, err := uuid.Parse(token.PrincipalID)
	if err != nil {
		return Admin{}, bizerr.New(bizerr.CodeUnauthorized)
	}
	row, found, err := a.loginRow(ctx, "org_users.id = ?", id)
	if err != nil {
		return Admin{}, err
	}
	if !found || row.Status != StatusActive || row.OrgStatus != StatusActive {
		return Admin{}, bizerr.New(bizerr.CodeUnauthorized)
	}
	return Admin{User: row.User, OrgName: row.OrgName}, nil
}

// Logout revokes one console token.
func (a *Accounts) Logout(ctx context.Context, token auth.APIToken) error {
	if err := a.tokens.Delete(ctx, token.ID); err != nil {
		return bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return nil
}

// ChangePassword replaces the admin password, revokes every token and issues a new one.
func (a *Accounts) ChangePassword(ctx context.Context, admin Admin, current, next string) (auth.IssuedToken, error) {
	if utf8.RuneCountInString(next) < auth.MinPasswordLen {
		return auth.IssuedToken{}, bizerr.New(bizerr.CodePasswordTooShort)
	}
	ok, err := auth.VerifyPassword(admin.PasswordHash, current)
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
	err = a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Model(&User{}).Where("id = ?", admin.ID).
			Updates(map[string]any{"password_hash": hash, "updated_at": a.now().UTC()}).Error
		if err != nil {
			return fmt.Errorf("org: update password: %w", err)
		}
		tokens := a.tokens.WithDB(tx)
		if err := tokens.DeleteByPrincipal(ctx, auth.TokenTypeConsole, admin.ID.String()); err != nil {
			return err
		}
		issued, err = tokens.IssueConsole(ctx, admin.ID.String(), auth.TokenNameLogin)
		return err
	})
	if err != nil {
		return auth.IssuedToken{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return issued, nil
}

func (a *Accounts) loginRow(ctx context.Context, where string, arg any) (loginRow, bool, error) {
	var rows []loginRow
	err := a.db.WithContext(ctx).Table("org_users").
		Select("org_users.*, orgs.name AS org_name, orgs.status AS org_status").
		Joins("JOIN orgs ON orgs.id = org_users.org_id").
		Where(where, arg).Limit(1).
		Scan(&rows).Error
	if err != nil {
		return loginRow{}, false, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	if len(rows) == 0 {
		return loginRow{}, false, nil
	}
	return rows[0], true, nil
}
