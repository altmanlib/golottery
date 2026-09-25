package apihttp

import (
	"context"

	api "golottery/api/api"
	"golottery/api/internal/org"
)

const (
	defaultPageLimit = 40
	maxPageLimit     = 100
)

// ListOrgs returns one page of organizations.
func (s *Server) ListOrgs(ctx context.Context, request api.ListOrgsRequestObject) (api.ListOrgsResponseObject, error) {
	offset, limit := page(request.Params.Offset, request.Params.Limit)
	rows, total, err := s.orgs.List(ctx, offset, limit)
	if err != nil {
		return nil, err
	}
	items := make([]api.OrgSummary, len(rows))
	for i, row := range rows {
		items[i] = orgSummary(row)
	}
	return api.ListOrgs200JSONResponse{Items: items, Total: int(total)}, nil
}

// CreateOrg opens an organization.
func (s *Server) CreateOrg(ctx context.Context, request api.CreateOrgRequestObject) (api.CreateOrgResponseObject, error) {
	by, err := operatorFrom(ctx)
	if err != nil {
		return nil, err
	}
	body := request.Body
	in := org.CreateInput{Name: body.Name, EventCredits: body.EventCredits, MaxAttendees: body.MaxAttendees}
	if body.Contact != nil {
		in.Contact = *body.Contact
	}
	created, err := s.orgs.Create(ctx, in, by)
	if err != nil {
		return nil, err
	}
	return api.CreateOrg201JSONResponse(orgSummary(created)), nil
}

// GetOrg returns an organization with its latest ledger entries.
func (s *Server) GetOrg(ctx context.Context, request api.GetOrgRequestObject) (api.GetOrgResponseObject, error) {
	detail, err := s.orgs.Get(ctx, request.OrgId)
	if err != nil {
		return nil, err
	}
	ledger := make([]api.LedgerEntry, len(detail.Ledger))
	for i, entry := range detail.Ledger {
		ledger[i] = ledgerEntry(entry)
	}
	return api.GetOrg200JSONResponse{Org: orgSummary(detail.Summary), Ledger: ledger}, nil
}

// DisableOrg disables an organization.
func (s *Server) DisableOrg(ctx context.Context, request api.DisableOrgRequestObject) (api.DisableOrgResponseObject, error) {
	if err := s.orgs.SetStatus(ctx, request.OrgId, org.StatusDisabled); err != nil {
		return nil, err
	}
	return api.DisableOrg204Response{}, nil
}

// EnableOrg enables an organization.
func (s *Server) EnableOrg(ctx context.Context, request api.EnableOrgRequestObject) (api.EnableOrgResponseObject, error) {
	if err := s.orgs.SetStatus(ctx, request.OrgId, org.StatusActive); err != nil {
		return nil, err
	}
	return api.EnableOrg204Response{}, nil
}

// AdjustOrgCredits adds or removes event credits.
func (s *Server) AdjustOrgCredits(ctx context.Context, request api.AdjustOrgCreditsRequestObject) (api.AdjustOrgCreditsResponseObject, error) {
	by, err := operatorFrom(ctx)
	if err != nil {
		return nil, err
	}
	entry, err := s.orgs.AdjustCredits(ctx, request.OrgId, request.Body.Delta, request.Body.Reason, by)
	if err != nil {
		return nil, err
	}
	return api.AdjustOrgCredits200JSONResponse(ledgerEntry(entry)), nil
}

// SetOrgMaxAttendees changes the default attendee limit.
func (s *Server) SetOrgMaxAttendees(ctx context.Context, request api.SetOrgMaxAttendeesRequestObject) (api.SetOrgMaxAttendeesResponseObject, error) {
	if err := s.orgs.SetMaxAttendees(ctx, request.OrgId, request.Body.MaxAttendees); err != nil {
		return nil, err
	}
	return api.SetOrgMaxAttendees204Response{}, nil
}

func operatorFrom(ctx context.Context) (org.Operator, error) {
	token, err := tokenFrom(ctx)
	if err != nil {
		return org.Operator{}, err
	}
	return org.Operator{Type: token.PrincipalType, ID: token.PrincipalID}, nil
}

// page clamps list parameters instead of rejecting them.
func page(offset, limit *int) (int, int) {
	o, l := 0, defaultPageLimit
	if offset != nil && *offset > 0 {
		o = *offset
	}
	if limit != nil && *limit > 0 {
		l = min(*limit, maxPageLimit)
	}
	return o, l
}

func orgSummary(row org.Summary) api.OrgSummary {
	return api.OrgSummary{
		Id:           row.ID,
		Name:         row.Name,
		Contact:      row.Contact,
		Status:       api.OrgStatus(row.Status),
		EventCredits: row.EventCredits,
		MaxAttendees: row.MaxAttendees,
		CreatedAt:    row.CreatedAt,
	}
}

func ledgerEntry(entry org.LedgerEntry) api.LedgerEntry {
	return api.LedgerEntry{
		Id:           entry.ID,
		Delta:        entry.Delta,
		BalanceAfter: entry.BalanceAfter,
		Reason:       entry.Reason,
		EventId:      entry.EventID,
		OperatorType: entry.OperatorType,
		OperatorId:   entry.OperatorID,
		CreatedAt:    entry.CreatedAt,
	}
}
