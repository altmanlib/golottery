// Package httpapi owns the process HTTP surface: echo, middleware and readiness.
package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

const maxRequestIDLen = 64

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)

// Deps are the runtime collaborators required by the HTTP surface.
type Deps struct {
	Logger         *slog.Logger
	Readiness      func(context.Context) error
	TrustedProxies []string
}

// NewRouter builds echo with health readiness, security headers and an empty 404.
func NewRouter(deps Deps) *echo.Echo {
	log := deps.Logger
	if log == nil {
		log = slog.Default()
	}

	engine := echo.New()
	engine.HideBanner = true
	engine.HidePort = true
	engine.HTTPErrorHandler = silentHTTPErrorHandler
	if extractor := ipExtractor(deps.TrustedProxies); extractor != nil {
		engine.IPExtractor = extractor
	}
	engine.Use(recovery(log), requestID(deps.TrustedProxies), requestLogger(log), securityHeaders())
	engine.GET("/readyz", readyz(deps.Readiness))
	return engine
}

func silentHTTPErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}
	code := http.StatusInternalServerError
	if he, ok := err.(*echo.HTTPError); ok {
		code = he.Code
	}
	_ = c.NoContent(code)
}

func readyz(check func(context.Context) error) echo.HandlerFunc {
	return func(c echo.Context) error {
		if check == nil {
			return c.NoContent(http.StatusServiceUnavailable)
		}
		if err := check(c.Request().Context()); err != nil {
			return c.NoContent(http.StatusServiceUnavailable)
		}
		return c.String(http.StatusOK, "ok")
	}
}

func securityHeaders() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			h := c.Response().Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Referrer-Policy", "no-referrer")
			h.Set("Cache-Control", "no-store")
			h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
			return next(c)
		}
	}
}

func requestID(trustedProxies []string) echo.MiddlewareFunc {
	nets := parseCIDRs(trustedProxies)
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			rid := ""
			inbound := strings.TrimSpace(c.Request().Header.Get(echo.HeaderXRequestID))
			if inbound != "" && len(inbound) <= maxRequestIDLen && requestIDPattern.MatchString(inbound) && peerTrusted(c, nets) {
				rid = inbound
			}
			if rid == "" {
				rid = newRequestID()
			}
			c.Response().Header().Set(echo.HeaderXRequestID, rid)
			c.Set(echo.HeaderXRequestID, rid)
			return next(c)
		}
	}
}

func requestLogger(log *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			rid, _ := c.Get(echo.HeaderXRequestID).(string)
			log.Info("http request",
				"method", c.Request().Method,
				"path", c.Request().URL.Path,
				"status", c.Response().Status,
				"latency", time.Since(start).String(),
				"client_ip", c.RealIP(),
				"request_id", rid,
			)
			return err
		}
	}
}

func recovery(log *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			defer func() {
				if recovered := recover(); recovered != nil {
					log.Error("http panic", "error", recovered, "path", c.Request().URL.Path)
					if !c.Response().Committed {
						_ = c.NoContent(http.StatusInternalServerError)
					}
				}
			}()
			return next(c)
		}
	}
}

func ipExtractor(trustedProxies []string) echo.IPExtractor {
	nets := parseCIDRs(trustedProxies)
	if len(nets) == 0 {
		return echo.ExtractIPDirect()
	}
	opts := []echo.TrustOption{
		echo.TrustLoopback(false),
		echo.TrustLinkLocal(false),
		echo.TrustPrivateNet(false),
	}
	for _, network := range nets {
		opts = append(opts, echo.TrustIPRange(network))
	}
	return echo.ExtractIPFromXFFHeader(opts...)
}

func parseCIDRs(raw []string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if !strings.Contains(item, "/") {
			if ip := net.ParseIP(item); ip != nil {
				if ip.To4() != nil {
					item += "/32"
				} else {
					item += "/128"
				}
			}
		}
		_, network, err := net.ParseCIDR(item)
		if err != nil {
			continue
		}
		out = append(out, network)
	}
	return out
}

func peerTrusted(c echo.Context, nets []*net.IPNet) bool {
	if len(nets) == 0 {
		return false
	}
	host, _, err := net.SplitHostPort(c.Request().RemoteAddr)
	if err != nil {
		host = c.Request().RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, network := range nets {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func newRequestID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format("20060102150405.000000000")))
	}
	return hex.EncodeToString(buf[:])
}
