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
)

// schemeTokenTypes maps OpenAPI security schemes to the token type they accept.
var schemeTokenTypes = map[string]string{
	"platformBearer": auth.TokenTypePlatform,
}

type tokenKey struct{}

// tokenFrom returns the token authenticated for the current request.
func tokenFrom(ctx context.Context) (auth.APIToken, error) {
	token, ok := ctx.Value(tokenKey{}).(auth.APIToken)
	if !ok {
		return auth.APIToken{}, bizerr.New(bizerr.CodeUnauthorized)
	}
	return token, nil
}

// securedOperations reads, from the embedded contract, the token type each operation requires.
// Keys are the Go operation names the strict middleware receives.
func securedOperations() (map[string]string, error) {
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
					typ, ok := schemeTokenTypes[scheme]
					if !ok {
						return nil, fmt.Errorf("apihttp: %s %s uses unknown security scheme %q", method, path, scheme)
					}
					out[goOperationName(op.OperationID)] = typ
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

// authenticate requires a live bearer token of the declared type before secured operations.
func authenticate(tokens *auth.TokenIssuer, secured map[string]string) api.StrictMiddlewareFunc {
	return func(next api.StrictHandlerFunc, operationID string) api.StrictHandlerFunc {
		typ, ok := secured[operationID]
		if !ok {
			return next
		}
		return func(c echo.Context, request any) (any, error) {
			plain, ok := bearerToken(c.Request().Header.Get(echo.HeaderAuthorization))
			if !ok {
				return nil, bizerr.New(bizerr.CodeUnauthorized)
			}
			ctx := c.Request().Context()
			token, err := tokens.Lookup(ctx, typ, plain)
			if errors.Is(err, auth.ErrTokenNotFound) {
				return nil, bizerr.New(bizerr.CodeUnauthorized)
			}
			if err != nil {
				return nil, bizerr.Wrap(bizerr.CodeInternal, err)
			}
			c.SetRequest(c.Request().WithContext(context.WithValue(ctx, tokenKey{}, token)))
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
