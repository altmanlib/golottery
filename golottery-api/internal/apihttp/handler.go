// Package apihttp adapts the generated OpenAPI server onto the process router.
package apihttp

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"

	api "golottery/api/api"
	"golottery/api/internal/auth"
	"golottery/api/internal/bizerr"
	"golottery/api/internal/event"
	"golottery/api/internal/guest"
	"golottery/api/internal/org"
	"golottery/api/internal/platform"
	"golottery/api/internal/redisx"
	"golottery/api/internal/wechat"
)

// Deps are the collaborators behind the generated routes.
// A nil DB reports the database as down.
type Deps struct {
	DB       *gorm.DB
	Redis    *redis.Client
	Tokens   *auth.TokenIssuer
	Platform *platform.Service
	Orgs     *org.Service
	Accounts *org.Accounts
	Events   *event.Service
	Wechat   *wechat.Client
	Guests   *guest.Service
	Logger   *slog.Logger
}

// Server implements the generated strict interface.
type Server struct {
	db       *gorm.DB
	redis    *redis.Client
	platform *platform.Service
	orgs     *org.Service
	accounts *org.Accounts
	events   *event.Service
	wechat   *wechat.Client
	guests   *guest.Service
}

var _ api.StrictServerInterface = (*Server)(nil)

// Register mounts generated routes, enforces the contract's security
// requirements and translates bizerr values to JSON.
func Register(engine *echo.Echo, deps Deps) error {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	server := &Server{db: deps.DB, redis: deps.Redis, platform: deps.Platform, orgs: deps.Orgs, accounts: deps.Accounts, events: deps.Events, wechat: deps.Wechat, guests: deps.Guests}
	auths := map[string]authenticator{
		schemePlatform: apiTokenAuth(deps.Tokens, auth.TokenTypePlatform, nil),
		schemeConsole:  apiTokenAuth(deps.Tokens, auth.TokenTypeConsole, server.resolveAdmin),
		schemeGuest:    guestAuth(deps.Guests),
	}
	secured, err := securedOperations(auths)
	if err != nil {
		return err
	}
	// The last middleware wraps outermost, so recoverBizErr also renders authentication errors.
	handler := api.NewStrictHandler(server, []api.StrictMiddlewareFunc{
		authenticate(secured, auths),
		recoverBizErr(logger),
	})
	api.RegisterHandlers(engine.Group("", withClientIP), handler)
	return nil
}

// recoverBizErr renders business errors as JSON and logs the cause of every 5xx,
// which the response body never shows.
func recoverBizErr(logger *slog.Logger) api.StrictMiddlewareFunc {
	return func(next api.StrictHandlerFunc, operationID string) api.StrictHandlerFunc {
		return func(ctx echo.Context, request any) (any, error) {
			response, err := next(ctx, request)
			if err == nil {
				return response, nil
			}
			be, ok := bizerr.As(err)
			if !ok {
				logger.Error("request failed", "operation", operationID, "error", err)
				return nil, err
			}
			if bizerr.StatusOf(be.Code) >= http.StatusInternalServerError {
				logger.Error("request failed", "operation", operationID, "code", be.Code, "error", err)
			}
			return nil, writeBizErr(ctx, be)
		}
	}
}

func writeBizErr(c echo.Context, be *bizerr.Error) error {
	if c.Response().Committed {
		return nil
	}
	return c.JSON(bizerr.StatusOf(be.Code), map[string]string{
		"code":    string(be.Code),
		"message": be.Message,
	})
}

// GetHealthz reports liveness and whether the database answers a ping.
func (s *Server) GetHealthz(ctx context.Context, _ api.GetHealthzRequestObject) (api.GetHealthzResponseObject, error) {
	dbStatus := api.HealthzDbDown
	ok := false
	if s.db != nil {
		sqlDB, err := s.db.DB()
		if err == nil {
			pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			err = sqlDB.PingContext(pingCtx)
			cancel()
		}
		if err == nil {
			dbStatus = api.HealthzDbUp
			ok = true
		}
	}
	redisStatus := api.HealthzRedisSkipped
	if s.redis != nil {
		redisStatus = api.HealthzRedisUp
		if err := redisx.Ping(ctx, s.redis); err != nil {
			redisStatus = api.HealthzRedisDown
		}
	}
	body := api.Healthz{
		Ok:      ok,
		Service: "golottery-api",
		Ts:      time.Now().UTC(),
		Db:      &dbStatus,
		Redis:   &redisStatus,
	}
	if !ok {
		return api.GetHealthz503JSONResponse(body), nil
	}
	return api.GetHealthz200JSONResponse(body), nil
}

// GetApiInfo returns service metadata.
func (s *Server) GetApiInfo(_ context.Context, _ api.GetApiInfoRequestObject) (api.GetApiInfoResponseObject, error) {
	return api.GetApiInfo200JSONResponse{
		Name:  "golottery",
		Phase: "M1",
		Stack: "echo+gorm+postgresql",
		Docs:  []string{"prd", "design", "roadmap"},
	}, nil
}

// GetOpenAPIJSON returns the embedded OpenAPI document as JSON.
func (s *Server) GetOpenAPIJSON(_ context.Context, _ api.GetOpenAPIJSONRequestObject) (api.GetOpenAPIJSONResponseObject, error) {
	doc, err := api.GetSpec()
	if err != nil {
		return nil, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	raw, err := doc.MarshalJSON()
	if err != nil {
		return nil, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return api.GetOpenAPIJSON200JSONResponse(payload), nil
}

// GetOpenAPIYaml returns the embedded OpenAPI document as YAML.
func (s *Server) GetOpenAPIYaml(_ context.Context, _ api.GetOpenAPIYamlRequestObject) (api.GetOpenAPIYamlResponseObject, error) {
	raw, err := api.GetSpecJSON()
	if err != nil {
		return nil, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	yamlBytes, err := yaml.Marshal(doc)
	if err != nil {
		return nil, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return api.GetOpenAPIYaml200ApplicationyamlResponse{
		Body:          bytes.NewReader(yamlBytes),
		ContentLength: int64(len(yamlBytes)),
	}, nil
}

// DBStatus is exported for tests that need the HTTP status of a health payload.
func DBStatus(ok bool) int {
	if ok {
		return http.StatusOK
	}
	return http.StatusServiceUnavailable
}
