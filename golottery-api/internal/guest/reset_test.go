package guest

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"golottery/api/internal/bizerr"
	"golottery/api/internal/event"
	"golottery/api/internal/org"
)

// trialRun leaves bindings, a check-in, attempts, a request and sessions behind.
func (e env) trialRun(t *testing.T) Guest {
	t.Helper()
	ctx := context.Background()
	g := e.bound(t, "device-aaaaaaaaaaaaaaaa", "李雷", "1000")
	if _, err := e.guests.Checkin(ctx, g, CheckinInput{Lat: ptr(gpsLat), Lng: ptr(gpsLng), Accuracy: ptr(10.0), CoordType: CoordWGS84}); err != nil {
		t.Fatal(err)
	}
	stranger := e.login(t, "device-bbbbbbbbbbbbbbbb")
	if _, err := e.guests.SubmitRequest(ctx, stranger, RequestInput{Name: "赵六", PhoneLast4: "7777"}); err != nil {
		t.Fatal(err)
	}
	return g
}

func (e env) moveWindow(t *testing.T, start time.Time) {
	t.Helper()
	end := start.Add(3 * time.Hour)
	if _, err := e.events.Update(context.Background(), e.orgID, e.ev.ID, event.Patch{CheckinStart: &start, CheckinEnd: &end}, org.Operator{}); err != nil {
		t.Fatal(err)
	}
}

func TestResetBeforeCheckinOpens(t *testing.T) {
	e := newEnv(t, ModeWeb)
	ctx := context.Background()
	staff := e.staffGuest(t, "device-ssssssssssssssss", RoleStaff)
	g := e.trialRun(t)

	// The window is open, so this is real data now.
	_, err := e.guests.ResetLiveData(ctx, e.orgID, e.ev.ID, "年会")
	expectCode(t, err, bizerr.CodeConflict)
	draft := event.StatusDraft
	_, err = e.events.Update(ctx, e.orgID, e.ev.ID, event.Patch{Status: &draft}, org.Operator{})
	expectCode(t, err, bizerr.CodeConflict)
	var li event.Attendee
	e.db.Gorm.Where("name = ?", "李雷").Take(&li)
	expectCode(t, e.events.DeleteAttendee(ctx, e.orgID, e.ev.ID, li.ID), bizerr.CodeConflict)

	e.moveWindow(t, time.Now().Add(24*time.Hour))
	_, err = e.guests.ResetLiveData(ctx, e.orgID, e.ev.ID, "别的活动")
	expectCode(t, err, bizerr.CodeBadRequest)
	if _, err = e.guests.ResetLiveData(ctx, e.orgID, e.ev.ID, " 年会 "); err != nil {
		t.Fatalf("trimmed name refused: %v", err)
	}
	counts, err := e.guests.ResetLiveData(ctx, e.orgID, e.ev.ID, "年会")
	if err != nil || counts != (ResetCounts{}) {
		t.Fatalf("second reset = %+v, %v", counts, err)
	}
	st, err := e.guests.Status(ctx, g)
	if err != nil || st.Attendee != nil || st.Request != nil {
		t.Fatalf("after reset = %+v, %v", st, err)
	}
	var sessions, attempts, attendees int64
	e.db.Gorm.Model(&Session{}).Count(&sessions)
	e.db.Gorm.Model(&Attempt{}).Count(&attempts)
	e.db.Gorm.Model(&event.Attendee{}).Where("event_id = ?", e.ev.ID).Count(&attendees)
	if sessions != 0 || attempts != 0 || attendees != 3 {
		t.Fatalf("sessions %d attempts %d attendees %d", sessions, attempts, attendees)
	}
	if role, _ := e.guests.staffRole(ctx, staff); role != RoleStaff {
		t.Fatal("reset removed staff")
	}
	if _, err := e.events.Update(ctx, e.orgID, e.ev.ID, event.Patch{Status: &draft}, org.Operator{}); err != nil {
		t.Fatalf("draft after reset: %v", err)
	}
	if err := e.events.DeleteAttendee(ctx, e.orgID, e.ev.ID, li.ID); err != nil {
		t.Fatalf("delete after reset: %v", err)
	}
}

func TestResetCountsAndIsolation(t *testing.T) {
	e := newEnv(t, ModeWeb)
	ctx := context.Background()
	e.trialRun(t)
	e.moveWindow(t, time.Now().Add(time.Hour))
	counts, err := e.guests.ResetLiveData(ctx, e.orgID, e.ev.ID, "年会")
	if err != nil {
		t.Fatal(err)
	}
	// 李雷 was bound and checked in; two guests had sessions; one tap and one request.
	if counts.Unbound != 1 || counts.Attempts != 1 || counts.Requests != 1 || counts.Sessions != 2 {
		t.Fatalf("counts = %+v", counts)
	}
	other, err := org.NewService(e.db.Gorm).Create(ctx, org.CreateInput{Name: "别家", EventCredits: 1, MaxAttendees: 10}, org.Operator{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.guests.ResetLiveData(ctx, other.ID, e.ev.ID, "年会")
	expectCode(t, err, bizerr.CodeNotFound)
}

func TestExportsCarryCheckins(t *testing.T) {
	e := newEnv(t, ModeWeb)
	ctx := context.Background()
	staff := e.bound(t, "device-ssssssssssssssss", "王五", "1002")
	code, _, _ := e.guests.CreateInvite(ctx, e.orgID, e.ev.ID, RoleStaff)
	if _, err := e.guests.Join(ctx, staff, code); err != nil {
		t.Fatal(err)
	}
	e.trialRun(t)
	var hmm event.Attendee
	e.db.Gorm.Where("name = ?", "韩梅梅").Take(&hmm)
	if _, err := e.guests.ProxyCheckin(ctx, staff, hmm.ID); err != nil {
		t.Fatal(err)
	}

	raw, _, err := e.events.ExportAttendees(ctx, e.orgID, e.ev.ID)
	if err != nil {
		t.Fatal(err)
	}
	book, _ := excelize.OpenReader(bytes.NewReader(raw))
	rows, _ := book.GetRows(book.GetSheetName(0))
	byName := map[string][]string{}
	for _, r := range rows[1:] {
		byName[r[0]] = r
	}
	if r := byName["李雷"]; len(r) < 7 || r[3] != "已签到" || r[6] != "定位" {
		t.Fatalf("李雷 row = %v", r)
	}
	if r := byName["韩梅梅"]; len(r) < 8 || r[6] != "代签到" || r[7] != "王五" {
		t.Fatalf("韩梅梅 row = %v", r)
	}

	raw, _, err = e.guests.ExportAttempts(ctx, e.orgID, e.ev.ID)
	if err != nil {
		t.Fatal(err)
	}
	book, _ = excelize.OpenReader(bytes.NewReader(raw))
	rows, _ = book.GetRows(book.GetSheetName(0))
	if len(rows) != 2 || rows[1][1] != "李雷" || rows[1][2] != "通过" || rows[1][4] == "" {
		t.Fatalf("attempt rows = %v", rows)
	}
}
