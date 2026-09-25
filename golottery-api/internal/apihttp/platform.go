package apihttp

import (
	"context"

	api "golottery/api/api"
	"golottery/api/internal/auth"
)

// PlatformLogin checks operator credentials and returns a new platform token.
func (s *Server) PlatformLogin(ctx context.Context, request api.PlatformLoginRequestObject) (api.PlatformLoginResponseObject, error) {
	issued, err := s.platform.Login(ctx, request.Body.Username, request.Body.Password)
	if err != nil {
		return nil, err
	}
	return api.PlatformLogin200JSONResponse(sessionToken(issued)), nil
}

// PlatformLogout revokes the bearer token of the request.
func (s *Server) PlatformLogout(ctx context.Context, _ api.PlatformLogoutRequestObject) (api.PlatformLogoutResponseObject, error) {
	token, err := tokenFrom(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.platform.Logout(ctx, token); err != nil {
		return nil, err
	}
	return api.PlatformLogout204Response{}, nil
}

// GetPlatformMe returns the current operator.
func (s *Server) GetPlatformMe(ctx context.Context, _ api.GetPlatformMeRequestObject) (api.GetPlatformMeResponseObject, error) {
	token, err := tokenFrom(ctx)
	if err != nil {
		return nil, err
	}
	user, err := s.platform.Me(ctx, token)
	if err != nil {
		return nil, err
	}
	return api.GetPlatformMe200JSONResponse{Username: user.Username}, nil
}

// ChangePlatformPassword changes the password and returns the only token left for the operator.
func (s *Server) ChangePlatformPassword(ctx context.Context, request api.ChangePlatformPasswordRequestObject) (api.ChangePlatformPasswordResponseObject, error) {
	token, err := tokenFrom(ctx)
	if err != nil {
		return nil, err
	}
	issued, err := s.platform.ChangePassword(ctx, token, request.Body.CurrentPassword, request.Body.NewPassword)
	if err != nil {
		return nil, err
	}
	return api.ChangePlatformPassword200JSONResponse(sessionToken(issued)), nil
}

func sessionToken(issued auth.IssuedToken) api.SessionToken {
	return api.SessionToken{Token: issued.Plain, ExpiresAt: issued.ExpiresAt}
}
