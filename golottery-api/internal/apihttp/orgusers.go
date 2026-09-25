package apihttp

import (
	"context"

	api "golottery/api/api"
	"golottery/api/internal/org"
)

// ListOrgUsers returns the admins of an organization.
func (s *Server) ListOrgUsers(ctx context.Context, request api.ListOrgUsersRequestObject) (api.ListOrgUsersResponseObject, error) {
	users, err := s.accounts.ListUsers(ctx, request.OrgId)
	if err != nil {
		return nil, err
	}
	out := make(api.ListOrgUsers200JSONResponse, len(users))
	for i, user := range users {
		out[i] = orgUser(user)
	}
	return out, nil
}

// CreateOrgUser creates an admin and returns its one-time password.
func (s *Server) CreateOrgUser(ctx context.Context, request api.CreateOrgUserRequestObject) (api.CreateOrgUserResponseObject, error) {
	user, password, err := s.accounts.CreateUser(ctx, request.OrgId, request.Body.Name, request.Body.Email)
	if err != nil {
		return nil, err
	}
	return api.CreateOrgUser201JSONResponse{User: orgUser(user), Password: password}, nil
}

// ResetOrgUserPassword returns a new one-time password.
func (s *Server) ResetOrgUserPassword(ctx context.Context, request api.ResetOrgUserPasswordRequestObject) (api.ResetOrgUserPasswordResponseObject, error) {
	password, err := s.accounts.ResetPassword(ctx, request.OrgId, request.UserId)
	if err != nil {
		return nil, err
	}
	return api.ResetOrgUserPassword200JSONResponse{Password: password}, nil
}

// DisableOrgUser disables an admin and revokes its tokens.
func (s *Server) DisableOrgUser(ctx context.Context, request api.DisableOrgUserRequestObject) (api.DisableOrgUserResponseObject, error) {
	if err := s.accounts.SetUserStatus(ctx, request.OrgId, request.UserId, org.StatusDisabled); err != nil {
		return nil, err
	}
	return api.DisableOrgUser204Response{}, nil
}

// EnableOrgUser enables an admin.
func (s *Server) EnableOrgUser(ctx context.Context, request api.EnableOrgUserRequestObject) (api.EnableOrgUserResponseObject, error) {
	if err := s.accounts.SetUserStatus(ctx, request.OrgId, request.UserId, org.StatusActive); err != nil {
		return nil, err
	}
	return api.EnableOrgUser204Response{}, nil
}

func orgUser(user org.User) api.OrgUser {
	return api.OrgUser{
		Id:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		Status:    api.OrgUserStatus(user.Status),
		CreatedAt: user.CreatedAt,
	}
}
