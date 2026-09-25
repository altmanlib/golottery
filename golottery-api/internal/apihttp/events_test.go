package apihttp

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func (e platformEnv) adminSession(t *testing.T, platformToken, email string, credits int) (string, uuid.UUID) {
	t.Helper()
	created := e.createOrg(t, platformToken, credits)
	admin := e.createAdmin(t, platformToken, created.ID, email)
	token, _ := e.adminLogin(t, email, admin.Password)
	return token, created.ID
}

type eventBodyT struct {
	ID             uuid.UUID `json:"id"`
	PublicID       string    `json:"public_id"`
	Status         string    `json:"status"`
	MaxAttendees   int       `json:"max_attendees"`
	AttendeeCount  int       `json:"attendee_count"`
	CreditConsumed bool      `json:"credit_consumed"`
}

func (e platformEnv) createEvent(t *testing.T, console string) eventBodyT {
	t.Helper()
	rec := e.do(t, http.MethodPost, "/api/organization/events", console, map[string]string{"name": "年会"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create event status = %d body = %s", rec.Code, rec.Body.String())
	}
	return decode[eventBodyT](t, rec)
}

func TestEventLifecycleOverHTTP(t *testing.T) {
	env := newPlatformEnv(t)
	ops := env.mustLogin(t)
	console, orgID := env.adminSession(t, ops, "a@example.com", 1)
	ev := env.createEvent(t, console)
	base := "/api/organization/events/" + ev.ID.String()
	if ev.Status != "draft" || ev.MaxAttendees != 800 || ev.CreditConsumed {
		t.Fatalf("created = %+v", ev)
	}

	expectError(t, env.do(t, http.MethodPatch, base, console, map[string]any{"status": "ready"}), http.StatusBadRequest, "E_EVENT_INCOMPLETE")

	rec := env.do(t, http.MethodPatch, base, console, map[string]any{
		"checkin_mode": "direct", "checkin_start": "2026-10-01T09:00:00+08:00", "checkin_end": "2026-10-01T12:00:00+08:00",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d body = %s", rec.Code, rec.Body.String())
	}
	if rec := env.do(t, http.MethodPost, base+"/attendees", console, map[string]any{"name": "李雷", "phone": "13800001234"}); rec.Code != http.StatusCreated {
		t.Fatalf("attendee status = %d body = %s", rec.Code, rec.Body.String())
	}
	if rec := env.do(t, http.MethodPost, base+"/prizes", console, map[string]any{"name": "一等奖", "quota": 1, "sort_no": 1}); rec.Code != http.StatusCreated {
		t.Fatalf("prize status = %d body = %s", rec.Code, rec.Body.String())
	}

	rec = env.do(t, http.MethodPatch, base, console, map[string]any{"status": "ready"})
	if rec.Code != http.StatusOK {
		t.Fatalf("ready status = %d body = %s", rec.Code, rec.Body.String())
	}
	ready := decode[eventBodyT](t, rec)
	if ready.Status != "ready" || !ready.CreditConsumed || ready.AttendeeCount != 1 {
		t.Fatalf("ready = %+v", ready)
	}

	me := decode[struct {
		EventCredits int `json:"event_credits"`
	}](t, env.do(t, http.MethodGet, "/api/organization/me", console, nil))
	if me.EventCredits != 0 {
		t.Fatalf("credits after ready = %d", me.EventCredits)
	}
	detail := decode[struct {
		Ledger []struct {
			Delta        int    `json:"delta"`
			EventID      string `json:"event_id"`
			OperatorType string `json:"operator_type"`
		} `json:"ledger"`
	}](t, env.do(t, http.MethodGet, "/api/platform/orgs/"+orgID.String(), ops, nil))
	if len(detail.Ledger) == 0 || detail.Ledger[0].Delta != -1 || detail.Ledger[0].EventID != ev.ID.String() || detail.Ledger[0].OperatorType != "console" {
		t.Fatalf("ledger = %+v", detail.Ledger)
	}

	second := env.createEvent(t, console)
	secondBase := "/api/organization/events/" + second.ID.String()
	env.do(t, http.MethodPatch, secondBase, console, map[string]any{"checkin_mode": "direct", "checkin_start": "2026-10-02T09:00:00Z", "checkin_end": "2026-10-02T10:00:00Z"})
	env.do(t, http.MethodPost, secondBase+"/attendees", console, map[string]any{"name": "韩梅梅", "phone": "5678"})
	expectError(t, env.do(t, http.MethodPatch, secondBase, console, map[string]any{"status": "ready"}), http.StatusConflict, "E_NO_EVENT_CREDITS")

	entry := decode[struct {
		PublicID string `json:"public_id"`
		Path     string `json:"path"`
	}](t, env.do(t, http.MethodGet, base+"/entry", console, nil))
	if entry.PublicID != ev.PublicID || entry.Path != "pages/index/index?e="+ev.PublicID {
		t.Fatalf("entry = %+v", entry)
	}
}

func TestEventsAreScopedToTheAdminsOrg(t *testing.T) {
	env := newPlatformEnv(t)
	ops := env.mustLogin(t)
	mine, _ := env.adminSession(t, ops, "a@example.com", 1)
	theirs, theirOrg := env.adminSession(t, ops, "b@example.com", 1)
	ev := env.createEvent(t, theirs)
	base := "/api/organization/events/" + ev.ID.String()

	expectError(t, env.do(t, http.MethodGet, base, mine, nil), http.StatusNotFound, "E_NOT_FOUND")
	expectError(t, env.do(t, http.MethodPatch, base, mine, map[string]any{"name": "x"}), http.StatusNotFound, "E_NOT_FOUND")
	expectError(t, env.do(t, http.MethodGet, base+"/attendees", mine, nil), http.StatusNotFound, "E_NOT_FOUND")
	expectError(t, env.do(t, http.MethodPost, base+"/prizes", mine, map[string]any{"name": "x", "quota": 1, "sort_no": 1}), http.StatusNotFound, "E_NOT_FOUND")
	list := decode[struct {
		Total int `json:"total"`
	}](t, env.do(t, http.MethodGet, "/api/organization/events", mine, nil))
	if list.Total != 0 {
		t.Fatalf("list leaked %d events", list.Total)
	}

	expectError(t, env.do(t, http.MethodGet, "/api/organization/events", ops, nil), http.StatusUnauthorized, "E_UNAUTHORIZED")

	if rec := env.do(t, http.MethodPost, "/api/platform/orgs/"+theirOrg.String()+"/disable", ops, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("disable status = %d", rec.Code)
	}
	expectError(t, env.do(t, http.MethodPost, "/api/organization/events", theirs, map[string]string{"name": "x"}), http.StatusUnauthorized, "E_UNAUTHORIZED")
}
