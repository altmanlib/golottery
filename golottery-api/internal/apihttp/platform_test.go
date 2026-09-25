package apihttp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"golottery/api/internal/auth"
	"golottery/api/internal/httpapi"
	"golottery/api/internal/platform"
	"golottery/api/internal/store"
)

const (
	testOperator = "ops"
	testPassword = "correct-password"
)

type platformEnv struct {
	engine *echo.Echo
	db     *gorm.DB
	tokens *auth.TokenIssuer
}

func newPlatformEnv(t *testing.T) platformEnv {
	t.Helper()
	db := store.OpenTest(t)
	store.Reset(t, db, &platform.User{}, &auth.APIToken{}, &auth.LoginAttempt{})
	hash, err := auth.HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := platform.Seed(context.Background(), db.Gorm, testOperator, hash); err != nil {
		t.Fatal(err)
	}
	tokens := auth.NewTokenIssuer(db.Gorm, time.Hour, time.Hour, time.Hour)
	limiter := auth.NewLoginLimiter(db.Gorm, 3, 15*time.Minute)
	engine := httpapi.NewRouter(httpapi.Deps{})
	mustRegister(t, engine, Deps{
		DB:       db.Gorm,
		Tokens:   tokens,
		Platform: platform.NewService(db.Gorm, tokens, limiter),
	})
	return platformEnv{engine: engine, db: db.Gorm, tokens: tokens}
}

func (e platformEnv) do(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if token != "" {
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.engine.ServeHTTP(rec, req)
	return rec
}

func (e platformEnv) login(t *testing.T, username, password string) *httptest.ResponseRecorder {
	t.Helper()
	return e.do(t, http.MethodPost, "/api/platform/login", "", map[string]string{"username": username, "password": password})
}

func (e platformEnv) mustLogin(t *testing.T) string {
	t.Helper()
	rec := e.login(t, testOperator, testPassword)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d body = %s", rec.Code, rec.Body.String())
	}
	return decode[struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}](t, rec).Token
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}
	return out
}

func expectError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d body = %s, want %d", rec.Code, rec.Body.String(), status)
	}
	if got := decode[struct {
		Code string `json:"code"`
	}](t, rec).Code; got != code {
		t.Fatalf("code = %s, want %s", got, code)
	}
}

func TestPlatformLogin(t *testing.T) {
	env := newPlatformEnv(t)

	token := env.mustLogin(t)
	if _, err := env.tokens.Lookup(context.Background(), auth.TokenTypePlatform, token); err != nil {
		t.Fatalf("issued token is not a platform token: %v", err)
	}

	expectError(t, env.login(t, testOperator, "wrong-password"), http.StatusUnauthorized, "E_INVALID_CREDENTIALS")
	expectError(t, env.login(t, "nobody", testPassword), http.StatusUnauthorized, "E_INVALID_CREDENTIALS")
	expectError(t, env.login(t, strings.Repeat("长", 200), testPassword), http.StatusUnauthorized, "E_INVALID_CREDENTIALS")
}

func TestPlatformLoginRateLimited(t *testing.T) {
	env := newPlatformEnv(t)
	for range 3 {
		expectError(t, env.login(t, testOperator, "wrong-password"), http.StatusUnauthorized, "E_INVALID_CREDENTIALS")
	}
	rec := env.login(t, testOperator, testPassword)
	expectError(t, rec, http.StatusTooManyRequests, "E_TOO_MANY_ATTEMPTS")
	if msg := decode[struct {
		Message string `json:"message"`
	}](t, rec).Message; msg != "尝试次数过多，请 15 分钟后再试" {
		t.Fatalf("message = %q", msg)
	}
}

func TestPlatformLoginSuccessResetsFailures(t *testing.T) {
	env := newPlatformEnv(t)
	for range 2 {
		env.login(t, testOperator, "wrong-password")
	}
	env.mustLogin(t)
	for range 2 {
		expectError(t, env.login(t, testOperator, "wrong-password"), http.StatusUnauthorized, "E_INVALID_CREDENTIALS")
	}
	env.mustLogin(t)
}

func TestPlatformMe(t *testing.T) {
	env := newPlatformEnv(t)
	token := env.mustLogin(t)

	rec := env.do(t, http.MethodGet, "/api/platform/me", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := decode[struct {
		Username string `json:"username"`
	}](t, rec).Username; got != testOperator {
		t.Fatalf("username = %q", got)
	}

	ctx := context.Background()
	host, err := env.tokens.IssueHost(ctx, "event-1", "")
	if err != nil {
		t.Fatal(err)
	}
	me := decode[struct {
		Username string `json:"username"`
	}](t, rec)
	var user platform.User
	if err := env.db.Where("username = ?", me.Username).Take(&user).Error; err != nil {
		t.Fatal(err)
	}
	expired, err := auth.NewTokenIssuer(env.db, time.Hour, time.Hour, time.Nanosecond).IssuePlatform(ctx, user.ID.String(), "")
	if err != nil {
		t.Fatal(err)
	}

	cases := map[string]string{
		"no token":      "",
		"host token":    host.Plain,
		"expired token": expired.Plain,
		"unknown token": "deadbeef",
	}
	for name, bearer := range cases {
		t.Run(name, func(t *testing.T) {
			expectError(t, env.do(t, http.MethodGet, "/api/platform/me", bearer, nil), http.StatusUnauthorized, "E_UNAUTHORIZED")
		})
	}
}

func TestPlatformLogout(t *testing.T) {
	env := newPlatformEnv(t)
	first := env.mustLogin(t)
	second := env.mustLogin(t)

	rec := env.do(t, http.MethodPost, "/api/platform/logout", first, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d body = %s", rec.Code, rec.Body.String())
	}
	expectError(t, env.do(t, http.MethodGet, "/api/platform/me", first, nil), http.StatusUnauthorized, "E_UNAUTHORIZED")
	if rec := env.do(t, http.MethodGet, "/api/platform/me", second, nil); rec.Code != http.StatusOK {
		t.Fatalf("other token status = %d, want 200", rec.Code)
	}
}

func TestChangePlatformPassword(t *testing.T) {
	env := newPlatformEnv(t)
	token := env.mustLogin(t)
	other := env.mustLogin(t)
	change := func(bearer, current, next string) *httptest.ResponseRecorder {
		return env.do(t, http.MethodPost, "/api/platform/password", bearer, map[string]string{
			"current_password": current,
			"new_password":     next,
		})
	}

	expectError(t, change(token, testPassword, "short"), http.StatusBadRequest, "E_PASSWORD_TOO_SHORT")
	expectError(t, change(token, testPassword, testPassword), http.StatusBadRequest, "E_PASSWORD_UNCHANGED")
	expectError(t, change(token, "wrong-password", "brand-new-password"), http.StatusBadRequest, "E_CURRENT_PASSWORD_WRONG")
	if rec := env.do(t, http.MethodGet, "/api/platform/me", token, nil); rec.Code != http.StatusOK {
		t.Fatalf("token after rejected change status = %d, want 200", rec.Code)
	}

	rec := change(token, testPassword, "brand-new-password")
	if rec.Code != http.StatusOK {
		t.Fatalf("change status = %d body = %s", rec.Code, rec.Body.String())
	}
	fresh := decode[struct {
		Token string `json:"token"`
	}](t, rec).Token

	for _, old := range []string{token, other} {
		expectError(t, env.do(t, http.MethodGet, "/api/platform/me", old, nil), http.StatusUnauthorized, "E_UNAUTHORIZED")
	}
	if rec := env.do(t, http.MethodGet, "/api/platform/me", fresh, nil); rec.Code != http.StatusOK {
		t.Fatalf("new token status = %d, want 200", rec.Code)
	}
	expectError(t, env.login(t, testOperator, testPassword), http.StatusUnauthorized, "E_INVALID_CREDENTIALS")
	if rec := env.login(t, testOperator, "brand-new-password"); rec.Code != http.StatusOK {
		t.Fatalf("login with new password status = %d", rec.Code)
	}
}

func TestSecuredOperationsFollowContract(t *testing.T) {
	secured, err := securedOperations()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"PlatformLogout":         auth.TokenTypePlatform,
		"GetPlatformMe":          auth.TokenTypePlatform,
		"ChangePlatformPassword": auth.TokenTypePlatform,
	}
	if len(secured) != len(want) {
		t.Fatalf("secured = %v, want %v", secured, want)
	}
	for op, typ := range want {
		if secured[op] != typ {
			t.Fatalf("secured[%s] = %q, want %q", op, secured[op], typ)
		}
	}
}

func mustRegister(t *testing.T, engine *echo.Echo, deps Deps) {
	t.Helper()
	if err := Register(engine, deps); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
}
