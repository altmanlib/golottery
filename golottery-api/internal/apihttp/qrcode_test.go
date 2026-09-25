package apihttp

import (
	"bytes"
	"image/png"
	"net/http"
	"strings"
	"testing"
	"time"

	"golottery/api/internal/auth"
	"golottery/api/internal/event"
	"golottery/api/internal/httpapi"
	"golottery/api/internal/org"
)

func TestEventQRCode(t *testing.T) {
	env := newPlatformEnv(t)
	ops := env.mustLogin(t)
	console, _ := env.adminSession(t, ops, "a@example.com", 1)
	ev := env.createEvent(t, console)
	path := "/api/organization/events/" + ev.ID.String() + "/entry/qrcode"

	rec := env.do(t, http.MethodGet, path, console, nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("qrcode status = %d type = %q body = %s", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}
	if _, err := png.Decode(bytes.NewReader(rec.Body.Bytes())); err != nil {
		t.Fatalf("body is not a PNG: %v", err)
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("content disposition = %q", rec.Header().Get("Content-Disposition"))
	}
	calls := env.wechat.QRCalls()
	if len(calls) != 1 || calls[0].Scene != ev.PublicID || calls[0].Page != "pages/index/index" {
		t.Fatalf("wechat calls = %+v", calls)
	}

	env.wechat.QRErrCode = 41030
	expectError(t, env.do(t, http.MethodGet, path, console, nil), http.StatusInternalServerError, "E_INTERNAL")
	if logs := env.logs.String(); !strings.Contains(logs, "errcode 41030") || !strings.Contains(logs, "GetEventQRCode") {
		t.Fatalf("logs do not carry the WeChat errcode: %s", logs)
	}

	mine, _ := env.adminSession(t, ops, "b@example.com", 1)
	expectError(t, env.do(t, http.MethodGet, path, mine, nil), http.StatusNotFound, "E_NOT_FOUND")
}

func TestEventQRCodeWithoutWechatConfig(t *testing.T) {
	env := newPlatformEnv(t)
	ops := env.mustLogin(t)
	console, _ := env.adminSession(t, ops, "a@example.com", 1)
	ev := env.createEvent(t, console)

	// Same data, a server without WeChat credentials.
	engine := httpapi.NewRouter(httpapi.Deps{})
	mustRegister(t, engine, Deps{DB: env.db, Tokens: env.tokens, Accounts: accountsFor(env), Events: eventsFor(env)})
	bare := env
	bare.engine = engine
	expectError(t, bare.do(t, http.MethodGet, "/api/organization/events/"+ev.ID.String()+"/entry/qrcode", console, nil),
		http.StatusServiceUnavailable, "E_WECHAT_NOT_CONFIGURED")
}

func accountsFor(env platformEnv) *org.Accounts {
	return org.NewAccounts(env.db, env.tokens, auth.NewLoginLimiter(env.db, 3, time.Minute))
}

func eventsFor(env platformEnv) *event.Service { return event.NewService(env.db) }
