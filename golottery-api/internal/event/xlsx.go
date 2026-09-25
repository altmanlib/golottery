package event

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"
	_ "time/tzdata" // exports use Asia/Shanghai even when the image has no zoneinfo
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"golottery/api/internal/bizerr"
)

// MaxImportBytes is the largest roster file accepted.
const MaxImportBytes = 5 << 20

const (
	headerName  = "姓名"
	headerDept  = "部门"
	headerPhone = "手机号"
)

// displayZone is the zone for times shown to admins and written to exports.
var displayZone = mustLoadZone("Asia/Shanghai")

func mustLoadZone(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// RowError explains why one spreadsheet row was rejected. Row counts from 1, the header row.
type RowError struct {
	Row    int
	Reason string
}

// ImportError rejects a whole import and lists the rows to fix.
type ImportError struct {
	Err  *bizerr.Error
	Rows []RowError
}

func (e *ImportError) Error() string { return e.Err.Error() }

// Unwrap exposes the business error so bizerr.As finds it.
func (e *ImportError) Unwrap() error { return e.Err }

// ImportAttendees appends every row of an xlsx roster, or none of them.
func (s *Service) ImportAttendees(ctx context.Context, orgID, eventID uuid.UUID, file io.Reader) (int, error) {
	rows, err := readRoster(file)
	if err != nil {
		return 0, err
	}
	imported := 0
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ev, err := editableEvent(ctx, tx, orgID, eventID)
		if err != nil {
			return err
		}
		var existing []Attendee
		if err := tx.Select("name", "phone_last4").Where("event_id = ?", eventID).Find(&existing).Error; err != nil {
			return fmt.Errorf("event: load roster: %w", err)
		}
		taken := make(map[string]bool, len(existing))
		for _, a := range existing {
			taken[a.Name+"\x00"+a.PhoneLast4] = true
		}
		var problems []RowError
		for _, r := range rows {
			if taken[r.key()] {
				problems = append(problems, RowError{Row: r.row, Reason: "与已有名单重复"})
			}
		}
		if len(problems) > 0 {
			return importError(problems)
		}
		if len(existing)+len(rows) > ev.MaxAttendees {
			return bizerr.New(bizerr.CodeRosterFull, ev.MaxAttendees)
		}
		now := s.now().UTC()
		batch := make([]Attendee, len(rows))
		for i, r := range rows {
			// Rows keep file order: created_at ties are broken by a microsecond step.
			batch[i] = Attendee{
				ID: uuid.New(), OrgID: orgID, EventID: eventID,
				Name: r.name, Dept: r.dept, PhoneLast4: r.phone,
				Status: AttendeePending, CreatedAt: now.Add(time.Duration(i) * time.Microsecond),
			}
		}
		if err := tx.CreateInBatches(batch, 500).Error; err != nil {
			return fmt.Errorf("event: insert roster: %w", err)
		}
		imported = len(batch)
		return nil
	})
	if err != nil {
		if ie, ok := err.(*ImportError); ok {
			return 0, ie
		}
		return 0, asBizErr(err)
	}
	return imported, nil
}

type rosterRow struct {
	row               int
	name, dept, phone string
}

func (r rosterRow) key() string { return r.name + "\x00" + r.phone }

func importError(rows []RowError) *ImportError {
	return &ImportError{Err: bizerr.New(bizerr.CodeImportInvalid, len(rows)), Rows: rows}
}

// readRoster parses and checks the file on its own, before any database work.
func readRoster(file io.Reader) ([]rosterRow, error) {
	limited := io.LimitReader(file, MaxImportBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil || len(raw) == 0 || len(raw) > MaxImportBytes {
		return nil, bizerr.New(bizerr.CodeImportFile)
	}
	book, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		return nil, bizerr.New(bizerr.CodeImportFile)
	}
	defer func() { _ = book.Close() }()
	sheets := book.GetSheetList()
	if len(sheets) == 0 {
		return nil, bizerr.New(bizerr.CodeImportFile)
	}
	// Raw values keep long phone numbers as digits instead of 1.38E+10.
	cells, err := book.GetRows(sheets[0], excelize.Options{RawCellValue: true})
	if err != nil || len(cells) == 0 {
		return nil, bizerr.New(bizerr.CodeImportFile)
	}

	col := map[string]int{}
	for i, h := range cells[0] {
		col[strings.TrimSpace(h)] = i
	}
	nameCol, okName := col[headerName]
	phoneCol, okPhone := col[headerPhone]
	deptCol, okDept := col[headerDept]
	if !okName || !okPhone {
		return nil, bizerr.New(bizerr.CodeImportFile)
	}
	cell := func(row []string, i int) string {
		if i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}

	var rows []rosterRow
	var problems []RowError
	seen := map[string]int{}
	for i, row := range cells[1:] {
		n := i + 2
		name, phone := cell(row, nameCol), cell(row, phoneCol)
		dept := ""
		if okDept {
			dept = cell(row, deptCol)
		}
		if name == "" && phone == "" && dept == "" {
			continue
		}
		last4, okLast4 := PhoneLast4(phone)
		switch {
		case name == "":
			problems = append(problems, RowError{Row: n, Reason: "姓名为空"})
		case utf8.RuneCountInString(name) > maxNameLen:
			problems = append(problems, RowError{Row: n, Reason: "姓名超过 100 字"})
		case utf8.RuneCountInString(dept) > maxDeptLen:
			problems = append(problems, RowError{Row: n, Reason: "部门超过 100 字"})
		case !okLast4:
			problems = append(problems, RowError{Row: n, Reason: "手机号不足四位"})
		default:
			r := rosterRow{row: n, name: name, dept: dept, phone: last4}
			if first, dup := seen[r.key()]; dup {
				problems = append(problems, RowError{Row: n, Reason: fmt.Sprintf("与第 %d 行重复", first)})
				continue
			}
			seen[r.key()] = n
			rows = append(rows, r)
		}
	}
	if len(problems) > 0 {
		return nil, importError(problems)
	}
	if len(rows) == 0 {
		return nil, bizerr.New(bizerr.CodeImportFile)
	}
	return rows, nil
}

// attendeeStatusLabels names roster statuses in exports; phase 6 adds check-in states.
var attendeeStatusLabels = map[string]string{AttendeePending: "未签到"}

// ExportAttendees writes the roster as an xlsx workbook and returns it with the event name.
func (s *Service) ExportAttendees(ctx context.Context, orgID, eventID uuid.UUID) ([]byte, string, error) {
	view, err := s.Get(ctx, orgID, eventID)
	if err != nil {
		return nil, "", err
	}
	var rows []Attendee
	if err := s.db.WithContext(ctx).Where("event_id = ?", eventID).Order("created_at, id").Find(&rows).Error; err != nil {
		return nil, "", bizerr.Wrap(bizerr.CodeInternal, err)
	}

	book := excelize.NewFile()
	defer func() { _ = book.Close() }()
	sheet := book.GetSheetName(0)
	header := []any{headerName, headerDept, "手机后四位", "状态", "加入时间"}
	if err := book.SetSheetRow(sheet, "A1", &header); err != nil {
		return nil, "", bizerr.Wrap(bizerr.CodeInternal, err)
	}
	for i, a := range rows {
		status := attendeeStatusLabels[a.Status]
		if status == "" {
			status = a.Status
		}
		line := []any{a.Name, a.Dept, a.PhoneLast4, status, a.CreatedAt.In(displayZone).Format("2006-01-02 15:04")}
		cellRef, _ := excelize.CoordinatesToCellName(1, i+2)
		if err := book.SetSheetRow(sheet, cellRef, &line); err != nil {
			return nil, "", bizerr.Wrap(bizerr.CodeInternal, err)
		}
	}
	var out bytes.Buffer
	if err := book.Write(&out); err != nil {
		return nil, "", bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return out.Bytes(), view.Name, nil
}
