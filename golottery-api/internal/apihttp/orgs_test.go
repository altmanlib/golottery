package apihttp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"golottery/api/internal/platform"
)

type orgBody struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Status       string    `json:"status"`
	EventCredits int       `json:"event_credits"`
	MaxAttendees int       `json:"max_attendees"`
}

func (e platformEnv) createOrg(t *testing.T, token string, credits int) orgBody {
	t.Helper()
	rec := e.do(t, http.MethodPost, "/api/platform/orgs", token, map[string]any{
		"name": "Acme", "contact": "Li", "event_credits": credits, "max_attendees": 800,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body = %s", rec.Code, rec.Body.String())
	}
	return decode[orgBody](t, rec)
}

func TestOrgsRequirePlatformToken(t *testing.T) {
	env := newPlatformEnv(t)
	host, err := env.tokens.IssueHost(context.Background(), "event-1", "")
	if err != nil {
		t.Fatal(err)
	}
	console, err := env.tokens.IssueConsole(context.Background(), "admin-1", "")
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	routes := []struct{ method, path string }{
		{http.MethodGet, "/api/platform/orgs"},
		{http.MethodPost, "/api/platform/orgs"},
		{http.MethodGet, "/api/platform/orgs/" + id.String()},
		{http.MethodPost, "/api/platform/orgs/" + id.String() + "/disable"},
		{http.MethodPost, "/api/platform/orgs/" + id.String() + "/enable"},
		{http.MethodPost, "/api/platform/orgs/" + id.String() + "/credits"},
		{http.MethodPost, "/api/platform/orgs/" + id.String() + "/max-attendees"},
	}
	for _, route := range routes {
		for name, bearer := range map[string]string{"none": "", "host": host.Plain, "console": console.Plain} {
			t.Run(fmt.Sprintf("%s %s %s", route.method, route.path, name), func(t *testing.T) {
				expectError(t, env.do(t, route.method, route.path, bearer, map[string]any{}), http.StatusUnauthorized, "E_UNAUTHORIZED")
			})
		}
	}
}

func TestOrgLifecycle(t *testing.T) {
	env := newPlatformEnv(t)
	token := env.mustLogin(t)
	created := env.createOrg(t, token, 2)
	base := "/api/platform/orgs/" + created.ID.String()

	rec := env.do(t, http.MethodPost, base+"/credits", token, map[string]any{"delta": 3, "reason": "补充"})
	if rec.Code != http.StatusOK {
		t.Fatalf("credits status = %d body = %s", rec.Code, rec.Body.String())
	}
	entry := decode[struct {
		BalanceAfter int    `json:"balance_after"`
		OperatorType string `json:"operator_type"`
		OperatorID   string `json:"operator_id"`
	}](t, rec)
	var operator platform.User
	if err := env.db.Where("username = ?", testOperator).Take(&operator).Error; err != nil {
		t.Fatal(err)
	}
	if entry.BalanceAfter != 5 || entry.OperatorType != "platform" || entry.OperatorID != operator.ID.String() {
		t.Fatalf("entry = %+v", entry)
	}

	expectError(t, env.do(t, http.MethodPost, base+"/credits", token, map[string]any{"delta": -6, "reason": "超扣"}), http.StatusConflict, "E_CONFLICT")
	expectError(t, env.do(t, http.MethodPost, base+"/credits", token, map[string]any{"delta": 0, "reason": "零"}), http.StatusBadRequest, "E_BAD_REQUEST")

	for _, action := range []string{"/disable", "/disable", "/enable"} {
		if rec := env.do(t, http.MethodPost, base+action, token, nil); rec.Code != http.StatusNoContent {
			t.Fatalf("%s status = %d body = %s", action, rec.Code, rec.Body.String())
		}
	}
	if rec := env.do(t, http.MethodPost, base+"/max-attendees", token, map[string]any{"max_attendees": 2000}); rec.Code != http.StatusNoContent {
		t.Fatalf("max-attendees status = %d body = %s", rec.Code, rec.Body.String())
	}

	rec = env.do(t, http.MethodGet, base, token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d body = %s", rec.Code, rec.Body.String())
	}
	detail := decode[struct {
		Org    orgBody `json:"org"`
		Ledger []struct {
			Delta int `json:"delta"`
		} `json:"ledger"`
	}](t, rec)
	if detail.Org.EventCredits != 5 || detail.Org.MaxAttendees != 2000 || detail.Org.Status != "active" {
		t.Fatalf("org = %+v", detail.Org)
	}
	if len(detail.Ledger) != 2 || detail.Ledger[0].Delta != 3 || detail.Ledger[1].Delta != 2 {
		t.Fatalf("ledger = %+v", detail.Ledger)
	}
}

func TestOrgErrors(t *testing.T) {
	env := newPlatformEnv(t)
	token := env.mustLogin(t)
	missing := "/api/platform/orgs/" + uuid.New().String()

	expectError(t, env.do(t, http.MethodGet, missing, token, nil), http.StatusNotFound, "E_NOT_FOUND")
	expectError(t, env.do(t, http.MethodPost, missing+"/disable", token, nil), http.StatusNotFound, "E_NOT_FOUND")
	expectError(t, env.do(t, http.MethodPost, missing+"/credits", token, map[string]any{"delta": 1, "reason": "x"}), http.StatusNotFound, "E_NOT_FOUND")
	expectError(t, env.do(t, http.MethodPost, missing+"/max-attendees", token, map[string]any{"max_attendees": 100}), http.StatusNotFound, "E_NOT_FOUND")

	create := func(body map[string]any) *httptest.ResponseRecorder {
		return env.do(t, http.MethodPost, "/api/platform/orgs", token, body)
	}
	expectError(t, create(map[string]any{"name": " ", "event_credits": 1, "max_attendees": 100}), http.StatusBadRequest, "E_NAME_REQUIRED")
	expectError(t, create(map[string]any{"name": "A", "event_credits": -1, "max_attendees": 100}), http.StatusBadRequest, "E_BAD_REQUEST")
	expectError(t, create(map[string]any{"name": "A", "event_credits": 1, "max_attendees": 0}), http.StatusBadRequest, "E_BAD_REQUEST")
}

func TestListOrgsPaging(t *testing.T) {
	env := newPlatformEnv(t)
	token := env.mustLogin(t)
	for range 3 {
		env.createOrg(t, token, 1)
	}
	type pageBody struct {
		Items []orgBody `json:"items"`
		Total int       `json:"total"`
	}
	cases := map[string]int{
		"/api/platform/orgs":                     3,
		"/api/platform/orgs?limit=2":             2,
		"/api/platform/orgs?offset=2&limit=2":    1,
		"/api/platform/orgs?limit=0&offset=-5":   3,
		"/api/platform/orgs?limit=1000&offset=0": 3,
	}
	for path, want := range cases {
		rec := env.do(t, http.MethodGet, path, token, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d body = %s", path, rec.Code, rec.Body.String())
		}
		body := decode[pageBody](t, rec)
		if body.Total != 3 || len(body.Items) != want {
			t.Fatalf("%s total = %d items = %d, want 3 and %d", path, body.Total, len(body.Items), want)
		}
	}
}
