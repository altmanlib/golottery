package apihttp

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	api "golottery/api/api"
	"golottery/api/internal/auth"
	"golottery/api/internal/event"
	"golottery/api/internal/guest"
	"golottery/api/internal/httpapi"
	"golottery/api/internal/org"
	"golottery/api/internal/platform"
	"golottery/api/internal/ratelimit"
	"golottery/api/internal/redisx"
	"golottery/api/internal/store"
	"golottery/api/internal/wechat"
	"golottery/api/internal/wechat/wechattest"
)

const (
	testOperator = "ops"
	testPassword = "correct-password"
)

type platformEnv struct {
	engine *echo.Echo
	db     *gorm.DB
	tokens *auth.TokenIssuer
	wechat *wechattest.Server
	logs   *syncBuffer
}

// syncBuffer collects log output from concurrent handlers.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func newPlatformEnv(t *testing.T) platformEnv {
	t.Helper()
	db := store.OpenTest(t)
	store.Reset(t, db, &platform.User{}, &auth.APIToken{}, &auth.LoginAttempt{}, &guest.Attempt{}, &guest.ManualRequest{}, &guest.Staff{}, &guest.Session{}, &event.Prize{}, &event.Attendee{}, &org.LedgerEntry{}, &event.Event{}, &org.User{}, &org.Quota{}, &org.Org{})
	hash, err := auth.HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := platform.Seed(context.Background(), db.Gorm, testOperator, hash); err != nil {
		t.Fatal(err)
	}
	tokens := auth.NewTokenIssuer(db.Gorm, time.Hour, time.Hour, time.Hour)
	limiter := auth.NewLoginLimiter(db.Gorm, 3, 15*time.Minute)
	fake := wechattest.New()
	t.Cleanup(fake.Close)
	logs := &syncBuffer{}
	engine := httpapi.NewRouter(httpapi.Deps{})
	mustRegister(t, engine, Deps{
		Wechat:   wechat.New(wechat.Config{AppID: "wx123", AppSecret: "secret", BaseURL: fake.URL, Redis: redisx.OpenTest(t)}),
		Logger:   slog.New(slog.NewTextHandler(logs, nil)),
		DB:       db.Gorm,
		Tokens:   tokens,
		Platform: platform.NewService(db.Gorm, tokens, limiter),
		Orgs:     org.NewService(db.Gorm),
		Accounts: org.NewAccounts(db.Gorm, tokens, limiter),
		Events:   event.NewService(db.Gorm),
		Guests:   guest.NewService(db.Gorm, guest.Config{Mode: guest.ModeWeb, Limiter: ratelimit.New(redisx.OpenTest(t), nil)}),
	})
	return platformEnv{engine: engine, db: db.Gorm, tokens: tokens, wechat: fake, logs: logs}
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
	secured, err := securedOperations(map[string]authenticator{schemePlatform: nil, schemeConsole: nil, schemeGuest: nil})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := securedOperations(map[string]authenticator{schemePlatform: nil}); err == nil {
		t.Fatal("a contract scheme without an authenticator was accepted")
	}
	doc, err := api.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	public := map[string]bool{"PlatformLogin": true, "OrganizationLogin": true, "GuestLogin": true}
	checked := 0
	for path, item := range doc.Paths.Map() {
		for _, op := range item.Operations() {
			name := goOperationName(op.OperationID)
			want := ""
			switch {
			case public[name]:
			case strings.HasPrefix(path, "/api/platform/"):
				want = schemePlatform
			case strings.HasPrefix(path, "/api/organization/"):
				want = schemeConsole
			case strings.HasPrefix(path, "/api/guest/"):
				want = schemeGuest
			}
			if secured[name] != want {
				t.Errorf("%s %s requires %q, want %q", path, name, secured[name], want)
			}
			checked++
		}
	}
	if checked < 40 {
		t.Fatalf("checked only %d operations", checked)
	}
}

func mustRegister(t *testing.T, engine *echo.Echo, deps Deps) {
	t.Helper()
	if err := Register(engine, deps); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
}
