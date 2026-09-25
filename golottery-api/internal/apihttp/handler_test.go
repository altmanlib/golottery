package apihttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"golottery/api/internal/httpapi"
	"golottery/api/internal/redisx"
	"golottery/api/internal/store"
)

func TestHealthzDatabaseUp(t *testing.T) {
	db := store.OpenTest(t)
	engine := httpapi.NewRouter(httpapi.Deps{
		Readiness: func(ctx context.Context) error { return db.Ping(ctx) },
	})
	mustRegister(t, engine, Deps{DB: db.Gorm})

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		OK bool   `json:"ok"`
		DB string `json:"db"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	if !body.OK || body.DB != "up" {
		t.Fatalf("body = %+v", body)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("security header missing")
	}
}

func TestHealthzDatabaseDown(t *testing.T) {
	engine := httpapi.NewRouter(httpapi.Deps{
		Readiness: func(context.Context) error { return context.Canceled },
	})
	mustRegister(t, engine, Deps{})

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestOpenAPIStillMounted(t *testing.T) {
	engine := httpapi.NewRouter(httpapi.Deps{})
	mustRegister(t, engine, Deps{})
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestHealthzReportsRedisWithoutFailingOK(t *testing.T) {
	db := store.OpenTest(t)
	cases := map[string]struct {
		deps Deps
		want string
	}{
		"up":      {Deps{DB: db.Gorm, Redis: redisx.OpenTest(t)}, "up"},
		"down":    {Deps{DB: db.Gorm, Redis: redisx.Unreachable()}, "down"},
		"skipped": {Deps{DB: db.Gorm}, "skipped"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			engine := httpapi.NewRouter(httpapi.Deps{})
			mustRegister(t, engine, tc.deps)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
			}
			var body struct {
				OK    bool   `json:"ok"`
				Redis string `json:"redis"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if !body.OK || body.Redis != tc.want {
				t.Fatalf("body = %+v, want ok and redis=%s", body, tc.want)
			}
		})
	}
}
