package guest

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"golottery/api/internal/bizerr"
	"golottery/api/internal/event"
)

func (e env) staffGuest(t *testing.T, device, role string) Guest {
	t.Helper()
	code, _, err := e.guests.CreateInvite(context.Background(), e.orgID, e.ev.ID, role)
	if err != nil {
		t.Fatal(err)
	}
	g := e.login(t, device)
	if _, err := e.guests.Join(context.Background(), g, code); err != nil {
		t.Fatalf("Join() error = %v", err)
	}
	return g
}

func TestInvites(t *testing.T) {
	e := newEnv(t, ModeWeb)
	ctx := context.Background()
	_, _, err := e.guests.CreateInvite(ctx, e.orgID, e.ev.ID, "boss")
	expectCode(t, err, bizerr.CodeBadRequest)
	_, _, err = e.guests.CreateInvite(ctx, uuid.New(), e.ev.ID, RoleStaff)
	expectCode(t, err, bizerr.CodeNotFound)

	code, _, err := e.guests.CreateInvite(ctx, e.orgID, e.ev.ID, RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	g := e.login(t, "device-aaaaaaaaaaaaaaaa")
	st, err := e.guests.Join(ctx, g, code)
	if err != nil || st.StaffRole != RoleAdmin {
		t.Fatalf("join = %q, %v", st.StaffRole, err)
	}
	other := e.login(t, "device-bbbbbbbbbbbbbbbb")
	_, err = e.guests.Join(ctx, other, code)
	expectCode(t, err, bizerr.CodeInviteInvalid)

	expired, _, _ := e.guests.CreateInvite(ctx, e.orgID, e.ev.ID, RoleStaff)
	e.db.Gorm.Model(&Invite{}).Where("used_at IS NULL").Update("expires_at", time.Now().Add(-time.Minute))
	_, err = e.guests.Join(ctx, other, expired)
	expectCode(t, err, bizerr.CodeInviteInvalid)

	staff, err := e.guests.ListStaff(ctx, e.orgID, e.ev.ID)
	if err != nil || len(staff) != 1 || staff[0].Role != RoleAdmin {
		t.Fatalf("staff = %+v, %v", staff, err)
	}
	if err := e.guests.RemoveStaff(ctx, e.orgID, e.ev.ID, staff[0].ID); err != nil {
		t.Fatal(err)
	}
	_, err = e.guests.Summary(ctx, g)
	expectCode(t, err, bizerr.CodeNotFound)
}

func TestInviteBelongsToItsEvent(t *testing.T) {
	e := newEnv(t, ModeWeb)
	ctx := context.Background()
	code, _, _ := e.guests.CreateInvite(ctx, e.orgID, e.ev.ID, RoleStaff)
	otherEvent, err := e.events.Create(ctx, e.orgID, "别的活动")
	if err != nil {
		t.Fatal(err)
	}
	issued, _, err := e.guests.Login(ctx, LoginInput{PublicID: otherEvent.PublicID, DeviceID: "device-aaaaaaaaaaaaaaaa"})
	if err != nil {
		t.Fatal(err)
	}
	g, _ := e.guests.Lookup(ctx, issued.Plain)
	_, err = e.guests.Join(ctx, g, code)
	expectCode(t, err, bizerr.CodeInviteInvalid)
}

func TestGuestsWithoutRoleSeeNothing(t *testing.T) {
	e := newEnv(t, ModeWeb)
	ctx := context.Background()
	g := e.login(t, "device-aaaaaaaaaaaaaaaa")
	_, err := e.guests.Summary(ctx, g)
	expectCode(t, err, bizerr.CodeNotFound)
	_, err = e.guests.PendingRequests(ctx, g)
	expectCode(t, err, bizerr.CodeNotFound)
	_, err = e.guests.ProxyCheckin(ctx, g, uuid.New())
	expectCode(t, err, bizerr.CodeNotFound)

	staff := e.staffGuest(t, "device-bbbbbbbbbbbbbbbb", RoleStaff)
	direct := event.ModeDirect
	_, err = e.guests.UpdateSettings(ctx, staff, SettingsPatch{CheckinMode: &direct})
	expectCode(t, err, bizerr.CodeNotFound)
}

func TestApproveThreeWays(t *testing.T) {
	e := newEnv(t, ModeWeb)
	ctx := context.Background()
	staff := e.staffGuest(t, "device-ssssssssssssssss", RoleStaff)

	// 1. A bound guest whose location failed.
	bound := e.bound(t, "device-aaaaaaaaaaaaaaaa", "李雷", "1000")
	st, _ := e.guests.SubmitRequest(ctx, bound, RequestInput{Reason: "定位失败"})
	if err := e.guests.ApproveRequest(ctx, staff, st.Request.ID, Approval{}); err != nil {
		t.Fatal(err)
	}
	st, _ = e.guests.Status(ctx, bound)
	if st.Attendee.Status != event.AttendeeCheckedIn || *st.Attendee.CheckinMethod != MethodManual || *st.Attendee.CheckinBy != staff.OpenID || st.Request.Status != RequestApproved {
		t.Fatalf("bound approval = %+v %+v", st.Attendee, st.Request)
	}

	// 2. A typo in the roster: staff link the guest to the existing person.
	typo := e.login(t, "device-bbbbbbbbbbbbbbbb")
	for range bindMaxFailures {
		_, _ = e.guests.Bind(ctx, typo, "韩梅", "1001", "10.0.0.2")
	}
	st, _ = e.guests.SubmitRequest(ctx, typo, RequestInput{Name: "韩梅", PhoneLast4: "1001"})
	var hmm event.Attendee
	e.db.Gorm.Where("name = ?", "韩梅梅").Take(&hmm)
	if err := e.guests.ApproveRequest(ctx, staff, st.Request.ID, Approval{AttendeeID: &hmm.ID}); err != nil {
		t.Fatal(err)
	}
	st, _ = e.guests.Status(ctx, typo)
	if st.Attendee == nil || st.Attendee.ID != hmm.ID || st.Attendee.Status != event.AttendeeCheckedIn {
		t.Fatalf("linked approval = %+v", st.Attendee)
	}
	if wait, _ := e.guests.binding.Check(ctx, "bind:"+e.ev.ID.String()+":"+typo.OpenID); wait != 0 {
		t.Fatal("approval did not clear the binding lockout")
	}

	// Linking to someone bound to another identity is refused.
	thief := e.login(t, "device-cccccccccccccccc")
	st, _ = e.guests.SubmitRequest(ctx, thief, RequestInput{Name: "李雷", PhoneLast4: "1000"})
	var lilei event.Attendee
	e.db.Gorm.Where("name = ?", "李雷").Take(&lilei)
	expectCode(t, e.guests.ApproveRequest(ctx, staff, st.Request.ID, Approval{AttendeeID: &lilei.ID}), bizerr.CodeConflict)

	// 3. Not on the roster at all: staff add the person.
	walkIn := e.login(t, "device-dddddddddddddddd")
	st, _ = e.guests.SubmitRequest(ctx, walkIn, RequestInput{Name: "赵六", PhoneLast4: "7777"})
	create := event.AttendeeInput{Name: "赵六", Dept: "访客", Phone: "7777"}
	if err := e.guests.ApproveRequest(ctx, staff, st.Request.ID, Approval{Create: &create}); err != nil {
		t.Fatal(err)
	}
	st, _ = e.guests.Status(ctx, walkIn)
	if st.Attendee == nil || st.Attendee.Name != "赵六" || st.Attendee.Status != event.AttendeeCheckedIn {
		t.Fatalf("walk-in approval = %+v", st.Attendee)
	}
	expectCode(t, e.guests.ApproveRequest(ctx, staff, st.Request.ID, Approval{}), bizerr.CodeConflict)

	sum, err := e.guests.Summary(ctx, staff)
	if err != nil || sum.Total != 4 || sum.CheckedIn != 3 || sum.PendingRequests != 1 {
		t.Fatalf("summary = %+v, %v", sum, err)
	}
}

func TestWalkInRespectsTheLimit(t *testing.T) {
	e := newEnv(t, ModeWeb)
	ctx := context.Background()
	e.db.Gorm.Model(&event.Event{}).Where("id = ?", e.ev.ID).Update("max_attendees", 3)
	staff := e.staffGuest(t, "device-ssssssssssssssss", RoleStaff)
	walkIn := e.login(t, "device-dddddddddddddddd")
	st, _ := e.guests.SubmitRequest(ctx, walkIn, RequestInput{Name: "赵六", PhoneLast4: "7777"})
	create := event.AttendeeInput{Name: "赵六", Phone: "7777"}
	expectCode(t, e.guests.ApproveRequest(ctx, staff, st.Request.ID, Approval{Create: &create}), bizerr.CodeRosterFull)
	st, _ = e.guests.Status(ctx, walkIn)
	if st.Attendee != nil || st.Request.Status != RequestPending {
		t.Fatal("a refused approval changed data")
	}
	if err := e.guests.RejectRequest(ctx, staff, st.Request.ID); err != nil {
		t.Fatal(err)
	}
	st, _ = e.guests.Status(ctx, walkIn)
	if st.Request.Status != RequestRejected {
		t.Fatalf("request = %s", st.Request.Status)
	}
	// After a rejection the guest may ask again.
	if _, err := e.guests.SubmitRequest(ctx, walkIn, RequestInput{Name: "赵六", PhoneLast4: "7777"}); err != nil {
		t.Fatal(err)
	}
}

func TestProxyAndSearch(t *testing.T) {
	e := newEnv(t, ModeWeb)
	ctx := context.Background()
	staff := e.staffGuest(t, "device-ssssssssssssssss", RoleStaff)
	found, err := e.guests.SearchAttendees(ctx, staff, "梅")
	if err != nil || len(found) != 1 || found[0].Name != "韩梅梅" {
		t.Fatalf("search = %+v, %v", found, err)
	}
	if found, _ := e.guests.SearchAttendees(ctx, staff, "%"); len(found) != 0 {
		t.Fatalf("%% matched %d rows; LIKE was not escaped", len(found))
	}
	a, err := e.guests.ProxyCheckin(ctx, staff, found[0].ID)
	if err != nil || a.Status != event.AttendeeCheckedIn || *a.CheckinMethod != MethodProxy {
		t.Fatalf("proxy = %+v, %v", a, err)
	}
	again, _ := e.guests.ProxyCheckin(ctx, staff, found[0].ID)
	if !again.CheckinAt.Equal(*a.CheckinAt) {
		t.Fatal("second proxy changed the time")
	}
	_, err = e.guests.ProxyCheckin(ctx, staff, uuid.New())
	expectCode(t, err, bizerr.CodeNotFound)
}

func TestOnSiteModeSwitch(t *testing.T) {
	e := newEnv(t, ModeWeb)
	ctx := context.Background()
	admin := e.staffGuest(t, "device-ssssssssssssssss", RoleAdmin)
	g := e.bound(t, "device-aaaaaaaaaaaaaaaa", "李雷", "1000")
	early := e.bound(t, "device-bbbbbbbbbbbbbbbb", "韩梅梅", "1001")
	if _, err := e.guests.Checkin(ctx, early, CheckinInput{Lat: ptr(gpsLat), Lng: ptr(gpsLng), Accuracy: ptr(10.0), CoordType: CoordWGS84}); err != nil {
		t.Fatal(err)
	}

	far := CheckinInput{Lat: ptr(31.2304), Lng: ptr(121.4737), Accuracy: ptr(10.0)}
	_, err := e.guests.Checkin(ctx, g, far)
	expectCode(t, err, bizerr.CodeOutOfRange)

	direct := event.ModeDirect
	if _, err := e.guests.UpdateSettings(ctx, admin, SettingsPatch{CheckinMode: &direct}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.guests.Checkin(ctx, g, far); err != nil {
		t.Fatalf("out-of-range guest after switching to direct: %v", err)
	}

	geoMode := event.ModeGeo
	if _, err := e.guests.UpdateSettings(ctx, admin, SettingsPatch{CheckinMode: &geoMode}); err != nil {
		t.Fatal(err)
	}
	st, _ := e.guests.Status(ctx, early)
	if st.Attendee.Status != event.AttendeeCheckedIn {
		t.Fatal("switching back to geo undid a check-in")
	}
	late := e.bound(t, "device-cccccccccccccccc", "王五", "1002")
	_, err = e.guests.Checkin(ctx, late, CheckinInput{})
	expectCode(t, err, bizerr.CodeBadRequest)
}
