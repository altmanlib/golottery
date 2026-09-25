package apihttp

import (
	"bytes"
	"context"
	"net/url"

	api "golottery/api/api"
	"golottery/api/internal/event"
	"golottery/api/internal/guest"
)

// ListEventStaff lists an event's on-site staff for the admin console.
func (s *Server) ListEventStaff(ctx context.Context, request api.ListEventStaffRequestObject) (api.ListEventStaffResponseObject, error) {
	admin, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.guests.ListStaff(ctx, admin.OrgID, request.EventId)
	if err != nil {
		return nil, err
	}
	out := make(api.ListEventStaff200JSONResponse, len(rows))
	for i, r := range rows {
		out[i] = api.StaffMember{Id: r.ID, Role: api.StaffRole(r.Role), AttendeeName: r.AttendeeName, CreatedAt: r.CreatedAt}
	}
	return out, nil
}

// CreateStaffInvite issues a one-time staff invite and the web path that uses it.
func (s *Server) CreateStaffInvite(ctx context.Context, request api.CreateStaffInviteRequestObject) (api.CreateStaffInviteResponseObject, error) {
	admin, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	code, inv, err := s.guests.CreateInvite(ctx, admin.OrgID, request.EventId, string(request.Body.Role))
	if err != nil {
		return nil, err
	}
	view, err := s.events.Get(ctx, admin.OrgID, request.EventId)
	if err != nil {
		return nil, err
	}
	return api.CreateStaffInvite201JSONResponse{
		Code:      code,
		Role:      api.StaffRole(inv.Role),
		ExpiresAt: inv.ExpiresAt,
		Path:      "/#/m/" + view.PublicID + "/staff?invite=" + code,
	}, nil
}

// RemoveEventStaff revokes a staff member.
func (s *Server) RemoveEventStaff(ctx context.Context, request api.RemoveEventStaffRequestObject) (api.RemoveEventStaffResponseObject, error) {
	admin, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.guests.RemoveStaff(ctx, admin.OrgID, request.EventId, request.StaffId); err != nil {
		return nil, err
	}
	return api.RemoveEventStaff204Response{}, nil
}

// JoinStaff turns the guest into staff with an invite code.
func (s *Server) JoinStaff(ctx context.Context, request api.JoinStaffRequestObject) (api.JoinStaffResponseObject, error) {
	g, err := guestFrom(ctx)
	if err != nil {
		return nil, err
	}
	st, err := s.guests.Join(ctx, g, request.Body.Invite)
	if err != nil {
		return nil, err
	}
	return api.JoinStaff200JSONResponse(guestStatus(st)), nil
}

// GetStaffSummary returns check-in progress.
func (s *Server) GetStaffSummary(ctx context.Context, _ api.GetStaffSummaryRequestObject) (api.GetStaffSummaryResponseObject, error) {
	g, err := guestFrom(ctx)
	if err != nil {
		return nil, err
	}
	sum, err := s.guests.Summary(ctx, g)
	if err != nil {
		return nil, err
	}
	return api.GetStaffSummary200JSONResponse{Total: int(sum.Total), CheckedIn: int(sum.CheckedIn), PendingRequests: int(sum.PendingRequests)}, nil
}

// ListPendingRequests returns open help requests.
func (s *Server) ListPendingRequests(ctx context.Context, _ api.ListPendingRequestsRequestObject) (api.ListPendingRequestsResponseObject, error) {
	g, err := guestFrom(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.guests.PendingRequests(ctx, g)
	if err != nil {
		return nil, err
	}
	out := make(api.ListPendingRequests200JSONResponse, len(rows))
	for i, r := range rows {
		out[i] = api.PendingRequest{
			Id: r.ID, ClaimedName: r.ClaimedName, ClaimedPhoneLast4: r.ClaimedPhoneLast4,
			Reason: r.Reason, CreatedAt: r.CreatedAt,
		}
		if r.Attendee != nil {
			a := staffAttendee(*r.Attendee)
			out[i].Attendee = &a
		}
	}
	return out, nil
}

// ApproveRequest resolves a help request.
func (s *Server) ApproveRequest(ctx context.Context, request api.ApproveRequestRequestObject) (api.ApproveRequestResponseObject, error) {
	g, err := guestFrom(ctx)
	if err != nil {
		return nil, err
	}
	how := guest.Approval{AttendeeID: request.Body.AttendeeId}
	if c := request.Body.Create; c != nil {
		in := event.AttendeeInput{Name: c.Name, Phone: c.Phone}
		if c.Dept != nil {
			in.Dept = *c.Dept
		}
		how.Create = &in
	}
	if err := s.guests.ApproveRequest(ctx, g, request.RequestId, how); err != nil {
		return nil, err
	}
	return api.ApproveRequest204Response{}, nil
}

// RejectRequest closes a help request.
func (s *Server) RejectRequest(ctx context.Context, request api.RejectRequestRequestObject) (api.RejectRequestResponseObject, error) {
	g, err := guestFrom(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.guests.RejectRequest(ctx, g, request.RequestId); err != nil {
		return nil, err
	}
	return api.RejectRequest204Response{}, nil
}

// ProxyCheckin checks someone in on their behalf.
func (s *Server) ProxyCheckin(ctx context.Context, request api.ProxyCheckinRequestObject) (api.ProxyCheckinResponseObject, error) {
	g, err := guestFrom(ctx)
	if err != nil {
		return nil, err
	}
	a, err := s.guests.ProxyCheckin(ctx, g, request.Body.AttendeeId)
	if err != nil {
		return nil, err
	}
	return api.ProxyCheckin200JSONResponse(staffAttendee(a)), nil
}

// SearchStaffAttendees finds roster people by name.
func (s *Server) SearchStaffAttendees(ctx context.Context, request api.SearchStaffAttendeesRequestObject) (api.SearchStaffAttendeesResponseObject, error) {
	g, err := guestFrom(ctx)
	if err != nil {
		return nil, err
	}
	q := ""
	if request.Params.Q != nil {
		q = *request.Params.Q
	}
	rows, err := s.guests.SearchAttendees(ctx, g, q)
	if err != nil {
		return nil, err
	}
	out := make(api.SearchStaffAttendees200JSONResponse, len(rows))
	for i, r := range rows {
		out[i] = staffAttendee(r)
	}
	return out, nil
}

// UpdateStaffSettings lets an on-site admin change the check-in mode or fence.
func (s *Server) UpdateStaffSettings(ctx context.Context, request api.UpdateStaffSettingsRequestObject) (api.UpdateStaffSettingsResponseObject, error) {
	g, err := guestFrom(ctx)
	if err != nil {
		return nil, err
	}
	b := request.Body
	p := guest.SettingsPatch{CenterLat: b.CenterLat, CenterLng: b.CenterLng, RadiusM: b.RadiusM}
	if b.CheckinMode != nil {
		mode := string(*b.CheckinMode)
		p.CheckinMode = &mode
	}
	if _, err := s.guests.UpdateSettings(ctx, g, p); err != nil {
		return nil, err
	}
	st, err := s.guests.Status(ctx, g)
	if err != nil {
		return nil, err
	}
	return api.UpdateStaffSettings200JSONResponse(guestStatus(st)), nil
}

func staffAttendee(a event.Attendee) api.StaffAttendee {
	out := api.StaffAttendee{
		Id: a.ID, Name: a.Name, Dept: a.Dept, PhoneLast4: a.PhoneLast4,
		CheckedIn: a.Status == event.AttendeeCheckedIn, Bound: a.OpenID != nil,
	}
	if a.CheckinMethod != nil {
		m := api.CheckinMethod(*a.CheckinMethod)
		out.CheckinMethod = &m
	}
	return out
}

// ResetLiveData clears a trial run and logs what it removed.
func (s *Server) ResetLiveData(ctx context.Context, request api.ResetLiveDataRequestObject) (api.ResetLiveDataResponseObject, error) {
	admin, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	counts, err := s.guests.ResetLiveData(ctx, admin.OrgID, request.EventId, request.Body.ConfirmName)
	if err != nil {
		return nil, err
	}
	s.logger.Info("event live data reset", "event", request.EventId, "admin", admin.ID,
		"unbound", counts.Unbound, "attempts", counts.Attempts, "requests", counts.Requests, "sessions", counts.Sessions)
	return api.ResetLiveData200JSONResponse{
		Unbound: int(counts.Unbound), Attempts: int(counts.Attempts), Requests: int(counts.Requests), Sessions: int(counts.Sessions),
	}, nil
}

// ExportCheckinAttempts returns the check-in attempt workbook.
func (s *Server) ExportCheckinAttempts(ctx context.Context, request api.ExportCheckinAttemptsRequestObject) (api.ExportCheckinAttemptsResponseObject, error) {
	admin, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	book, name, err := s.guests.ExportAttempts(ctx, admin.OrgID, request.EventId)
	if err != nil {
		return nil, err
	}
	disposition := `attachment; filename="checkin-attempts.xlsx"; filename*=UTF-8''` + url.PathEscape(name+"-签到明细.xlsx")
	return api.ExportCheckinAttempts200ApplicationvndOpenxmlformatsOfficedocumentSpreadsheetmlSheetResponse{
		Body:          bytes.NewReader(book),
		ContentLength: int64(len(book)),
		Headers:       api.ExportCheckinAttempts200ResponseHeaders{ContentDisposition: &disposition},
	}, nil
}
