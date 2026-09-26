package apihttp

import (
	"net/http"
	"testing"
	"time"
)

type guestStatusT struct {
	Event struct {
		Name        string `json:"name"`
		Status      string `json:"status"`
		CheckinMode string `json:"checkin_mode"`
	} `json:"event"`
	Attendee *struct {
		Name          string `json:"name"`
		CheckedIn     bool   `json:"checked_in"`
		CheckinMethod string `json:"checkin_method"`
	} `json:"attendee"`
	Request *struct {
		Status string `json:"status"`
	} `json:"request"`
	StaffRole *string `json:"staff_role"`
}

// readyDirectEvent prepares a ready direct-mode event with one person and returns its public id.
func (e platformEnv) readyDirectEvent(t *testing.T) (console string, ev eventBodyT) {
	t.Helper()
	ops := e.mustLogin(t)
	console, _ = e.adminSession(t, ops, "a@example.com", 1)
	ev = e.createEvent(t, console)
	base := "/api/organization/events/" + ev.ID.String()
	e.do(t, http.MethodPatch, base, console, map[string]any{
		"checkin_mode": "direct", "checkin_start": time.Now().Add(-time.Hour), "checkin_end": time.Now().Add(time.Hour),
	})
	e.do(t, http.MethodPost, base+"/attendees", console, map[string]any{"name": "李雷", "phone": "13800001234"})
	if rec := e.do(t, http.MethodPatch, base, console, map[string]any{"status": "ready"}); rec.Code != http.StatusOK {
		t.Fatalf("ready status = %d body = %s", rec.Code, rec.Body.String())
	}
	return console, ev
}

func (e platformEnv) guestLogin(t *testing.T, publicID, device string) string {
	t.Helper()
	rec := e.do(t, http.MethodPost, "/api/guest/session", "", map[string]string{"public_id": publicID, "device_id": device})
	if rec.Code != http.StatusOK {
		t.Fatalf("guest login status = %d body = %s", rec.Code, rec.Body.String())
	}
	return decode[struct {
		Token string `json:"token"`
	}](t, rec).Token
}

func TestGuestFlowOverHTTP(t *testing.T) {
	env := newPlatformEnv(t)
	console, ev := env.readyDirectEvent(t)
	token := env.guestLogin(t, ev.PublicID, "device-aaaaaaaaaaaaaaaa")

	st := decode[guestStatusT](t, env.do(t, http.MethodGet, "/api/guest/checkin", token, nil))
	if st.Event.Name != "年会" || st.Event.CheckinMode != "direct" || st.Attendee != nil || st.StaffRole != nil {
		t.Fatalf("initial status = %+v", st)
	}
	expectError(t, env.do(t, http.MethodPost, "/api/guest/checkin", token, map[string]any{}), http.StatusBadRequest, "E_NOT_BOUND")
	expectError(t, env.do(t, http.MethodPost, "/api/guest/bind", token, map[string]string{"name": "李雷", "phone_last4": "0000"}), http.StatusBadRequest, "E_ATTENDEE_NOT_MATCHED")

	rec := env.do(t, http.MethodPost, "/api/guest/bind", token, map[string]string{"name": "李雷", "phone_last4": "1234"})
	if rec.Code != http.StatusOK || decode[guestStatusT](t, rec).Attendee.Name != "李雷" {
		t.Fatalf("bind status = %d body = %s", rec.Code, rec.Body.String())
	}
	rec = env.do(t, http.MethodPost, "/api/guest/checkin", token, map[string]any{})
	st = decode[guestStatusT](t, rec)
	if rec.Code != http.StatusOK || !st.Attendee.CheckedIn || st.Attendee.CheckinMethod != "direct" {
		t.Fatalf("checkin status = %d body = %s", rec.Code, rec.Body.String())
	}

	other := env.guestLogin(t, ev.PublicID, "device-bbbbbbbbbbbbbbbb")
	expectError(t, env.do(t, http.MethodPost, "/api/guest/bind", other, map[string]string{"name": "李雷", "phone_last4": "1234"}), http.StatusConflict, "E_ATTENDEE_TAKEN")
	rec = env.do(t, http.MethodPost, "/api/guest/manual-requests", other, map[string]string{"name": "李雷", "phone_last4": "1234", "reason": "被别人绑了"})
	if rec.Code != http.StatusCreated || decode[guestStatusT](t, rec).Request.Status != "pending" {
		t.Fatalf("request status = %d body = %s", rec.Code, rec.Body.String())
	}

	// Tokens stay in their lane.
	expectError(t, env.do(t, http.MethodGet, "/api/organization/me", token, nil), http.StatusUnauthorized, "E_UNAUTHORIZED")
	expectError(t, env.do(t, http.MethodGet, "/api/guest/checkin", console, nil), http.StatusUnauthorized, "E_UNAUTHORIZED")
	expectError(t, env.do(t, http.MethodGet, "/api/guest/checkin", "", nil), http.StatusUnauthorized, "E_UNAUTHORIZED")
	expectError(t, env.do(t, http.MethodPost, "/api/guest/session", "", map[string]string{"public_id": "nope", "device_id": "device-aaaaaaaaaaaaaaaa"}), http.StatusNotFound, "E_NOT_FOUND")
}
