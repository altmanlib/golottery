package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func newRouter(readiness func(context.Context) error, proxies ...string) *echo.Echo {
	return NewRouter(Deps{Readiness: readiness, TrustedProxies: proxies})
}

func TestReadyzSuccessAndFailure(t *testing.T) {
	okRouter := newRouter(func(context.Context) error { return nil })
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	okRouter.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("ready ok: status=%d body=%q", rec.Code, rec.Body.String())
	}
	assertSecurityHeaders(t, rec.Header())

	bad := newRouter(func(context.Context) error { return errors.New("db down") })
	rec = httptest.NewRecorder()
	bad.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready fail status = %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("ready fail body = %q", rec.Body.String())
	}
}

func TestUnknownPathEmpty404(t *testing.T) {
	router := newRouter(func(context.Context) error { return nil })
	req := httptest.NewRequest(http.MethodGet, "/nope", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.Len() != 0 {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestRequestIDRejectsUntrustedAndAcceptsTrusted(t *testing.T) {
	untrusted := newRouter(func(context.Context) error { return nil })
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	req.Header.Set(echo.HeaderXRequestID, "client-forged-id")
	req.RemoteAddr = "203.0.113.9:12345"
	rec := httptest.NewRecorder()
	untrusted.ServeHTTP(rec, req)
	if got := rec.Header().Get(echo.HeaderXRequestID); got == "" || got == "client-forged-id" {
		t.Fatalf("untrusted id = %q", got)
	}

	trusted := newRouter(func(context.Context) error { return nil }, "127.0.0.1/32")
	req = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	req.Header.Set(echo.HeaderXRequestID, "proxy-passed-id-1")
	req.RemoteAddr = "127.0.0.1:54321"
	rec = httptest.NewRecorder()
	trusted.ServeHTTP(rec, req)
	if got := rec.Header().Get(echo.HeaderXRequestID); got != "proxy-passed-id-1" {
		t.Fatalf("trusted id = %q", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	req.Header.Set(echo.HeaderXRequestID, strings.Repeat("a", 65))
	req.RemoteAddr = "127.0.0.1:1"
	rec = httptest.NewRecorder()
	trusted.ServeHTTP(rec, req)
	if got := rec.Header().Get(echo.HeaderXRequestID); got == strings.Repeat("a", 65) {
		t.Fatal("overlong id accepted")
	}
}

func assertSecurityHeaders(t *testing.T, h http.Header) {
	t.Helper()
	want := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "no-referrer",
		"Cache-Control":           "no-store",
		"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'",
	}
	for key, value := range want {
		if h.Get(key) != value {
			t.Fatalf("%s = %q, want %q", key, h.Get(key), value)
		}
	}
}
