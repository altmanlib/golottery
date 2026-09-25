package apihttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type createdAdmin struct {
	User struct {
		ID     uuid.UUID `json:"id"`
		Email  string    `json:"email"`
		Status string    `json:"status"`
	} `json:"user"`
	Password string `json:"password"`
}

func (e platformEnv) createAdmin(t *testing.T, platformToken string, orgID uuid.UUID, email string) createdAdmin {
	t.Helper()
	rec := e.do(t, http.MethodPost, "/api/platform/orgs/"+orgID.String()+"/users", platformToken, map[string]string{"name": "管理员", "email": email})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create admin status = %d body = %s", rec.Code, rec.Body.String())
	}
	return decode[createdAdmin](t, rec)
}

func (e platformEnv) adminLogin(t *testing.T, email, password string) (string, uuid.UUID) {
	t.Helper()
	rec := e.do(t, http.MethodPost, "/api/organization/login", "", map[string]string{"email": email, "password": password})
	if rec.Code != http.StatusOK {
		t.Fatalf("admin login status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := decode[struct {
		Token string    `json:"token"`
		OrgID uuid.UUID `json:"org_id"`
	}](t, rec)
	return body.Token, body.OrgID
}

func TestOrganizationSessionIsolation(t *testing.T) {
	env := newPlatformEnv(t)
	ops := env.mustLogin(t)
	acme := env.createOrg(t, ops, 1)
	other := env.createOrg(t, ops, 1)
	admin := env.createAdmin(t, ops, acme.ID, "a@example.com")
	env.createAdmin(t, ops, other.ID, "b@example.com")

	console, orgID := env.adminLogin(t, "a@example.com", admin.Password)
	if orgID != acme.ID {
		t.Fatalf("login org_id = %s, want %s", orgID, acme.ID)
	}

	rec := env.do(t, http.MethodGet, "/api/organization/me", console, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("me status = %d body = %s", rec.Code, rec.Body.String())
	}
	me := decode[struct {
		Email   string    `json:"email"`
		OrgID   uuid.UUID `json:"org_id"`
		OrgName string    `json:"org_name"`
	}](t, rec)
	if me.Email != "a@example.com" || me.OrgID != acme.ID || me.OrgName != "Acme" {
		t.Fatalf("me = %+v", me)
	}

	expectError(t, env.do(t, http.MethodGet, "/api/platform/me", console, nil), http.StatusUnauthorized, "E_UNAUTHORIZED")
	expectError(t, env.do(t, http.MethodGet, "/api/platform/orgs", console, nil), http.StatusUnauthorized, "E_UNAUTHORIZED")
	expectError(t, env.do(t, http.MethodGet, "/api/organization/me", ops, nil), http.StatusUnauthorized, "E_UNAUTHORIZED")

	// Disabling the organization cuts the admin off immediately.
	if rec := env.do(t, http.MethodPost, "/api/platform/orgs/"+acme.ID.String()+"/disable", ops, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("disable org status = %d", rec.Code)
	}
	expectError(t, env.do(t, http.MethodGet, "/api/organization/me", console, nil), http.StatusUnauthorized, "E_UNAUTHORIZED")
	expectError(t, env.do(t, http.MethodPost, "/api/organization/login", "", map[string]string{"email": "a@example.com", "password": admin.Password}), http.StatusUnauthorized, "E_INVALID_CREDENTIALS")
}

func TestPlatformManagesAdmins(t *testing.T) {
	env := newPlatformEnv(t)
	ops := env.mustLogin(t)
	acme := env.createOrg(t, ops, 1)
	base := "/api/platform/orgs/" + acme.ID.String() + "/users"

	admin := env.createAdmin(t, ops, acme.ID, " A@Example.com ")
	if admin.User.Email != "a@example.com" || admin.User.Status != "active" || admin.Password == "" {
		t.Fatalf("created = %+v", admin)
	}
	expectError(t, env.do(t, http.MethodPost, base, ops, map[string]string{"name": "x", "email": "a@example.com"}), http.StatusConflict, "E_CONFLICT")
	expectError(t, env.do(t, http.MethodPost, base, ops, map[string]string{"name": "x", "email": "bad"}), http.StatusBadRequest, "E_BAD_REQUEST")
	expectError(t, env.do(t, http.MethodPost, "/api/platform/orgs/"+uuid.New().String()+"/users", ops, map[string]string{"name": "x", "email": "c@example.com"}), http.StatusNotFound, "E_NOT_FOUND")

	rec := env.do(t, http.MethodGet, base, ops, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d", rec.Code)
	}
	if body := rec.Body.String(); len(decode[[]map[string]any](t, rec)) != 1 || strings.Contains(body, "password") {
		t.Fatalf("list body = %s", body)
	}

	console, _ := env.adminLogin(t, "a@example.com", admin.Password)
	userPath := base + "/" + admin.User.ID.String()

	rec = env.do(t, http.MethodPost, userPath+"/reset-password", ops, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("reset status = %d body = %s", rec.Code, rec.Body.String())
	}
	fresh := decode[struct {
		Password string `json:"password"`
	}](t, rec).Password
	expectError(t, env.do(t, http.MethodGet, "/api/organization/me", console, nil), http.StatusUnauthorized, "E_UNAUTHORIZED")
	console, _ = env.adminLogin(t, "a@example.com", fresh)

	if rec := env.do(t, http.MethodPost, userPath+"/disable", ops, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("disable status = %d", rec.Code)
	}
	expectError(t, env.do(t, http.MethodGet, "/api/organization/me", console, nil), http.StatusUnauthorized, "E_UNAUTHORIZED")
	if rec := env.do(t, http.MethodPost, userPath+"/enable", ops, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("enable status = %d", rec.Code)
	}
	env.adminLogin(t, "a@example.com", fresh)

	other := env.createOrg(t, ops, 1)
	wrongOrg := "/api/platform/orgs/" + other.ID.String() + "/users/" + admin.User.ID.String()
	expectError(t, env.do(t, http.MethodPost, wrongOrg+"/disable", ops, nil), http.StatusNotFound, "E_NOT_FOUND")
	expectError(t, env.do(t, http.MethodPost, wrongOrg+"/reset-password", ops, nil), http.StatusNotFound, "E_NOT_FOUND")
}

func TestOrganizationLogoutAndPassword(t *testing.T) {
	env := newPlatformEnv(t)
	ops := env.mustLogin(t)
	acme := env.createOrg(t, ops, 1)
	admin := env.createAdmin(t, ops, acme.ID, "a@example.com")

	first, _ := env.adminLogin(t, "a@example.com", admin.Password)
	second, _ := env.adminLogin(t, "a@example.com", admin.Password)
	if rec := env.do(t, http.MethodPost, "/api/organization/logout", first, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d", rec.Code)
	}
	expectError(t, env.do(t, http.MethodGet, "/api/organization/me", first, nil), http.StatusUnauthorized, "E_UNAUTHORIZED")

	change := func(current, next string) *httptest.ResponseRecorder {
		return env.do(t, http.MethodPost, "/api/organization/password", second, map[string]string{"current_password": current, "new_password": next})
	}
	expectError(t, change("wrong-password", "a-new-password"), http.StatusBadRequest, "E_CURRENT_PASSWORD_WRONG")
	expectError(t, change(admin.Password, admin.Password), http.StatusBadRequest, "E_PASSWORD_UNCHANGED")
	rec := change(admin.Password, "a-new-password")
	if rec.Code != http.StatusOK {
		t.Fatalf("change status = %d body = %s", rec.Code, rec.Body.String())
	}
	fresh := decode[struct {
		Token string `json:"token"`
	}](t, rec).Token
	expectError(t, env.do(t, http.MethodGet, "/api/organization/me", second, nil), http.StatusUnauthorized, "E_UNAUTHORIZED")
	if rec := env.do(t, http.MethodGet, "/api/organization/me", fresh, nil); rec.Code != http.StatusOK {
		t.Fatalf("new token status = %d", rec.Code)
	}
}
