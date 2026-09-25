package event

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"golottery/api/internal/bizerr"
)

// workbook builds an xlsx file; the first row is the header.
func workbook(t *testing.T, rows ...[]any) *bytes.Reader {
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
	return bytes.NewReader(buf.Bytes())
}

var header = []any{"姓名", "部门", "手机号"}

func expectRows(t *testing.T, err error, want ...string) {
	t.Helper()
	var ie *ImportError
	if !errors.As(err, &ie) {
		t.Fatalf("error = %v, want ImportError", err)
	}
	var got []string
	for _, r := range ie.Rows {
		got = append(got, fmt.Sprintf("%02d %s", r.Row, r.Reason))
	}
	if strings.Join(got, "; ") != strings.Join(want, "; ") {
		t.Fatalf("rows = %v, want %v", got, want)
	}
}

func TestImportAppendsInFileOrder(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	orgID := e.newOrg(t, 1)
	ev := e.newEvent(t, orgID)
	e.db.Gorm.Model(&Event{}).Where("id = ?", ev.ID).Update("max_attendees", 10)

	n, err := e.events.ImportAttendees(ctx, orgID, ev.ID, workbook(t, header,
		[]any{"李雷", "研发", 13800001234},
		[]any{"", "", ""},
		[]any{" 韩梅梅 ", "", "+86 139-0000-5678"},
	))
	if err != nil || n != 2 {
		t.Fatalf("import = %d, %v", n, err)
	}
	rows, total, _ := e.events.ListAttendees(ctx, orgID, ev.ID, 0, 40)
	if total != 2 || rows[0].Name != "李雷" || rows[0].PhoneLast4 != "1234" || rows[0].Dept != "研发" || rows[1].Name != "韩梅梅" || rows[1].PhoneLast4 != "5678" {
		t.Fatalf("roster = %+v", rows)
	}

	// A number format must not leak into the phone digits (13800009876.00 would give 7600).
	styled := excelize.NewFile()
	sheet := styled.GetSheetName(0)
	_ = styled.SetSheetRow(sheet, "A1", &header)
	_ = styled.SetSheetRow(sheet, "A2", &[]any{"格式", "", 13800009876})
	style, _ := styled.NewStyle(&excelize.Style{NumFmt: 2})
	_ = styled.SetCellStyle(sheet, "C2", "C2", style)
	var buf bytes.Buffer
	_ = styled.Write(&buf)
	if _, err := e.events.ImportAttendees(ctx, orgID, ev.ID, bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatal(err)
	}
	rows, _, _ = e.events.ListAttendees(ctx, orgID, ev.ID, 0, 40)
	if rows[2].PhoneLast4 != "9876" {
		t.Fatalf("styled phone last4 = %q, want 9876", rows[2].PhoneLast4)
	}

	// Columns may come in any order and 部门 is optional.
	n, err = e.events.ImportAttendees(ctx, orgID, ev.ID, workbook(t, []any{"手机号", "姓名"}, []any{"0001", "王五"}))
	if err != nil || n != 1 {
		t.Fatalf("second import = %d, %v", n, err)
	}
}

func TestImportRejectsTheWholeFile(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	orgID := e.newOrg(t, 1)
	ev := e.newEvent(t, orgID)
	e.db.Gorm.Model(&Event{}).Where("id = ?", ev.ID).Update("max_attendees", 10)
	if _, err := e.events.AddAttendee(ctx, orgID, ev.ID, AttendeeInput{Name: "已有", Phone: "9999"}); err != nil {
		t.Fatal(err)
	}

	_, err := e.events.ImportAttendees(ctx, orgID, ev.ID, workbook(t, header,
		[]any{"李雷", "", "13800001234"},
		[]any{"", "", "13800001111"},
		[]any{"韩梅梅", "", "12"},
		[]any{"李雷", "", "1234"},
		[]any{"已有", "", "139 9999"},
	))
	expectRows(t, err, "03 姓名为空", "04 手机号不足四位", "05 与第 2 行重复")
	be, _ := bizerr.As(err)
	if be.Code != bizerr.CodeImportInvalid || be.Message != "有 3 行需要修正，整份文件未导入" {
		t.Fatalf("error = %+v", be)
	}

	_, err = e.events.ImportAttendees(ctx, orgID, ev.ID, workbook(t, header, []any{"新人", "", "1111"}, []any{"已有", "", "9999"}))
	expectRows(t, err, "03 与已有名单重复")

	_, total, _ := e.events.ListAttendees(ctx, orgID, ev.ID, 0, 40)
	if total != 1 {
		t.Fatalf("rejected imports wrote rows: total = %d", total)
	}
}

func TestImportRespectsTheLimitAcrossImports(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	orgID := e.newOrg(t, 1)
	ev := e.newEvent(t, orgID)

	if _, err := e.events.ImportAttendees(ctx, orgID, ev.ID, workbook(t, header, []any{"a", "", "0001"}, []any{"b", "", "0002"})); err != nil {
		t.Fatal(err)
	}
	_, err := e.events.ImportAttendees(ctx, orgID, ev.ID, workbook(t, header, []any{"c", "", "0003"}, []any{"d", "", "0004"}))
	expectCode(t, err, bizerr.CodeRosterFull)
	if be, _ := bizerr.As(err); be.Message != "名单不能超过人数上限 3 人" {
		t.Fatalf("message = %q", be.Message)
	}
	if _, err := e.events.ImportAttendees(ctx, orgID, ev.ID, workbook(t, header, []any{"c", "", "0003"})); err != nil {
		t.Fatalf("import up to the limit: %v", err)
	}
}

func TestImportRejectsUnreadableFiles(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	orgID := e.newOrg(t, 1)
	ev := e.newEvent(t, orgID)
	cases := map[string]*bytes.Reader{
		"not xlsx":      bytes.NewReader([]byte("姓名,手机号\n李雷,1234\n")),
		"empty":         bytes.NewReader(nil),
		"no phone col":  workbook(t, []any{"姓名", "部门"}, []any{"李雷", "研发"}),
		"only header":   workbook(t, header),
		"over the size": bytes.NewReader(make([]byte, MaxImportBytes+1)),
	}
	for name, file := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := e.events.ImportAttendees(ctx, orgID, ev.ID, file)
			expectCode(t, err, bizerr.CodeImportFile)
		})
	}
}

func TestExportAttendees(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	orgID := e.newOrg(t, 1)
	ev := e.newEvent(t, orgID)
	e.events.now = func() time.Time { return time.Date(2026, 9, 30, 16, 30, 0, 0, time.UTC) }
	if _, err := e.events.AddAttendee(ctx, orgID, ev.ID, AttendeeInput{Name: "李雷", Dept: "研发", Phone: "13800001234"}); err != nil {
		t.Fatal(err)
	}

	raw, name, err := e.events.ExportAttendees(ctx, orgID, ev.ID)
	if err != nil || name != "年会" {
		t.Fatalf("export = %q, %v", name, err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := book.GetRows(book.GetSheetName(0))
	want := [][]string{{"姓名", "部门", "手机后四位", "状态", "加入时间"}, {"李雷", "研发", "1234", "未签到", "2026-10-01 00:30"}}
	if len(rows) != 2 || strings.Join(rows[0], ",") != strings.Join(want[0], ",") || strings.Join(rows[1], ",") != strings.Join(want[1], ",") {
		t.Fatalf("rows = %v", rows)
	}

	other := e.newOrg(t, 1)
	_, _, err = e.events.ExportAttendees(ctx, other, ev.ID)
	expectCode(t, err, bizerr.CodeNotFound)
}

func TestImportRejectsValidWorkbooksOverTheLimit(t *testing.T) {
	e := newEnv(t)
	orgID := e.newOrg(t, 1)
	ev := e.newEvent(t, orgID)

	// Random text barely compresses, so a few hundred full cells push the file past 5 MB.
	book := excelize.NewFile()
	sheet := book.GetSheetName(0)
	_ = book.SetSheetRow(sheet, "A1", &header)
	noise := make([]byte, 24000)
	for i := 2; i < 300; i++ {
		_, _ = rand.Read(noise)
		_ = book.SetSheetRow(sheet, fmt.Sprintf("A%d", i), &[]any{base64.StdEncoding.EncodeToString(noise)[:30000], "", "1234"})
	}
	var buf bytes.Buffer
	if err := book.Write(&buf); err != nil {
		t.Fatal(err)
	}
	if buf.Len() <= MaxImportBytes {
		t.Fatalf("test workbook is only %d bytes", buf.Len())
	}
	_, err := e.events.ImportAttendees(context.Background(), orgID, ev.ID, bytes.NewReader(buf.Bytes()))
	expectCode(t, err, bizerr.CodeImportFile)
}
