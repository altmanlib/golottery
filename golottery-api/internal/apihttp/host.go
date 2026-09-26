package apihttp

import (
	"bytes"
	"context"
	"net/http"
	"net/url"

	"github.com/labstack/echo/v4"

	api "golottery/api/api"
	"golottery/api/internal/auth"
	"golottery/api/internal/bizerr"
	"golottery/api/internal/draw"
)

type hostKey struct{}

func hostFrom(ctx context.Context) (draw.HostSession, error) {
	host, ok := ctx.Value(hostKey{}).(draw.HostSession)
	if !ok {
		return draw.HostSession{}, bizerr.New(bizerr.CodeUnauthorized)
	}
	return host, nil
}

func (s *Server) resolveHost(ctx context.Context, token auth.APIToken) (context.Context, error) {
	if s.draws == nil {
		return nil, bizerr.New(bizerr.CodeUnauthorized)
	}
	host, err := s.draws.ResolveHost(ctx, token)
	if err != nil {
		return nil, err
	}
	return context.WithValue(ctx, hostKey{}, host), nil
}

// UpsertEventHost creates or resets the host password of an event.
func (s *Server) UpsertEventHost(ctx context.Context, request api.UpsertEventHostRequestObject) (api.UpsertEventHostResponseObject, error) {
	admin, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	cred, err := s.draws.UpsertHost(ctx, admin.OrgID, request.EventId)
	if err != nil {
		return nil, err
	}
	return api.UpsertEventHost200JSONResponse{
		Password: cred.Password, PublicId: cred.PublicID, Path: cred.Path,
	}, nil
}

// HostLogin signs in as the host of one event.
func (s *Server) HostLogin(ctx context.Context, request api.HostLoginRequestObject) (api.HostLoginResponseObject, error) {
	issued, err := s.draws.Login(ctx, request.Body.EventPublicId, request.Body.Password)
	if err != nil {
		return nil, err
	}
	return api.HostLogin200JSONResponse{Token: issued.Plain, ExpiresAt: issued.ExpiresAt}, nil
}

// GetHostSnapshot returns the current draw state.
func (s *Server) GetHostSnapshot(ctx context.Context, _ api.GetHostSnapshotRequestObject) (api.GetHostSnapshotResponseObject, error) {
	host, err := hostFrom(ctx)
	if err != nil {
		return nil, err
	}
	snap, err := s.draws.Snapshot(ctx, host)
	if err != nil {
		return nil, err
	}
	return api.GetHostSnapshot200JSONResponse(toHostSnapshot(snap)), nil
}

// GetHostPool returns the people currently eligible for a draw.
func (s *Server) GetHostPool(ctx context.Context, _ api.GetHostPoolRequestObject) (api.GetHostPoolResponseObject, error) {
	host, err := hostFrom(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.draws.Pool(ctx, host)
	if err != nil {
		return nil, err
	}
	items := make([]api.HostPoolPerson, 0, len(rows))
	for _, row := range rows {
		items = append(items, api.HostPoolPerson{Id: row.ID, Name: row.Name, Dept: row.Dept})
	}
	return api.GetHostPool200JSONResponse{Items: items}, nil
}

// CreateDraw draws winners for one prize.
func (s *Server) CreateDraw(ctx context.Context, request api.CreateDrawRequestObject) (api.CreateDrawResponseObject, error) {
	host, err := hostFrom(ctx)
	if err != nil {
		return nil, err
	}
	batch, err := s.draws.Draw(ctx, host, request.Body.PrizeId, request.Body.Count, request.Body.RequestId)
	if err != nil {
		return nil, err
	}
	return api.CreateDraw200JSONResponse(toDrawBatch(batch)), nil
}

// VoidDrawResult marks a winner as void.
func (s *Server) VoidDrawResult(ctx context.Context, request api.VoidDrawResultRequestObject) (api.VoidDrawResultResponseObject, error) {
	host, err := hostFrom(ctx)
	if err != nil {
		return nil, err
	}
	row, err := s.draws.Void(ctx, host, request.ResultId, request.Body.Reason)
	if err != nil {
		return nil, err
	}
	return api.VoidDrawResult200JSONResponse(toDrawResult(row)), nil
}

// ExportWinners returns the winners workbook.
func (s *Server) ExportWinners(ctx context.Context, request api.ExportWinnersRequestObject) (api.ExportWinnersResponseObject, error) {
	admin, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	book, name, err := s.draws.ExportWinners(ctx, admin.OrgID, request.EventId)
	if err != nil {
		return nil, err
	}
	disposition := `attachment; filename="winners.xlsx"; filename*=UTF-8''` + url.PathEscape(name+"-中奖名单.xlsx")
	return api.ExportWinners200ApplicationvndOpenxmlformatsOfficedocumentSpreadsheetmlSheetResponse{
		Body: bytes.NewReader(book), ContentLength: int64(len(book)),
		Headers: api.ExportWinners200ResponseHeaders{ContentDisposition: &disposition},
	}, nil
}

// ExportDrawLog returns the draw log workbook.
func (s *Server) ExportDrawLog(ctx context.Context, request api.ExportDrawLogRequestObject) (api.ExportDrawLogResponseObject, error) {
	admin, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	book, name, err := s.draws.ExportLogs(ctx, admin.OrgID, request.EventId)
	if err != nil {
		return nil, err
	}
	disposition := `attachment; filename="draw-log.xlsx"; filename*=UTF-8''` + url.PathEscape(name+"-抽奖日志.xlsx")
	return api.ExportDrawLog200ApplicationvndOpenxmlformatsOfficedocumentSpreadsheetmlSheetResponse{
		Body: bytes.NewReader(book), ContentLength: int64(len(book)),
		Headers: api.ExportDrawLog200ResponseHeaders{ContentDisposition: &disposition},
	}, nil
}

func (s *Server) mountHostStream(engine *echo.Echo) {
	engine.GET("/api/host/stream", s.hostStream)
}

func (s *Server) hostStream(c echo.Context) error {
	plain, ok := bearerToken(c.Request().Header.Get(echo.HeaderAuthorization))
	if !ok || s.draws == nil || s.tokens == nil {
		return writeBizErr(c, bizerr.New(bizerr.CodeUnauthorized))
	}
	token, err := s.tokens.Lookup(c.Request().Context(), auth.TokenTypeHost, plain)
	if err != nil {
		return writeBizErr(c, bizerr.New(bizerr.CodeUnauthorized))
	}
	host, err := s.draws.ResolveHost(c.Request().Context(), token)
	if err != nil {
		if be, ok := bizerr.As(err); ok {
			return writeBizErr(c, be)
		}
		return writeBizErr(c, bizerr.New(bizerr.CodeUnauthorized))
	}

	snap, err := s.draws.Snapshot(c.Request().Context(), host)
	if err != nil {
		return err
	}
	events, cancel := s.draws.Hub().Subscribe(host.EventID)
	defer cancel()

	c.Response().Header().Set(echo.HeaderContentType, "text/event-stream")
	c.Response().Header().Set(echo.HeaderCacheControl, "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().Header().Set("X-Accel-Buffering", "no")
	c.Response().WriteHeader(http.StatusOK)

	if err := writeSSE(c, "snapshot", toHostSnapshot(snap)); err != nil {
		return nil
	}
	if flusher, ok := c.Response().Writer.(http.Flusher); ok {
		flusher.Flush()
	}

	ctx := c.Request().Context()
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-events:
			if !ok {
				return nil
			}
			var payload any
			switch msg.Type {
			case "stats":
				payload = map[string]any{"checked_in": msg.CheckedIn, "draw_version": msg.DrawVersion}
			case "draw", "void":
				payload = map[string]any{
					"draw_version": msg.DrawVersion,
					"results":      toDrawResults(msg.Results),
				}
			default:
				payload = msg
			}
			if err := writeSSE(c, msg.Type, payload); err != nil {
				return nil
			}
			if flusher, ok := c.Response().Writer.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}

func writeSSE(c echo.Context, eventType string, payload any) error {
	body, err := draw.EncodeSSE(eventType, payload)
	if err != nil {
		return err
	}
	_, err = c.Response().Write(body)
	return err
}

func toHostSnapshot(snap draw.Snapshot) api.HostSnapshot {
	prizes := make([]api.HostPrize, 0, len(snap.Prizes))
	for _, p := range snap.Prizes {
		prizes = append(prizes, api.HostPrize{
			Id: p.ID, Name: p.Name, Gift: p.Gift, Quota: p.Quota, Remaining: p.Remaining, SortNo: p.SortNo,
		})
	}
	return api.HostSnapshot{
		EventName: snap.EventName, PublicId: snap.PublicID, Status: api.EventStatus(snap.Status),
		CheckedIn: int(snap.CheckedIn), DrawVersion: snap.DrawVersion,
		Prizes: prizes, Winners: toDrawResults(snap.Winners),
	}
}

func toDrawBatch(batch draw.Batch) api.DrawBatch {
	return api.DrawBatch{
		RequestId: batch.RequestID, DrawVersion: batch.DrawVersion, Results: toDrawResults(batch.Results),
	}
}

func toDrawResults(rows []draw.ResultView) []api.DrawResult {
	out := make([]api.DrawResult, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDrawResult(row))
	}
	return out
}

func toDrawResult(row draw.ResultView) api.DrawResult {
	out := api.DrawResult{
		Id: row.ID, PrizeId: row.PrizeID, PrizeName: row.PrizeName,
		AttendeeId: row.AttendeeID, AttendeeName: row.AttendeeName, AttendeeDept: row.AttendeeDept,
		Status: api.DrawResultStatus(row.Status), CreatedAt: row.CreatedAt,
	}
	if row.VoidReason != nil {
		out.VoidReason = row.VoidReason
	}
	if row.VoidedAt != nil {
		t := row.VoidedAt.UTC()
		out.VoidedAt = &t
	}
	return out
}
