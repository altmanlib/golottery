package apihttp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/url"

	api "golottery/api/api"
	"golottery/api/internal/bizerr"
	"golottery/api/internal/event"
)

// ImportAttendees appends an xlsx roster. Every failure caused by the file answers 400
// with the same body, so the page can list the rows to fix.
func (s *Server) ImportAttendees(ctx context.Context, request api.ImportAttendeesRequestObject) (api.ImportAttendeesResponseObject, error) {
	admin, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	file, err := uploadedFile(request.Body)
	if err != nil {
		return importFailure(bizerr.New(bizerr.CodeImportFile), nil), nil
	}
	imported, err := s.events.ImportAttendees(ctx, admin.OrgID, request.EventId, file)
	var ie *event.ImportError
	switch {
	case errors.As(err, &ie):
		return importFailure(ie.Err, ie.Rows), nil
	case err != nil:
		if be, ok := bizerr.As(err); ok && (be.Code == bizerr.CodeImportFile || be.Code == bizerr.CodeRosterFull) {
			return importFailure(be, nil), nil
		}
		return nil, err
	}
	return api.ImportAttendees200JSONResponse{Imported: imported}, nil
}

// uploadedFile returns the "file" part of the form, read up to the import limit.
func uploadedFile(form *multipart.Reader) (io.Reader, error) {
	if form == nil {
		return nil, errors.New("no form")
	}
	for {
		part, err := form.NextPart()
		if err != nil {
			return nil, err
		}
		if part.FormName() != "file" {
			continue
		}
		raw, err := io.ReadAll(io.LimitReader(part, event.MaxImportBytes+1))
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(raw), nil
	}
}

func importFailure(be *bizerr.Error, rows []event.RowError) api.ImportAttendees400JSONResponse {
	out := make([]api.ImportRowError, len(rows))
	for i, r := range rows {
		out[i] = api.ImportRowError{Row: r.Row, Reason: r.Reason}
	}
	return api.ImportAttendees400JSONResponse{Code: string(be.Code), Message: be.Message, Rows: out}
}

// ExportAttendees returns the roster workbook.
func (s *Server) ExportAttendees(ctx context.Context, request api.ExportAttendeesRequestObject) (api.ExportAttendeesResponseObject, error) {
	admin, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	book, name, err := s.events.ExportAttendees(ctx, admin.OrgID, request.EventId)
	if err != nil {
		return nil, err
	}
	disposition := `attachment; filename="attendees.xlsx"; filename*=UTF-8''` + url.PathEscape(name+"-名单.xlsx")
	return api.ExportAttendees200ApplicationvndOpenxmlformatsOfficedocumentSpreadsheetmlSheetResponse{
		Body:          bytes.NewReader(book),
		ContentLength: int64(len(book)),
		Headers:       api.ExportAttendees200ResponseHeaders{ContentDisposition: &disposition},
	}, nil
}
