package apihttp

import (
	"net/http"
	"strings"
	"testing"
)

func TestStaffFlowOverHTTP(t *testing.T) {
	env := newPlatformEnv(t)
	console, ev := env.readyDirectEvent(t)
	base := "/api/organization/events/" + ev.ID.String()

	rec := env.do(t, http.MethodPost, base+"/staff", console, map[string]string{"role": "admin"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("invite status = %d body = %s", rec.Code, rec.Body.String())
	}
	invite := decode[struct {
		Code string `json:"code"`
		Path string `json:"path"`
	}](t, rec)
	if !strings.HasPrefix(invite.Path, "/#/m/"+ev.PublicID+"/staff?invite=") || !strings.HasSuffix(invite.Path, invite.Code) {
		t.Fatalf("invite path = %q", invite.Path)
	}

	staff := env.guestLogin(t, ev.PublicID, "device-ssssssssssssssss")
	expectError(t, env.do(t, http.MethodGet, "/api/guest/staff/summary", staff, nil), http.StatusNotFound, "E_NOT_FOUND")
	expectError(t, env.do(t, http.MethodPost, "/api/guest/staff/join", staff, map[string]string{"invite": "wrong"}), http.StatusBadRequest, "E_INVITE_INVALID")
	rec = env.do(t, http.MethodPost, "/api/guest/staff/join", staff, map[string]string{"invite": invite.Code})
	if rec.Code != http.StatusOK || *decode[guestStatusT](t, rec).StaffRole != "admin" {
		t.Fatalf("join status = %d body = %s", rec.Code, rec.Body.String())
	}

	walkIn := env.guestLogin(t, ev.PublicID, "device-wwwwwwwwwwwwwwww")
	env.do(t, http.MethodPost, "/api/guest/manual-requests", walkIn, map[string]string{"name": "赵六", "phone_last4": "7777"})
	pending := decode[[]struct {
		ID          string `json:"id"`
		ClaimedName string `json:"claimed_name"`
	}](t, env.do(t, http.MethodGet, "/api/guest/staff/manual-requests", staff, nil))
	if len(pending) != 1 || pending[0].ClaimedName != "赵六" {
		t.Fatalf("pending = %+v", pending)
	}
	rec = env.do(t, http.MethodPost, "/api/guest/staff/manual-requests/"+pending[0].ID+"/approve", staff, map[string]any{"create": map[string]string{"name": "赵六", "phone": "7777"}})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("approve status = %d body = %s", rec.Code, rec.Body.String())
	}
	if st := decode[guestStatusT](t, env.do(t, http.MethodGet, "/api/guest/checkin", walkIn, nil)); st.Attendee == nil || !st.Attendee.CheckedIn || st.Attendee.CheckinMethod != "manual" {
		t.Fatalf("walk-in = %+v", st.Attendee)
	}

	found := decode[[]struct {
		ID        string `json:"id"`
		CheckedIn bool   `json:"checked_in"`
	}](t, env.do(t, http.MethodGet, "/api/guest/staff/attendees?q="+"%E6%9D%8E", staff, nil))
	if len(found) != 1 || found[0].CheckedIn {
		t.Fatalf("search = %+v", found)
	}
	rec = env.do(t, http.MethodPost, "/api/guest/staff/checkins/proxy", staff, map[string]string{"attendee_id": found[0].ID})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"checkin_method":"proxy"`) {
		t.Fatalf("proxy status = %d body = %s", rec.Code, rec.Body.String())
	}
	sum := decode[struct {
		Total     int `json:"total"`
		CheckedIn int `json:"checked_in"`
	}](t, env.do(t, http.MethodGet, "/api/guest/staff/summary", staff, nil))
	if sum.Total != 2 || sum.CheckedIn != 2 {
		t.Fatalf("summary = %+v", sum)
	}

	rec = env.do(t, http.MethodPatch, "/api/guest/staff/checkin-settings", staff, map[string]any{"checkin_mode": "geo"})
	expectError(t, rec, http.StatusBadRequest, "E_EVENT_INCOMPLETE")

	list := decode[[]struct {
		ID           string  `json:"id"`
		Role         string  `json:"role"`
		AttendeeName *string `json:"attendee_name"`
	}](t, env.do(t, http.MethodGet, base+"/staff", console, nil))
	if len(list) != 1 || list[0].Role != "admin" || list[0].AttendeeName != nil {
		t.Fatalf("staff list = %+v", list)
	}
	if rec := env.do(t, http.MethodDelete, base+"/staff/"+list[0].ID, console, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("remove status = %d", rec.Code)
	}
	expectError(t, env.do(t, http.MethodGet, "/api/guest/staff/summary", staff, nil), http.StatusNotFound, "E_NOT_FOUND")
}
