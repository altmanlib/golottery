package apihttp

import (
	"context"

	api "golottery/api/api"
	"golottery/api/internal/auth"
	"golottery/api/internal/event"
	"golottery/api/internal/org"
)

// adminOperator records console changes, such as a credit used by an event, against the admin.
func adminOperator(admin org.Admin) org.Operator {
	return org.Operator{Type: auth.TokenTypeConsole, ID: admin.ID.String()}
}

// ListEvents returns one page of the organization's events.
func (s *Server) ListEvents(ctx context.Context, request api.ListEventsRequestObject) (api.ListEventsResponseObject, error) {
	admin, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	offset, limit := page(request.Params.Offset, request.Params.Limit)
	rows, total, err := s.events.List(ctx, admin.OrgID, offset, limit)
	if err != nil {
		return nil, err
	}
	items := make([]api.Event, len(rows))
	for i, row := range rows {
		items[i] = eventBody(row)
	}
	return api.ListEvents200JSONResponse{Items: items, Total: int(total)}, nil
}

// CreateEvent creates a draft event.
func (s *Server) CreateEvent(ctx context.Context, request api.CreateEventRequestObject) (api.CreateEventResponseObject, error) {
	admin, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	created, err := s.events.Create(ctx, admin.OrgID, request.Body.Name)
	if err != nil {
		return nil, err
	}
	return api.CreateEvent201JSONResponse(eventBody(created)), nil
}

// GetEvent returns one event.
func (s *Server) GetEvent(ctx context.Context, request api.GetEventRequestObject) (api.GetEventResponseObject, error) {
	admin, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	view, err := s.events.Get(ctx, admin.OrgID, request.EventId)
	if err != nil {
		return nil, err
	}
	return api.GetEvent200JSONResponse(eventBody(view)), nil
}

// UpdateEvent changes settings or status.
func (s *Server) UpdateEvent(ctx context.Context, request api.UpdateEventRequestObject) (api.UpdateEventResponseObject, error) {
	admin, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	b := request.Body
	p := event.Patch{
		Name: b.Name, CenterLat: b.CenterLat, CenterLng: b.CenterLng, RadiusM: b.RadiusM,
		CheckinStart: b.CheckinStart, CheckinEnd: b.CheckinEnd, AllowMultiWin: b.AllowMultiWin,
	}
	if b.CheckinMode != nil {
		mode := string(*b.CheckinMode)
		p.CheckinMode = &mode
	}
	if b.Status != nil {
		status := string(*b.Status)
		p.Status = &status
	}
	view, err := s.events.Update(ctx, admin.OrgID, request.EventId, p, adminOperator(admin))
	if err != nil {
		return nil, err
	}
	return api.UpdateEvent200JSONResponse(eventBody(view)), nil
}

// GetEventEntry returns the public code and mini program path of an event.
func (s *Server) GetEventEntry(ctx context.Context, request api.GetEventEntryRequestObject) (api.GetEventEntryResponseObject, error) {
	admin, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	view, err := s.events.Get(ctx, admin.OrgID, request.EventId)
	if err != nil {
		return nil, err
	}
	return api.GetEventEntry200JSONResponse{PublicId: view.PublicID, Path: event.EntryPath(view.PublicID)}, nil
}

// ListPrizes returns an event's prizes.
func (s *Server) ListPrizes(ctx context.Context, request api.ListPrizesRequestObject) (api.ListPrizesResponseObject, error) {
	admin, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.events.ListPrizes(ctx, admin.OrgID, request.EventId)
	if err != nil {
		return nil, err
	}
	out := make(api.ListPrizes200JSONResponse, len(rows))
	for i, row := range rows {
		out[i] = prizeBody(row)
	}
	return out, nil
}

// CreatePrize adds a prize.
func (s *Server) CreatePrize(ctx context.Context, request api.CreatePrizeRequestObject) (api.CreatePrizeResponseObject, error) {
	admin, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	b := request.Body
	in := event.PrizeInput{Name: b.Name, Quota: b.Quota, SortNo: b.SortNo}
	if b.Gift != nil {
		in.Gift = *b.Gift
	}
	row, err := s.events.AddPrize(ctx, admin.OrgID, request.EventId, in)
	if err != nil {
		return nil, err
	}
	return api.CreatePrize201JSONResponse(prizeBody(row)), nil
}

// UpdatePrize changes a prize.
func (s *Server) UpdatePrize(ctx context.Context, request api.UpdatePrizeRequestObject) (api.UpdatePrizeResponseObject, error) {
	admin, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	b := request.Body
	row, err := s.events.UpdatePrize(ctx, admin.OrgID, request.EventId, request.PrizeId,
		event.PrizePatch{Name: b.Name, Gift: b.Gift, Quota: b.Quota, SortNo: b.SortNo})
	if err != nil {
		return nil, err
	}
	return api.UpdatePrize200JSONResponse(prizeBody(row)), nil
}

// DeletePrize removes a prize.
func (s *Server) DeletePrize(ctx context.Context, request api.DeletePrizeRequestObject) (api.DeletePrizeResponseObject, error) {
	admin, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.events.DeletePrize(ctx, admin.OrgID, request.EventId, request.PrizeId); err != nil {
		return nil, err
	}
	return api.DeletePrize204Response{}, nil
}

// ListAttendees returns one page of the roster.
func (s *Server) ListAttendees(ctx context.Context, request api.ListAttendeesRequestObject) (api.ListAttendeesResponseObject, error) {
	admin, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	offset, limit := page(request.Params.Offset, request.Params.Limit)
	rows, total, err := s.events.ListAttendees(ctx, admin.OrgID, request.EventId, offset, limit)
	if err != nil {
		return nil, err
	}
	items := make([]api.Attendee, len(rows))
	for i, row := range rows {
		items[i] = attendeeBody(row)
	}
	return api.ListAttendees200JSONResponse{Items: items, Total: int(total)}, nil
}

// CreateAttendee adds one person to the roster.
func (s *Server) CreateAttendee(ctx context.Context, request api.CreateAttendeeRequestObject) (api.CreateAttendeeResponseObject, error) {
	admin, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	b := request.Body
	in := event.AttendeeInput{Name: b.Name, Phone: b.Phone}
	if b.Dept != nil {
		in.Dept = *b.Dept
	}
	row, err := s.events.AddAttendee(ctx, admin.OrgID, request.EventId, in)
	if err != nil {
		return nil, err
	}
	return api.CreateAttendee201JSONResponse(attendeeBody(row)), nil
}

// UpdateAttendee changes a roster row.
func (s *Server) UpdateAttendee(ctx context.Context, request api.UpdateAttendeeRequestObject) (api.UpdateAttendeeResponseObject, error) {
	admin, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	b := request.Body
	row, err := s.events.UpdateAttendee(ctx, admin.OrgID, request.EventId, request.AttendeeId,
		event.AttendeePatch{Name: b.Name, Dept: b.Dept, Phone: b.Phone})
	if err != nil {
		return nil, err
	}
	return api.UpdateAttendee200JSONResponse(attendeeBody(row)), nil
}

// DeleteAttendee removes a roster row.
func (s *Server) DeleteAttendee(ctx context.Context, request api.DeleteAttendeeRequestObject) (api.DeleteAttendeeResponseObject, error) {
	admin, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.events.DeleteAttendee(ctx, admin.OrgID, request.EventId, request.AttendeeId); err != nil {
		return nil, err
	}
	return api.DeleteAttendee204Response{}, nil
}

func eventBody(v event.View) api.Event {
	return api.Event{
		Id:             v.ID,
		PublicId:       v.PublicID,
		Name:           v.Name,
		Status:         api.EventStatus(v.Status),
		CheckinMode:    api.CheckinMode(v.CheckinMode),
		CenterLat:      v.CenterLat,
		CenterLng:      v.CenterLng,
		RadiusM:        v.RadiusM,
		CheckinStart:   v.CheckinStart,
		CheckinEnd:     v.CheckinEnd,
		AllowMultiWin:  v.AllowMultiWin,
		MaxAttendees:   v.MaxAttendees,
		AttendeeCount:  v.AttendeeCount,
		PrizeCount:     v.PrizeCount,
		CreditConsumed: v.CreditConsumedAt != nil,
		CreatedAt:      v.CreatedAt,
	}
}

func prizeBody(p event.Prize) api.Prize {
	return api.Prize{Id: p.ID, Name: p.Name, Gift: p.Gift, Quota: p.Quota, SortNo: p.SortNo}
}

func attendeeBody(a event.Attendee) api.Attendee {
	return api.Attendee{Id: a.ID, Name: a.Name, Dept: a.Dept, PhoneLast4: a.PhoneLast4, Status: a.Status, CreatedAt: a.CreatedAt}
}

// ListOrgEvents lets an operator see an organization's events.
func (s *Server) ListOrgEvents(ctx context.Context, request api.ListOrgEventsRequestObject) (api.ListOrgEventsResponseObject, error) {
	if _, err := s.orgs.Get(ctx, request.OrgId); err != nil {
		return nil, err
	}
	offset, limit := page(request.Params.Offset, request.Params.Limit)
	rows, total, err := s.events.List(ctx, request.OrgId, offset, limit)
	if err != nil {
		return nil, err
	}
	items := make([]api.Event, len(rows))
	for i, row := range rows {
		items[i] = eventBody(row)
	}
	return api.ListOrgEvents200JSONResponse{Items: items, Total: int(total)}, nil
}

// SetEventMaxAttendees changes one event's attendee limit on behalf of the organization.
func (s *Server) SetEventMaxAttendees(ctx context.Context, request api.SetEventMaxAttendeesRequestObject) (api.SetEventMaxAttendeesResponseObject, error) {
	view, err := s.events.SetMaxAttendees(ctx, request.OrgId, request.EventId, request.Body.MaxAttendees)
	if err != nil {
		return nil, err
	}
	return api.SetEventMaxAttendees200JSONResponse(eventBody(view)), nil
}
