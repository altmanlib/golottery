package apihttp

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/xuri/excelize/v2"
)

func xlsx(t *testing.T, rows ...[]any) []byte {
	t.Helper()
	book := excelize.NewFile()
	sheet := book.GetSheetName(0)
	for i, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := book.SetSheetRow(sheet, cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := book.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func (e platformEnv) upload(t *testing.T, path, token string, file []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("file", "名单.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(file)
	_ = form.Close()
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set(echo.HeaderContentType, form.FormDataContentType())
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	rec := httptest.NewRecorder()
	e.engine.ServeHTTP(rec, req)
	return rec
}

func TestImportAndExportOverHTTP(t *testing.T) {
	env := newPlatformEnv(t)
	ops := env.mustLogin(t)
	console, _ := env.adminSession(t, ops, "a@example.com", 1)
	ev := env.createEvent(t, console)
	base := "/api/organization/events/" + ev.ID.String()
	header := []any{"姓名", "部门", "手机号"}

	rec := env.upload(t, base+"/attendees/import", console, xlsx(t, header, []any{"李雷", "研发", 13800001234}, []any{"", "", "1"}))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad import status = %d body = %s", rec.Code, rec.Body.String())
	}
	bad := decode[struct {
		Code string `json:"code"`
		Rows []struct {
			Row    int    `json:"row"`
			Reason string `json:"reason"`
		} `json:"rows"`
	}](t, rec)
	if bad.Code != "E_IMPORT_INVALID" || len(bad.Rows) != 1 || bad.Rows[0].Row != 3 || bad.Rows[0].Reason != "姓名为空" {
		t.Fatalf("bad import = %+v", bad)
	}

	rec = env.upload(t, base+"/attendees/import", console, []byte("not a workbook"))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "E_IMPORT_FILE") || !strings.Contains(rec.Body.String(), `"rows":[]`) {
		t.Fatalf("unreadable import status = %d body = %s", rec.Code, rec.Body.String())
	}

	rec = env.upload(t, base+"/attendees/import", console, xlsx(t, header, []any{"李雷", "研发", 13800001234}, []any{"韩梅梅", "", "5678"}))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"imported":2`) {
		t.Fatalf("import status = %d body = %s", rec.Code, rec.Body.String())
	}

	rec = env.do(t, http.MethodGet, base+"/exports/attendees", console, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("export status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("content type = %q", got)
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, "filename*=UTF-8''%E5%B9%B4%E4%BC%9A-%E5%90%8D%E5%8D%95.xlsx") {
		t.Fatalf("content disposition = %q", got)
	}
	book, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := book.GetRows(book.GetSheetName(0))
	if len(rows) != 3 || rows[1][2] != "1234" {
		t.Fatalf("exported rows = %v", rows)
	}

	mine, _ := env.adminSession(t, ops, "b@example.com", 1)
	expectError(t, env.do(t, http.MethodGet, base+"/exports/attendees", mine, nil), http.StatusNotFound, "E_NOT_FOUND")
	rec = env.upload(t, base+"/attendees/import", mine, xlsx(t, header, []any{"x", "", "1234"}))
	expectError(t, rec, http.StatusNotFound, "E_NOT_FOUND")
}
