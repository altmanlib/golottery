package apihttp

import (
	"context"

	api "golottery/api/api"
	"golottery/api/internal/auth"
	"golottery/api/internal/bizerr"
	"golottery/api/internal/org"
)

type adminKey struct{}

// resolveAdmin turns a console token into its admin; the organization comes from the
// admin row, never from the client.
func (s *Server) resolveAdmin(ctx context.Context, token auth.APIToken) (context.Context, error) {
	if s.accounts == nil {
		return nil, bizerr.New(bizerr.CodeUnauthorized)
	}
	admin, err := s.accounts.Principal(ctx, token)
	if err != nil {
		return nil, err
	}
	return context.WithValue(ctx, adminKey{}, admin), nil
}

// adminFrom returns the admin authenticated for the current request.
func adminFrom(ctx context.Context) (org.Admin, error) {
	admin, ok := ctx.Value(adminKey{}).(org.Admin)
	if !ok {
		return org.Admin{}, bizerr.New(bizerr.CodeUnauthorized)
	}
	return admin, nil
}

// OrganizationLogin checks admin credentials and returns a console token.
func (s *Server) OrganizationLogin(ctx context.Context, request api.OrganizationLoginRequestObject) (api.OrganizationLoginResponseObject, error) {
	issued, user, err := s.accounts.Login(ctx, request.Body.Email, request.Body.Password)
	if err != nil {
		return nil, err
	}
	return api.OrganizationLogin200JSONResponse{Token: issued.Plain, ExpiresAt: issued.ExpiresAt, OrgId: user.OrgID}, nil
}

// OrganizationLogout revokes the bearer token of the request.
func (s *Server) OrganizationLogout(ctx context.Context, _ api.OrganizationLogoutRequestObject) (api.OrganizationLogoutResponseObject, error) {
	token, err := tokenFrom(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.accounts.Logout(ctx, token); err != nil {
		return nil, err
	}
	return api.OrganizationLogout204Response{}, nil
}

// GetOrganizationMe returns the current admin and its organization.
func (s *Server) GetOrganizationMe(ctx context.Context, _ api.GetOrganizationMeRequestObject) (api.GetOrganizationMeResponseObject, error) {
	admin, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	credits, err := org.Credits(ctx, s.db, admin.OrgID)
	if err != nil {
		return nil, err
	}
	return api.GetOrganizationMe200JSONResponse{Name: admin.Name, Email: admin.Email, OrgId: admin.OrgID, OrgName: admin.OrgName, EventCredits: credits}, nil
}

// ChangeOrganizationPassword changes the password and returns the only token left for the admin.
func (s *Server) ChangeOrganizationPassword(ctx context.Context, request api.ChangeOrganizationPasswordRequestObject) (api.ChangeOrganizationPasswordResponseObject, error) {
	admin, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	issued, err := s.accounts.ChangePassword(ctx, admin, request.Body.CurrentPassword, request.Body.NewPassword)
	if err != nil {
		return nil, err
	}
	return api.ChangeOrganizationPassword200JSONResponse(sessionToken(issued)), nil
}
