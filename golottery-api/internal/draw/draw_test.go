package draw

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"golottery/api/internal/auth"
	"golottery/api/internal/bizerr"
	"golottery/api/internal/event"
	"golottery/api/internal/guest"
	"golottery/api/internal/org"
	"golottery/api/internal/store"
)

type env struct {
	db     *store.DB
	events *event.Service
	draws  *Service
	orgID  uuid.UUID
	ev     event.View
	host   HostSession
}

func newEnv(t *testing.T) env {
	t.Helper()
	db := store.OpenTest(t)
	store.Reset(t, db,
		&Result{}, &Log{}, &Host{},
		&guest.Attempt{}, &guest.ManualRequest{}, &guest.Staff{}, &guest.Session{},
		&event.Prize{}, &event.Attendee{}, &org.LedgerEntry{}, &event.Event{},
		&org.User{}, &org.Quota{}, &org.Org{}, &auth.APIToken{}, &auth.LoginAttempt{},
	)
	tokens := auth.NewTokenIssuer(db.Gorm, time.Hour, time.Hour, time.Hour)
	limiter := auth.NewLoginLimiter(db.Gorm, 5, time.Minute)
	draws := NewService(Config{DB: db.Gorm, Tokens: tokens, Limiter: limiter})
	events := event.NewService(db.Gorm)
	orgs := org.NewService(db.Gorm)
	created, err := orgs.Create(context.Background(), org.CreateInput{Name: "Acme", EventCredits: 5, MaxAttendees: 20}, org.Operator{Type: "platform", ID: "ops"})
	if err != nil {
		t.Fatal(err)
	}
	ev, err := events.Create(context.Background(), created.ID, "年会")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now().Add(-time.Hour)
	end := start.Add(5 * time.Hour)
	lat, lng := 39.9, 116.4
	admin := org.Operator{Type: "console", ID: "admin"}
	if _, err := events.Update(context.Background(), created.ID, ev.ID, event.Patch{
		CheckinStart: &start, CheckinEnd: &end, CenterLat: &lat, CenterLng: &lng,
	}, admin); err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"李雷", "韩梅梅", "王五", "赵六"} {
		if _, err := events.AddAttendee(context.Background(), created.ID, ev.ID, event.AttendeeInput{
			Name: name, Phone: fmt.Sprintf("1380000100%d", i),
		}); err != nil {
			t.Fatal(err)
		}
	}
	ready := event.StatusReady
	if _, err := events.Update(context.Background(), created.ID, ev.ID, event.Patch{Status: &ready}, admin); err != nil {
		t.Fatal(err)
	}
	ev, err = events.Get(context.Background(), created.ID, ev.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Gorm.Model(&event.Attendee{}).Where("event_id = ?", ev.ID).
		Updates(map[string]any{"status": event.AttendeeCheckedIn, "checkin_at": time.Now().UTC()}).Error; err != nil {
		t.Fatal(err)
	}
	cred, err := draws.UpsertHost(context.Background(), created.ID, ev.ID)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := draws.Login(context.Background(), cred.PublicID, cred.Password)
	if err != nil {
		t.Fatal(err)
	}
	token, err := tokens.Lookup(context.Background(), auth.TokenTypeHost, issued.Plain)
	if err != nil {
		t.Fatal(err)
	}
	host, err := draws.ResolveHost(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	return env{db: db, events: events, draws: draws, orgID: created.ID, ev: ev, host: host}
}

func (e env) addPrize(t *testing.T, name string, quota, sortNo int) event.Prize {
	t.Helper()
	p, err := e.events.AddPrize(context.Background(), e.orgID, e.ev.ID, event.PrizeInput{Name: name, Quota: quota, SortNo: sortNo})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func expectCode(t *testing.T, err error, code bizerr.Code) {
	t.Helper()
	be, ok := bizerr.As(err)
	if !ok || be.Code != code {
		t.Fatalf("error = %v, want %s", err, code)
	}
}

func TestDrawFromPoolAndIdempotent(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	prize := e.addPrize(t, "三等奖", 2, 1)
	req := uuid.New()
	batch, err := e.draws.Draw(ctx, e.host, prize.ID, 2, req)
	if err != nil || len(batch.Results) != 2 {
		t.Fatalf("draw = %+v, %v", batch, err)
	}
	seen := map[uuid.UUID]bool{}
	for _, r := range batch.Results {
		if r.Status != StatusValid || seen[r.AttendeeID] {
			t.Fatalf("bad result %+v", r)
		}
		seen[r.AttendeeID] = true
	}
	again, err := e.draws.Draw(ctx, e.host, prize.ID, 2, req)
	if err != nil || again.DrawVersion != batch.DrawVersion || len(again.Results) != 2 {
		t.Fatalf("idempotent = %+v, %v", again, err)
	}
	replayed := map[uuid.UUID]bool{}
	for _, r := range again.Results {
		replayed[r.ID] = true
	}
	for _, r := range batch.Results {
		if !replayed[r.ID] {
			t.Fatalf("idempotent missing %s", r.ID)
		}
	}
	var n int64
	e.db.Gorm.Model(&Result{}).Where("prize_id = ?", prize.ID).Count(&n)
	if n != 2 {
		t.Fatalf("stored %d results", n)
	}
}

func TestConcurrentDrawRespectsQuota(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	prize := e.addPrize(t, "二等奖", 1, 1)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.draws.Draw(ctx, e.host, prize.ID, 1, uuid.New())
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	var ok, conflict int
	for err := range errs {
		if err == nil {
			ok++
			continue
		}
		be, yes := bizerr.As(err)
		if yes && be.Code == bizerr.CodeConflict {
			conflict++
			continue
		}
		t.Fatalf("unexpected error %v", err)
	}
	if ok != 1 || conflict != 1 {
		t.Fatalf("ok=%d conflict=%d", ok, conflict)
	}
}

func TestVoidAndNoRedrawSamePrize(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	prize := e.addPrize(t, "一等奖", 1, 1)
	batch, err := e.draws.Draw(ctx, e.host, prize.ID, 1, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	winner := batch.Results[0]
	voided, err := e.draws.Void(ctx, e.host, winner.ID, "缺席")
	if err != nil || voided.Status != StatusVoid {
		t.Fatalf("void = %+v, %v", voided, err)
	}
	again, err := e.draws.Void(ctx, e.host, winner.ID, "再次")
	if err != nil || again.Status != StatusVoid || again.VoidReason == nil || *again.VoidReason != "缺席" {
		t.Fatalf("idempotent void = %+v, %v", again, err)
	}
	batch2, err := e.draws.Draw(ctx, e.host, prize.ID, 1, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if batch2.Results[0].AttendeeID == winner.AttendeeID {
		t.Fatal("redraw picked the voided person")
	}
}

func TestExclusiveBlocksSecondPrize(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	first := e.addPrize(t, "三等奖", 1, 1)
	second := e.addPrize(t, "二等奖", 1, 2)
	batch, err := e.draws.Draw(ctx, e.host, first.ID, 1, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	pool, err := e.draws.Pool(ctx, e.host)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pool {
		if p.ID == batch.Results[0].AttendeeID {
			t.Fatal("winner still in exclusive pool")
		}
	}
	batch2, err := e.draws.Draw(ctx, e.host, second.ID, 1, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if batch2.Results[0].AttendeeID == batch.Results[0].AttendeeID {
		t.Fatal("exclusive winner drawn again")
	}
	allow := true
	_, err = e.events.Update(ctx, e.orgID, e.ev.ID, event.Patch{AllowMultiWin: &allow}, org.Operator{Type: "console", ID: "admin"})
	expectCode(t, err, bizerr.CodeConflict)
}

func TestPrizeAndAttendeeGuards(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	prize := e.addPrize(t, "特等奖", 2, 1)
	batch, err := e.draws.Draw(ctx, e.host, prize.ID, 2, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	expectCode(t, e.events.DeletePrize(ctx, e.orgID, e.ev.ID, prize.ID), bizerr.CodeConflict)
	expectCode(t, e.events.DeleteAttendee(ctx, e.orgID, e.ev.ID, batch.Results[0].AttendeeID), bizerr.CodeConflict)
	one := 1
	_, err = e.events.UpdatePrize(ctx, e.orgID, e.ev.ID, prize.ID, event.PrizePatch{Quota: &one})
	expectCode(t, err, bizerr.CodeConflict)
}

func TestResetClearsDrawRows(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	prize := e.addPrize(t, "三等奖", 1, 1)
	if _, err := e.draws.Draw(ctx, e.host, prize.ID, 1, uuid.New()); err != nil {
		t.Fatal(err)
	}
	guests := guest.NewService(e.db.Gorm, guest.Config{Mode: guest.ModeWeb})
	start := time.Now().Add(24 * time.Hour)
	end := start.Add(3 * time.Hour)
	if _, err := e.events.Update(ctx, e.orgID, e.ev.ID, event.Patch{CheckinStart: &start, CheckinEnd: &end}, org.Operator{Type: "console", ID: "admin"}); err != nil {
		t.Fatal(err)
	}
	counts, err := guests.ResetLiveData(ctx, e.orgID, e.ev.ID, "年会")
	if err != nil {
		t.Fatal(err)
	}
	if counts.Results != 1 || counts.Logs != 1 {
		t.Fatalf("reset counts = %+v", counts)
	}
	var results, logs int64
	e.db.Gorm.Model(&Result{}).Count(&results)
	e.db.Gorm.Model(&Log{}).Count(&logs)
	if results != 0 || logs != 0 {
		t.Fatalf("left results=%d logs=%d", results, logs)
	}
	var version int64
	e.db.Gorm.Model(&event.Event{}).Where("id = ?", e.ev.ID).Select("draw_version").Scan(&version)
	if version != 0 {
		t.Fatalf("draw_version=%d", version)
	}
}
