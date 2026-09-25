package event

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"golottery/api/internal/bizerr"
	"golottery/api/internal/org"
	"golottery/api/internal/store"
)

var admin = org.Operator{Type: "console", ID: "admin-1"}

type env struct {
	events *Service
	orgs   *org.Service
	db     *store.DB
}

func newEnv(t *testing.T) env {
	t.Helper()
	db := store.OpenTest(t)
	store.Reset(t, db, &Prize{}, &Attendee{}, &org.LedgerEntry{}, &Event{}, &org.User{}, &org.Quota{}, &org.Org{})
	return env{events: NewService(db.Gorm), orgs: org.NewService(db.Gorm), db: db}
}

func (e env) newOrg(t *testing.T, credits int) uuid.UUID {
	t.Helper()
	created, err := e.orgs.Create(context.Background(), org.CreateInput{Name: "Acme", EventCredits: credits, MaxAttendees: 3}, org.Operator{Type: "platform", ID: "ops"})
	if err != nil {
		t.Fatal(err)
	}
	return created.ID
}

func (e env) newEvent(t *testing.T, orgID uuid.UUID) View {
	t.Helper()
	ev, err := e.events.Create(context.Background(), orgID, " 年会 ")
	if err != nil {
		t.Fatal(err)
	}
	return ev
}

// readyable fills in everything a geo event needs before it can become ready.
func (e env) readyable(t *testing.T, orgID, id uuid.UUID) {
	t.Helper()
	start := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	end := start.Add(3 * time.Hour)
	lat, lng := 39.908823, 116.397470
	if _, err := e.events.Update(context.Background(), orgID, id, Patch{CheckinStart: &start, CheckinEnd: &end, CenterLat: &lat, CenterLng: &lng}, admin); err != nil {
		t.Fatal(err)
	}
	if _, err := e.events.AddAttendee(context.Background(), orgID, id, AttendeeInput{Name: "李雷", Phone: "13800001234"}); err != nil {
		t.Fatal(err)
	}
}

func setStatus(e env, orgID, id uuid.UUID, status string) (View, error) {
	return e.events.Update(context.Background(), orgID, id, Patch{Status: &status}, admin)
}

func expectCode(t *testing.T, err error, code bizerr.Code) {
	t.Helper()
	be, ok := bizerr.As(err)
	if !ok || be.Code != code {
		t.Fatalf("error = %v, want %s", err, code)
	}
}

func credits(t *testing.T, e env, orgID uuid.UUID) int {
	t.Helper()
	n, err := org.Credits(context.Background(), e.db.Gorm, orgID)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCreateUsesNoCreditAndCopiesLimit(t *testing.T) {
	e := newEnv(t)
	orgID := e.newOrg(t, 1)
	ev := e.newEvent(t, orgID)
	if ev.Name != "年会" || ev.Status != StatusDraft || ev.CheckinMode != ModeGeo || ev.RadiusM != DefaultRadius || ev.MaxAttendees != 3 {
		t.Fatalf("event = %+v", ev.Event)
	}
	if len(ev.PublicID) != publicIDLen || strings.Trim(ev.PublicID, publicIDAlphabet) != "" {
		t.Fatalf("public id = %q", ev.PublicID)
	}
	if credits(t, e, orgID) != 1 {
		t.Fatal("creating an event used a credit")
	}
	_, err := e.events.Create(context.Background(), orgID, " ")
	expectCode(t, err, bizerr.CodeNameRequired)
}

func TestFirstReadyUsesOneCredit(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	orgID := e.newOrg(t, 2)
	ev := e.newEvent(t, orgID)
	e.readyable(t, orgID, ev.ID)

	ready, err := setStatus(e, orgID, ev.ID, StatusReady)
	if err != nil {
		t.Fatal(err)
	}
	if ready.Status != StatusReady || ready.CreditConsumedAt == nil || credits(t, e, orgID) != 1 {
		t.Fatalf("after ready: status %s consumed %v credits %d", ready.Status, ready.CreditConsumedAt, credits(t, e, orgID))
	}
	var entries []org.LedgerEntry
	e.db.Gorm.Where("event_id = ?", ev.ID).Find(&entries)
	if len(entries) != 1 || entries[0].Delta != -1 || entries[0].Reason != org.EventReadyReason || entries[0].OperatorType != "console" {
		t.Fatalf("ledger = %+v", entries)
	}

	if _, err := setStatus(e, orgID, ev.ID, StatusDraft); err != nil {
		t.Fatal(err)
	}
	if _, err := setStatus(e, orgID, ev.ID, StatusReady); err != nil {
		t.Fatal(err)
	}
	if credits(t, e, orgID) != 1 {
		t.Fatal("going back to draft and ready again charged another credit")
	}
	if _, err := setStatus(e, orgID, ev.ID, StatusClosed); err != nil {
		t.Fatal(err)
	}
	if credits(t, e, orgID) != 1 {
		t.Fatal("closing refunded or charged a credit")
	}
	_, err = e.events.Get(ctx, orgID, ev.ID)
	if err != nil {
		t.Fatal(err)
	}
}

func TestReadyWithoutCreditsIsRefused(t *testing.T) {
	e := newEnv(t)
	orgID := e.newOrg(t, 0)
	ev := e.newEvent(t, orgID)
	e.readyable(t, orgID, ev.ID)

	_, err := setStatus(e, orgID, ev.ID, StatusReady)
	expectCode(t, err, bizerr.CodeNoEventCredits)
	got, _ := e.events.Get(context.Background(), orgID, ev.ID)
	if got.Status != StatusDraft || got.CreditConsumedAt != nil {
		t.Fatalf("event after refusal = %+v", got.Event)
	}
}

func TestConcurrentReadyCannotOverdraw(t *testing.T) {
	e := newEnv(t)
	orgID := e.newOrg(t, 1)
	a := e.newEvent(t, orgID)
	b := e.newEvent(t, orgID)
	e.readyable(t, orgID, a.ID)
	e.readyable(t, orgID, b.ID)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, id := range []uuid.UUID{a.ID, b.ID} {
		wg.Go(func() { _, errs[i] = setStatus(e, orgID, id, StatusReady) })
	}
	wg.Wait()
	succeeded := 0
	for _, err := range errs {
		if err == nil {
			succeeded++
		} else {
			expectCode(t, err, bizerr.CodeNoEventCredits)
		}
	}
	if succeeded != 1 || credits(t, e, orgID) != 0 {
		t.Fatalf("succeeded = %d credits = %d, want 1 and 0", succeeded, credits(t, e, orgID))
	}
}

func TestReadyRequirements(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	orgID := e.newOrg(t, 5)
	ev := e.newEvent(t, orgID)

	_, err := setStatus(e, orgID, ev.ID, StatusReady)
	expectCode(t, err, bizerr.CodeEventIncomplete)
	if be, _ := bizerr.As(err); be.Message != "就绪前请补全：签到时间窗、签到中心点、名单" {
		t.Fatalf("message = %q", be.Message)
	}

	direct := ModeDirect
	start := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	if _, err := e.events.Update(ctx, orgID, ev.ID, Patch{CheckinMode: &direct, CheckinStart: &start, CheckinEnd: &end}, admin); err != nil {
		t.Fatal(err)
	}
	if _, err := e.events.AddAttendee(ctx, orgID, ev.ID, AttendeeInput{Name: "李雷", Phone: "1234"}); err != nil {
		t.Fatal(err)
	}
	if _, err := setStatus(e, orgID, ev.ID, StatusReady); err != nil {
		t.Fatalf("direct event without a fence could not become ready: %v", err)
	}

	geo := ModeGeo
	_, err = e.events.Update(ctx, orgID, ev.ID, Patch{CheckinMode: &geo}, admin)
	expectCode(t, err, bizerr.CodeEventIncomplete)
	got, _ := e.events.Get(ctx, orgID, ev.ID)
	if got.CheckinMode != ModeDirect {
		t.Fatal("rejected switch to geo was saved")
	}
}

func TestTransitionsAndClosed(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	orgID := e.newOrg(t, 5)
	ev := e.newEvent(t, orgID)

	_, err := setStatus(e, orgID, ev.ID, StatusClosed)
	expectCode(t, err, bizerr.CodeConflict)
	bogus := "archived"
	_, err = e.events.Update(ctx, orgID, ev.ID, Patch{Status: &bogus}, admin)
	expectCode(t, err, bizerr.CodeConflict)

	e.readyable(t, orgID, ev.ID)
	if _, err := setStatus(e, orgID, ev.ID, StatusReady); err != nil {
		t.Fatal(err)
	}
	if _, err := setStatus(e, orgID, ev.ID, StatusClosed); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{StatusDraft, StatusReady} {
		_, err = setStatus(e, orgID, ev.ID, status)
		expectCode(t, err, bizerr.CodeConflict)
	}
	name := "改名"
	_, err = e.events.Update(ctx, orgID, ev.ID, Patch{Name: &name}, admin)
	expectCode(t, err, bizerr.CodeConflict)
	_, err = e.events.AddAttendee(ctx, orgID, ev.ID, AttendeeInput{Name: "韩梅梅", Phone: "5678"})
	expectCode(t, err, bizerr.CodeConflict)
	_, err = e.events.AddPrize(ctx, orgID, ev.ID, PrizeInput{Name: "一等奖", Quota: 1, SortNo: 1})
	expectCode(t, err, bizerr.CodeConflict)
}

func TestFieldValidation(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	orgID := e.newOrg(t, 1)
	ev := e.newEvent(t, orgID)
	start := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	before := start.Add(-time.Hour)
	bad := []Patch{
		{RadiusM: new(99)},
		{RadiusM: new(1001)},
		{CenterLat: new(91.0)},
		{CenterLng: new(-181.0)},
		{CheckinStart: &start, CheckinEnd: &before},
		{CheckinMode: new("walk")},
	}
	for _, p := range bad {
		_, err := e.events.Update(ctx, orgID, ev.ID, p, admin)
		expectCode(t, err, bizerr.CodeBadRequest)
	}
	lat := 31.230416
	got, err := e.events.Update(ctx, orgID, ev.ID, Patch{CenterLat: &lat, RadiusM: new(100)}, admin)
	if err != nil || got.CenterLat == nil || *got.CenterLat != lat || got.RadiusM != 100 {
		t.Fatalf("update = %+v, %v", got.Event, err)
	}
}

func TestOtherOrgsEventsAreNotFound(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	mine := e.newOrg(t, 1)
	theirs := e.newOrg(t, 1)
	ev := e.newEvent(t, theirs)

	_, err := e.events.Get(ctx, mine, ev.ID)
	expectCode(t, err, bizerr.CodeNotFound)
	_, err = setStatus(e, mine, ev.ID, StatusReady)
	expectCode(t, err, bizerr.CodeNotFound)
	_, err = e.events.AddAttendee(ctx, mine, ev.ID, AttendeeInput{Name: "x", Phone: "1234"})
	expectCode(t, err, bizerr.CodeNotFound)
	_, err = e.events.ListPrizes(ctx, mine, ev.ID)
	expectCode(t, err, bizerr.CodeNotFound)
	rows, total, _ := e.events.List(ctx, mine, 0, 40)
	if total != 0 || len(rows) != 0 {
		t.Fatalf("list leaked %d events", total)
	}
}

func TestRosterRules(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	orgID := e.newOrg(t, 1)
	ev := e.newEvent(t, orgID)

	first, err := e.events.AddAttendee(ctx, orgID, ev.ID, AttendeeInput{Name: " 李雷 ", Dept: " 研发 ", Phone: "138-0000-1234"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Name != "李雷" || first.Dept != "研发" || first.PhoneLast4 != "1234" {
		t.Fatalf("attendee = %+v", first)
	}
	_, err = e.events.AddAttendee(ctx, orgID, ev.ID, AttendeeInput{Name: "李雷", Phone: "9991234"})
	expectCode(t, err, bizerr.CodeConflict)
	_, err = e.events.AddAttendee(ctx, orgID, ev.ID, AttendeeInput{Name: "x", Phone: "123"})
	expectCode(t, err, bizerr.CodeBadRequest)
	_, err = e.events.AddAttendee(ctx, orgID, ev.ID, AttendeeInput{Name: " ", Phone: "1234"})
	expectCode(t, err, bizerr.CodeNameRequired)

	second, _ := e.events.AddAttendee(ctx, orgID, ev.ID, AttendeeInput{Name: "韩梅梅", Phone: "5678"})
	if _, err := e.events.AddAttendee(ctx, orgID, ev.ID, AttendeeInput{Name: "王五", Phone: "0000"}); err != nil {
		t.Fatal(err)
	}
	_, err = e.events.AddAttendee(ctx, orgID, ev.ID, AttendeeInput{Name: "赵六", Phone: "1111"})
	expectCode(t, err, bizerr.CodeRosterFull)

	name := "李雷"
	phone := "1234"
	_, err = e.events.UpdateAttendee(ctx, orgID, ev.ID, second.ID, AttendeePatch{Name: &name, Phone: &phone})
	expectCode(t, err, bizerr.CodeConflict)
	dept := "市场"
	updated, err := e.events.UpdateAttendee(ctx, orgID, ev.ID, second.ID, AttendeePatch{Dept: &dept})
	if err != nil || updated.Dept != "市场" || updated.Name != "韩梅梅" {
		t.Fatalf("update = %+v, %v", updated, err)
	}

	if err := e.events.DeleteAttendee(ctx, orgID, ev.ID, second.ID); err != nil {
		t.Fatal(err)
	}
	expectCode(t, e.events.DeleteAttendee(ctx, orgID, ev.ID, second.ID), bizerr.CodeNotFound)
	rows, total, _ := e.events.ListAttendees(ctx, orgID, ev.ID, 0, 40)
	if total != 2 || rows[0].ID != first.ID {
		t.Fatalf("roster = %+v", rows)
	}
}

func TestPrizeRules(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	orgID := e.newOrg(t, 1)
	ev := e.newEvent(t, orgID)

	_, err := e.events.AddPrize(ctx, orgID, ev.ID, PrizeInput{Name: "一等奖", Quota: 0, SortNo: 1})
	expectCode(t, err, bizerr.CodeBadRequest)
	first, err := e.events.AddPrize(ctx, orgID, ev.ID, PrizeInput{Name: "一等奖", Gift: "手机", Quota: 1, SortNo: 1})
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.events.AddPrize(ctx, orgID, ev.ID, PrizeInput{Name: "二等奖", Quota: 2, SortNo: 1})
	expectCode(t, err, bizerr.CodeConflict)
	second, _ := e.events.AddPrize(ctx, orgID, ev.ID, PrizeInput{Name: "二等奖", Quota: 2, SortNo: 2})

	_, err = e.events.UpdatePrize(ctx, orgID, ev.ID, second.ID, PrizePatch{SortNo: new(1)})
	expectCode(t, err, bizerr.CodeConflict)
	_, err = e.events.UpdatePrize(ctx, orgID, ev.ID, second.ID, PrizePatch{Quota: new(-1)})
	expectCode(t, err, bizerr.CodeBadRequest)
	if _, err := e.events.UpdatePrize(ctx, orgID, ev.ID, first.ID, PrizePatch{SortNo: new(3)}); err != nil {
		t.Fatal(err)
	}
	prizes, _ := e.events.ListPrizes(ctx, orgID, ev.ID)
	if len(prizes) != 2 || prizes[0].ID != second.ID {
		t.Fatalf("prizes by sort_no = %+v", prizes)
	}
	if err := e.events.DeletePrize(ctx, orgID, ev.ID, first.ID); err != nil {
		t.Fatal(err)
	}
	view, _ := e.events.Get(ctx, orgID, ev.ID)
	if view.PrizeCount != 1 {
		t.Fatalf("prize count = %d", view.PrizeCount)
	}
}

func TestPhoneLast4(t *testing.T) {
	cases := map[string]string{"13800001234": "1234", "+86 138-0000-5678": "5678", "0012": "0012"}
	for in, want := range cases {
		if got, ok := PhoneLast4(in); !ok || got != want {
			t.Fatalf("PhoneLast4(%q) = %q, %v", in, got, ok)
		}
	}
	for _, in := range []string{"", "123", "abc-12"} {
		if _, ok := PhoneLast4(in); ok {
			t.Fatalf("PhoneLast4(%q) accepted", in)
		}
	}
}
