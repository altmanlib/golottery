package apihttp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/labstack/echo/v4"

	api "golottery/api/api"
	"golottery/api/internal/auth"
	"golottery/api/internal/bizerr"
	"golottery/api/internal/guest"
)

// Security schemes declared in openapi.yaml.
const (
	schemePlatform = "platformBearer"
	schemeConsole  = "consoleBearer"
	schemeGuest    = "guestBearer"
)

// authenticator checks a bearer token for one scheme and returns the request context
// carrying the caller, or a bizerr.
type authenticator func(ctx context.Context, plain string) (context.Context, error)

type (
	tokenKey struct{}
	guestKey struct{}
)

// tokenFrom returns the back-office token authenticated for the current request.
func tokenFrom(ctx context.Context) (auth.APIToken, error) {
	token, ok := ctx.Value(tokenKey{}).(auth.APIToken)
	if !ok {
		return auth.APIToken{}, bizerr.New(bizerr.CodeUnauthorized)
	}
	return token, nil
}

// guestFrom returns the guest authenticated for the current request.
func guestFrom(ctx context.Context) (guest.Guest, error) {
	g, ok := ctx.Value(guestKey{}).(guest.Guest)
	if !ok {
		return guest.Guest{}, bizerr.New(bizerr.CodeUnauthorized)
	}
	return g, nil
}

// apiTokenAuth accepts api_tokens rows of typ; resolve, when set, also checks the owner
// (for example that a console admin and its organization are still active).
func apiTokenAuth(tokens *auth.TokenIssuer, typ string, resolve func(context.Context, auth.APIToken) (context.Context, error)) authenticator {
	return func(ctx context.Context, plain string) (context.Context, error) {
		token, err := tokens.Lookup(ctx, typ, plain)
		if errors.Is(err, auth.ErrTokenNotFound) {
			return nil, bizerr.New(bizerr.CodeUnauthorized)
		}
		if err != nil {
			return nil, bizerr.Wrap(bizerr.CodeInternal, err)
		}
		ctx = context.WithValue(ctx, tokenKey{}, token)
		if resolve != nil {
			return resolve(ctx, token)
		}
		return ctx, nil
	}
}

// guestAuth accepts guest session tokens.
func guestAuth(guests *guest.Service) authenticator {
	return func(ctx context.Context, plain string) (context.Context, error) {
		if guests == nil {
			return nil, bizerr.New(bizerr.CodeUnauthorized)
		}
		g, err := guests.Lookup(ctx, plain)
		if errors.Is(err, guest.ErrSessionNotFound) {
			return nil, bizerr.New(bizerr.CodeUnauthorized)
		}
		if err != nil {
			return nil, bizerr.Wrap(bizerr.CodeInternal, err)
		}
		return context.WithValue(ctx, guestKey{}, g), nil
	}
}

// securedOperations reads, from the embedded contract, the security scheme each operation
// requires. Keys are the Go operation names the strict middleware receives.
func securedOperations(known map[string]authenticator) (map[string]string, error) {
	doc, err := api.GetSpec()
	if err != nil {
		return nil, fmt.Errorf("apihttp: load spec: %w", err)
	}
	out := map[string]string{}
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			if op.Security == nil {
				continue
			}
			for _, requirement := range *op.Security {
				for scheme := range requirement {
					if _, ok := known[scheme]; !ok {
						return nil, fmt.Errorf("apihttp: %s %s uses unknown security scheme %q", method, path, scheme)
					}
					out[goOperationName(op.OperationID)] = scheme
				}
			}
		}
	}
	return out, nil
}

func goOperationName(operationID string) string {
	if operationID == "" {
		return ""
	}
	return strings.ToUpper(operationID[:1]) + operationID[1:]
}

// authenticate requires a valid bearer token of the declared scheme before secured operations.
func authenticate(secured map[string]string, auths map[string]authenticator) api.StrictMiddlewareFunc {
	return func(next api.StrictHandlerFunc, operationID string) api.StrictHandlerFunc {
		scheme, ok := secured[operationID]
		if !ok {
			return next
		}
		check := auths[scheme]
		return func(c echo.Context, request any) (any, error) {
			plain, ok := bearerToken(c.Request().Header.Get(echo.HeaderAuthorization))
			if !ok {
				return nil, bizerr.New(bizerr.CodeUnauthorized)
			}
			ctx, err := check(c.Request().Context(), plain)
			if err != nil {
				return nil, err
			}
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c, request)
		}
	}
}

func bearerToken(header string) (string, bool) {
	scheme, value, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	value = strings.TrimSpace(value)
	return value, value != ""
}
