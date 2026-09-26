package apihttp

import (
	"context"

	"github.com/labstack/echo/v4"

	api "golottery/api/api"
	"golottery/api/internal/bizerr"
	"golottery/api/internal/guest"
)

type clientIPKey struct{}

// withClientIP keeps echo's client IP (after TRUSTED_PROXIES) in the request context,
// where the web-mode bind limit reads it.
func withClientIP(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.SetRequest(c.Request().WithContext(context.WithValue(c.Request().Context(), clientIPKey{}, c.RealIP())))
		return next(c)
	}
}

func clientIP(ctx context.Context) string {
	ip, _ := ctx.Value(clientIPKey{}).(string)
	return ip
}

// GuestLogin signs a guest in for one event.
func (s *Server) GuestLogin(ctx context.Context, request api.GuestLoginRequestObject) (api.GuestLoginResponseObject, error) {
	if s.guests == nil {
		return nil, bizerr.New(bizerr.CodeNotFound)
	}
	b := request.Body
	in := guest.LoginInput{PublicID: b.PublicId}
	if b.Code != nil {
		in.Code = *b.Code
	}
	if b.DeviceId != nil {
		in.DeviceID = *b.DeviceId
	}
	issued, _, err := s.guests.Login(ctx, in)
	if err != nil {
		return nil, err
	}
	return api.GuestLogin200JSONResponse(sessionToken(issued)), nil
}

// GuestBind binds the guest to a roster person.
func (s *Server) GuestBind(ctx context.Context, request api.GuestBindRequestObject) (api.GuestBindResponseObject, error) {
	g, err := guestFrom(ctx)
	if err != nil {
		return nil, err
	}
	st, err := s.guests.Bind(ctx, g, request.Body.Name, request.Body.PhoneLast4, clientIP(ctx))
	if err != nil {
		return nil, err
	}
	return api.GuestBind200JSONResponse(guestStatus(st)), nil
}

// GetGuestStatus returns the guest page state.
func (s *Server) GetGuestStatus(ctx context.Context, _ api.GetGuestStatusRequestObject) (api.GetGuestStatusResponseObject, error) {
	g, err := guestFrom(ctx)
	if err != nil {
		return nil, err
	}
	st, err := s.guests.Status(ctx, g)
	if err != nil {
		return nil, err
	}
	return api.GetGuestStatus200JSONResponse(guestStatus(st)), nil
}

// GuestCheckin handles a check-in tap.
func (s *Server) GuestCheckin(ctx context.Context, request api.GuestCheckinRequestObject) (api.GuestCheckinResponseObject, error) {
	g, err := guestFrom(ctx)
	if err != nil {
		return nil, err
	}
	b := request.Body
	in := guest.CheckinInput{Lat: b.Lat, Lng: b.Lng, Accuracy: b.Accuracy}
	if b.CoordType != nil {
		in.CoordType = string(*b.CoordType)
	}
	st, err := s.guests.Checkin(ctx, g, in)
	if err != nil {
		return nil, err
	}
	return api.GuestCheckin200JSONResponse(guestStatus(st)), nil
}

// SubmitManualRequest asks staff for help.
func (s *Server) SubmitManualRequest(ctx context.Context, request api.SubmitManualRequestRequestObject) (api.SubmitManualRequestResponseObject, error) {
	g, err := guestFrom(ctx)
	if err != nil {
		return nil, err
	}
	b := request.Body
	in := guest.RequestInput{}
	if b.Name != nil {
		in.Name = *b.Name
	}
	if b.PhoneLast4 != nil {
		in.PhoneLast4 = *b.PhoneLast4
	}
	if b.Reason != nil {
		in.Reason = *b.Reason
	}
	st, err := s.guests.SubmitRequest(ctx, g, in)
	if err != nil {
		return nil, err
	}
	return api.SubmitManualRequest201JSONResponse(guestStatus(st)), nil
}

func guestStatus(st guest.Status) api.GuestStatus {
	out := api.GuestStatus{Event: api.GuestEvent{
		Name:         st.Event.Name,
		Status:       api.EventStatus(st.Event.Status),
		CheckinMode:  api.CheckinMode(st.Event.CheckinMode),
		CheckinStart: st.Event.CheckinStart,
		CheckinEnd:   st.Event.CheckinEnd,
	}}
	if a := st.Attendee; a != nil {
		ga := api.GuestAttendee{Name: a.Name, Dept: a.Dept, CheckedIn: a.Status == "checked_in", CheckinAt: a.CheckinAt}
		if a.CheckinMethod != nil {
			m := api.CheckinMethod(*a.CheckinMethod)
			ga.CheckinMethod = &m
		}
		out.Attendee = &ga
	}
	if r := st.Request; r != nil {
		out.Request = &api.GuestRequest{Status: api.GuestRequestStatus(r.Status), CreatedAt: r.CreatedAt}
	}
	if st.StaffRole != "" {
		role := api.StaffRole(st.StaffRole)
		out.StaffRole = &role
	}
	return out
}
