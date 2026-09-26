package guest

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
	"golottery/api/internal/geo"
	"golottery/api/internal/org"
	"golottery/api/internal/ratelimit"
	"golottery/api/internal/redisx"
	"golottery/api/internal/store"
	"golottery/api/internal/wechat"
	"golottery/api/internal/wechat/wechattest"
)

// The fence center is the GCJ-02 image of this GPS point.
const gpsLat, gpsLng = 39.907500, 116.391200

type env struct {
	guests *Service
	events *event.Service
	db     *store.DB
	orgID  uuid.UUID
	ev     event.View
}

func newEnv(t *testing.T, mode string) env {
	t.Helper()
	db := store.OpenTest(t)
	store.Reset(t, db, &Attempt{}, &ManualRequest{}, &Staff{}, &Session{}, &event.Prize{}, &event.Attendee{},
		&org.LedgerEntry{}, &event.Event{}, &org.User{}, &org.Quota{}, &org.Org{}, &auth.LoginAttempt{})
	ctx := context.Background()
	created, err := org.NewService(db.Gorm).Create(ctx, org.CreateInput{Name: "Acme", EventCredits: 5, MaxAttendees: 50}, org.Operator{Type: "platform", ID: "ops"})
	if err != nil {
		t.Fatal(err)
	}
	events := event.NewService(db.Gorm)
	ev, err := events.Create(ctx, created.ID, "年会")
	if err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"李雷", "韩梅梅", "王五"} {
		if _, err := events.AddAttendee(ctx, created.ID, ev.ID, event.AttendeeInput{Name: name, Phone: fmt.Sprintf("100%d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	lat, lng := geo.WGS84ToGCJ02(gpsLat, gpsLng)
	start, end := time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
	radius, ready := 100, event.StatusReady
	ev, err = events.Update(ctx, created.ID, ev.ID, event.Patch{CenterLat: &lat, CenterLng: &lng, RadiusM: &radius, CheckinStart: &start, CheckinEnd: &end, Status: &ready}, org.Operator{Type: "console", ID: "a"})
	if err != nil {
		t.Fatal(err)
	}
	fake := wechattest.New()
	t.Cleanup(fake.Close)
	guests := NewService(db.Gorm, Config{
		Mode:    mode,
		Wechat:  wechat.New(wechat.Config{AppID: "wx", AppSecret: "s", BaseURL: fake.URL}),
		Limiter: ratelimit.New(redisx.OpenTest(t), nil),
	})
	return env{guests: guests, events: events, db: db, orgID: created.ID, ev: ev}
}

func (e env) login(t *testing.T, device string) Guest {
	t.Helper()
	issued, _, err := e.guests.Login(context.Background(), LoginInput{PublicID: e.ev.PublicID, DeviceID: device})
	if err != nil {
		t.Fatalf("Login(%s) error = %v", device, err)
	}
	g, err := e.guests.Lookup(context.Background(), issued.Plain)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func (e env) bound(t *testing.T, device, name, last4 string) Guest {
	t.Helper()
	g := e.login(t, device)
	if _, err := e.guests.Bind(context.Background(), g, name, last4, "10.0.0.1"); err != nil {
		t.Fatalf("Bind(%s) error = %v", name, err)
	}
	return g
}

func expectCode(t *testing.T, err error, code bizerr.Code) {
	t.Helper()
	be, ok := bizerr.As(err)
	if !ok || be.Code != code {
		t.Fatalf("error = %v, want %s", err, code)
	}
}

func ptr[T any](v T) *T { return &v }

func TestWebLogin(t *testing.T) {
	e := newEnv(t, ModeWeb)
	ctx := context.Background()
	_, _, err := e.guests.Login(ctx, LoginInput{PublicID: e.ev.PublicID, DeviceID: "short"})
	expectCode(t, err, bizerr.CodeBadRequest)
	_, _, err = e.guests.Login(ctx, LoginInput{PublicID: "missing", DeviceID: "device-aaaaaaaaaaaaaaaa"})
	expectCode(t, err, bizerr.CodeNotFound)

	first, ev, err := e.guests.Login(ctx, LoginInput{PublicID: e.ev.PublicID, DeviceID: "device-aaaaaaaaaaaaaaaa"})
	if err != nil || ev.ID != e.ev.ID {
		t.Fatalf("login = %v, %v", ev.ID, err)
	}
	g, err := e.guests.Lookup(ctx, first.Plain)
	if err != nil || g.OpenID != "web:device-aaaaaaaaaaaaaaaa" {
		t.Fatalf("guest = %+v, %v", g, err)
	}
	second, _, _ := e.guests.Login(ctx, LoginInput{PublicID: e.ev.PublicID, DeviceID: "device-aaaaaaaaaaaaaaaa"})
	if _, err := e.guests.Lookup(ctx, first.Plain); err != ErrSessionNotFound {
		t.Fatal("logging in again kept the old token")
	}
	if _, err := e.guests.Lookup(ctx, second.Plain); err != nil {
		t.Fatal(err)
	}
}

func TestWechatLogin(t *testing.T) {
	e := newEnv(t, ModeWechat)
	ctx := context.Background()
	issued, _, err := e.guests.Login(ctx, LoginInput{PublicID: e.ev.PublicID, Code: "c-alice"})
	if err != nil {
		t.Fatal(err)
	}
	if g, _ := e.guests.Lookup(ctx, issued.Plain); g.OpenID != "o-alice" {
		t.Fatalf("openid = %q", g.OpenID)
	}
	_, _, err = e.guests.Login(ctx, LoginInput{PublicID: e.ev.PublicID, Code: "bogus"})
	expectCode(t, err, bizerr.CodeUnauthorized)
	_, _, err = e.guests.Login(ctx, LoginInput{PublicID: e.ev.PublicID, DeviceID: "device-aaaaaaaaaaaaaaaa"})
	expectCode(t, err, bizerr.CodeBadRequest)

	unconfigured := NewService(e.db.Gorm, Config{Mode: ModeWechat, Wechat: wechat.New(wechat.Config{})})
	_, _, err = unconfigured.Login(ctx, LoginInput{PublicID: e.ev.PublicID, Code: "c-alice"})
	expectCode(t, err, bizerr.CodeWechatNotConfigured)
}

func TestBind(t *testing.T) {
	e := newEnv(t, ModeWeb)
	ctx := context.Background()
	g := e.login(t, "device-aaaaaaaaaaaaaaaa")

	_, err := e.guests.Bind(ctx, g, "李雷", "9999", "10.0.0.1")
	expectCode(t, err, bizerr.CodeAttendeeNotMatched)
	st, err := e.guests.Bind(ctx, g, " 李雷 ", "1000", "10.0.0.1")
	if err != nil || st.Attendee == nil || st.Attendee.Name != "李雷" {
		t.Fatalf("bind = %+v, %v", st.Attendee, err)
	}
	if again, err := e.guests.Bind(ctx, g, "韩梅梅", "1001", "10.0.0.1"); err != nil || again.Attendee.Name != "李雷" {
		t.Fatalf("binding twice changed the person: %+v, %v", again.Attendee, err)
	}

	other := e.login(t, "device-bbbbbbbbbbbbbbbb")
	_, err = e.guests.Bind(ctx, other, "李雷", "1000", "10.0.0.2")
	expectCode(t, err, bizerr.CodeAttendeeTaken)
}

func TestBindLockouts(t *testing.T) {
	e := newEnv(t, ModeWeb)
	ctx := context.Background()
	g := e.login(t, "device-aaaaaaaaaaaaaaaa")
	for range bindMaxFailures {
		_, err := e.guests.Bind(ctx, g, "李雷", "0000", "10.0.0.1")
		expectCode(t, err, bizerr.CodeAttendeeNotMatched)
	}
	_, err := e.guests.Bind(ctx, g, "李雷", "1000", "10.0.0.1")
	expectCode(t, err, bizerr.CodeTooManyAttempts)

	// A new browser identity from the same IP is still stopped by the IP limit.
	for i := range bindIPMaxFailures - bindMaxFailures {
		fresh := e.login(t, fmt.Sprintf("device-%016d", i))
		_, err := e.guests.Bind(ctx, fresh, "李雷", "0000", "10.0.0.1")
		expectCode(t, err, bizerr.CodeAttendeeNotMatched)
	}
	fresh := e.login(t, "device-cccccccccccccccc")
	_, err = e.guests.Bind(ctx, fresh, "李雷", "1000", "10.0.0.1")
	expectCode(t, err, bizerr.CodeTooManyAttempts)
	if _, err := e.guests.Bind(ctx, fresh, "李雷", "1000", "10.0.0.9"); err != nil {
		t.Fatalf("another IP was blocked: %v", err)
	}
}

func TestGeoCheckin(t *testing.T) {
	e := newEnv(t, ModeWeb)
	ctx := context.Background()
	g := e.bound(t, "device-aaaaaaaaaaaaaaaa", "李雷", "1000")

	_, err := e.guests.Checkin(ctx, g, CheckinInput{})
	expectCode(t, err, bizerr.CodeBadRequest)
	_, err = e.guests.Checkin(ctx, g, CheckinInput{Lat: ptr(gpsLat), Lng: ptr(gpsLng), Accuracy: ptr(20.0), CoordType: CoordGCJ02})
	expectCode(t, err, bizerr.CodeOutOfRange)
	_, err = e.guests.Checkin(ctx, g, CheckinInput{Lat: ptr(gpsLat), Lng: ptr(gpsLng), Accuracy: ptr(600.0), CoordType: CoordWGS84})
	expectCode(t, err, bizerr.CodeLowAccuracy)

	st, err := e.guests.Checkin(ctx, g, CheckinInput{Lat: ptr(gpsLat), Lng: ptr(gpsLng), Accuracy: ptr(20.0), CoordType: CoordWGS84})
	if err != nil || st.Attendee.Status != event.AttendeeCheckedIn || *st.Attendee.CheckinMethod != MethodGeo {
		t.Fatalf("checkin = %+v, %v", st.Attendee, err)
	}
	firstAt := *st.Attendee.CheckinAt
	st, err = e.guests.Checkin(ctx, g, CheckinInput{})
	if err != nil || !st.Attendee.CheckinAt.Equal(firstAt) {
		t.Fatalf("second tap changed the time: %v, %v", st.Attendee.CheckinAt, err)
	}

	var attempts []Attempt
	e.db.Gorm.Where("openid = ?", g.OpenID).Order("id").Find(&attempts)
	reasons := ""
	for _, a := range attempts {
		reasons += a.Result + ":" + a.Reason + " "
	}
	if reasons != "rejected:bad_coordinates rejected:out_of_range rejected:low_accuracy passed: passed:already " {
		t.Fatalf("attempts = %q", reasons)
	}
	if attempts[1].DistanceM == nil || *attempts[1].DistanceM < 300 {
		t.Fatalf("out-of-range distance = %v", attempts[1].DistanceM)
	}
}

func TestAccuracyCountsTowardTheFence(t *testing.T) {
	e := newEnv(t, ModeWeb)
	ctx := context.Background()
	g := e.bound(t, "device-aaaaaaaaaaaaaaaa", "李雷", "1000")
	lat, lng := geo.WGS84ToGCJ02(gpsLat, gpsLng)
	// About 250 m north: outside a 100 m fence, inside once 200 m of accuracy is credited.
	far := lat + 0.00225
	_, err := e.guests.Checkin(ctx, g, CheckinInput{Lat: &far, Lng: &lng, Accuracy: ptr(100.0)})
	expectCode(t, err, bizerr.CodeOutOfRange)
	if _, err := e.guests.Checkin(ctx, g, CheckinInput{Lat: &far, Lng: &lng, Accuracy: ptr(400.0)}); err != nil {
		t.Fatalf("accuracy credit not applied: %v", err)
	}
}

func TestCheckinOrderAndModes(t *testing.T) {
	e := newEnv(t, ModeWeb)
	ctx := context.Background()
	stranger := e.login(t, "device-aaaaaaaaaaaaaaaa")
	_, err := e.guests.Checkin(ctx, stranger, CheckinInput{})
	expectCode(t, err, bizerr.CodeNotBound)

	// Back to draft before anyone binds: live data would block the move.
	direct, draft := event.ModeDirect, event.StatusDraft
	if _, err := e.events.Update(ctx, e.orgID, e.ev.ID, event.Patch{Status: &draft}, org.Operator{}); err != nil {
		t.Fatal(err)
	}
	g := e.bound(t, "device-bbbbbbbbbbbbbbbb", "韩梅梅", "1001")
	_, err = e.guests.Checkin(ctx, g, CheckinInput{})
	expectCode(t, err, bizerr.CodeEventNotOpen)

	ready := event.StatusReady
	past := time.Now().Add(-2 * time.Hour)
	pastEnd := time.Now().Add(-time.Hour)
	if _, err := e.events.Update(ctx, e.orgID, e.ev.ID, event.Patch{Status: &ready, CheckinMode: &direct, CheckinStart: &past, CheckinEnd: &pastEnd}, org.Operator{}); err != nil {
		t.Fatal(err)
	}
	_, err = e.guests.Checkin(ctx, g, CheckinInput{})
	expectCode(t, err, bizerr.CodeWindowClosed)

	end := time.Now().Add(time.Hour)
	if _, err := e.events.Update(ctx, e.orgID, e.ev.ID, event.Patch{CheckinEnd: &end}, org.Operator{}); err != nil {
		t.Fatal(err)
	}
	// Direct mode ignores and never stores coordinates, even far away ones.
	st, err := e.guests.Checkin(ctx, g, CheckinInput{Lat: ptr(0.0), Lng: ptr(0.0), Accuracy: ptr(5.0)})
	if err != nil || *st.Attendee.CheckinMethod != MethodDirect {
		t.Fatalf("direct checkin = %+v, %v", st.Attendee, err)
	}
	var last Attempt
	e.db.Gorm.Where("openid = ?", g.OpenID).Order("id DESC").Take(&last)
	if last.Lat != nil || last.DistanceM != nil {
		t.Fatal("direct mode stored coordinates")
	}
}

func TestConcurrentCheckinSetsOneTime(t *testing.T) {
	e := newEnv(t, ModeWeb)
	ctx := context.Background()
	g := e.bound(t, "device-aaaaaaaaaaaaaaaa", "李雷", "1000")
	in := CheckinInput{Lat: ptr(gpsLat), Lng: ptr(gpsLng), Accuracy: ptr(10.0), CoordType: CoordWGS84}
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() { _, _ = e.guests.Checkin(ctx, g, in) })
	}
	wg.Wait()
	var a event.Attendee
	e.db.Gorm.Where("openid = ?", g.OpenID).Take(&a)
	if a.Status != event.AttendeeCheckedIn || a.CheckinAt == nil {
		t.Fatalf("attendee = %+v", a)
	}
}

func TestCheckinRateLimit(t *testing.T) {
	e := newEnv(t, ModeWeb)
	ctx := context.Background()
	g := e.login(t, "device-aaaaaaaaaaaaaaaa")
	for range checkinPerMinute {
		_, err := e.guests.Checkin(ctx, g, CheckinInput{})
		expectCode(t, err, bizerr.CodeNotBound)
	}
	_, err := e.guests.Checkin(ctx, g, CheckinInput{})
	expectCode(t, err, bizerr.CodeTooManyAttempts)
}

func TestHelpRequests(t *testing.T) {
	e := newEnv(t, ModeWeb)
	ctx := context.Background()
	stranger := e.login(t, "device-aaaaaaaaaaaaaaaa")
	_, err := e.guests.SubmitRequest(ctx, stranger, RequestInput{Reason: "名单没有我"})
	expectCode(t, err, bizerr.CodeBadRequest)
	st, err := e.guests.SubmitRequest(ctx, stranger, RequestInput{Name: "赵六", PhoneLast4: "7777", Reason: "名单没有我"})
	if err != nil || st.Request == nil || st.Request.Status != RequestPending || st.Request.ClaimedName != "赵六" {
		t.Fatalf("request = %+v, %v", st.Request, err)
	}
	_, err = e.guests.SubmitRequest(ctx, stranger, RequestInput{Name: "赵六", PhoneLast4: "7777"})
	expectCode(t, err, bizerr.CodeConflict)

	g := e.bound(t, "device-bbbbbbbbbbbbbbbb", "李雷", "1000")
	st, err = e.guests.SubmitRequest(ctx, g, RequestInput{Reason: "定位不准"})
	if err != nil || st.Request.AttendeeID == nil || *st.Request.AttendeeID != st.Attendee.ID {
		t.Fatalf("bound request = %+v, %v", st.Request, err)
	}
}

func TestExpiredGuestTokenIsRejected(t *testing.T) {
	e := newEnv(t, ModeWeb)
	ctx := context.Background()
	e.guests.now = func() time.Time { return time.Now().Add(-25 * time.Hour) }
	issued, _, err := e.guests.Login(ctx, LoginInput{PublicID: e.ev.PublicID, DeviceID: "device-aaaaaaaaaaaaaaaa"})
	if err != nil {
		t.Fatal(err)
	}
	e.guests.now = time.Now
	if _, err := e.guests.Lookup(ctx, issued.Plain); err != ErrSessionNotFound {
		t.Fatalf("expired token lookup = %v", err)
	}
}
