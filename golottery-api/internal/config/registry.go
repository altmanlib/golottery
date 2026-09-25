package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Kind is the value type of a configuration key.
type Kind int

const (
	KindString Kind = iota
	KindInt
	KindCSV
)

// Scope decides which layers may supply a key.
type Scope int

const (
	// ScopeInfra keys describe process and storage wiring. They never live in settings.
	ScopeInfra Scope = iota
	// ScopeApp keys describe behaviour. The settings table is the highest-precedence source.
	ScopeApp
)

// Group organizes keys for display.
type Group string

const (
	GroupServer Group = "server"
	GroupAuth   Group = "auth"
)

// Spec describes one configuration key.
type Spec struct {
	Key      string
	Kind     Kind
	Group    Group
	Default  string
	Scope    Scope
	Secret   bool
	Required bool
	Set      func(*Config, string) error
}

// Registry is the single source of truth for configuration keys.
var Registry = []Spec{
	{
		Key: "DATABASE_URL", Kind: KindString, Group: GroupServer,
		Scope: ScopeInfra, Secret: true, Required: true,
		Set: func(c *Config, v string) error { c.DatabaseURL = v; return nil },
	},
	{
		Key: "APP_HOST", Kind: KindString, Group: GroupServer,
		Default: "127.0.0.1", Scope: ScopeInfra,
		Set: func(c *Config, v string) error { c.AppHost = v; return nil },
	},
	{
		Key: "APP_PORT", Kind: KindInt, Group: GroupServer,
		Default: "5568", Scope: ScopeInfra,
		Set: func(c *Config, v string) error {
			port, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid APP_PORT: %w", err)
			}
			if port <= 0 || port > 65535 {
				return fmt.Errorf("invalid APP_PORT: %d", port)
			}
			c.AppPort = port
			return nil
		},
	},
	{
		Key: "SESSION_SECRET", Kind: KindString, Group: GroupAuth,
		Scope: ScopeInfra, Secret: true, Required: true,
		Set: func(c *Config, v string) error { c.SessionSecret = v; return nil },
	},
	{
		Key: "TRUSTED_PROXIES", Kind: KindCSV, Group: GroupServer, Scope: ScopeInfra,
		Set: func(c *Config, v string) error { c.TrustedProxies = splitCSV(v); return nil },
	},
	{
		Key: "PLATFORM_USER", Kind: KindString, Group: GroupAuth, Scope: ScopeInfra,
		Set: func(c *Config, v string) error { c.PlatformUser = v; return nil },
	},
	{
		Key: "PLATFORM_PASSWORD_HASH", Kind: KindString, Group: GroupAuth,
		Scope: ScopeInfra, Secret: true,
		Set: func(c *Config, v string) error { c.PlatformPasswordHash = v; return nil },
	},
	{
		Key: "CONSOLE_SESSION_TTL", Kind: KindString, Group: GroupAuth,
		Default: "12h", Scope: ScopeApp,
		Set: durationSetter("CONSOLE_SESSION_TTL", func(c *Config, d time.Duration) { c.ConsoleSessionTTL = d }),
	},
	{
		Key: "HOST_SESSION_TTL", Kind: KindString, Group: GroupAuth,
		Default: "12h", Scope: ScopeApp,
		Set: durationSetter("HOST_SESSION_TTL", func(c *Config, d time.Duration) { c.HostSessionTTL = d }),
	},
	{
		Key: "PLATFORM_SESSION_TTL", Kind: KindString, Group: GroupAuth,
		Default: "8h", Scope: ScopeApp,
		Set: durationSetter("PLATFORM_SESSION_TTL", func(c *Config, d time.Duration) { c.PlatformSessionTTL = d }),
	},
	{
		Key: "LOGIN_MAX_FAILURES", Kind: KindInt, Group: GroupAuth,
		Default: "5", Scope: ScopeApp,
		Set: func(c *Config, v string) error {
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid LOGIN_MAX_FAILURES: %w", err)
			}
			if n <= 0 {
				return fmt.Errorf("invalid LOGIN_MAX_FAILURES: must be > 0")
			}
			c.LoginMaxFailures = n
			return nil
		},
	},
	{
		Key: "LOGIN_WINDOW", Kind: KindString, Group: GroupAuth,
		Default: "15m", Scope: ScopeApp,
		Set: durationSetter("LOGIN_WINDOW", func(c *Config, d time.Duration) { c.LoginWindow = d }),
	},
}

var registryByKey map[string]Spec

func init() {
	registryByKey = make(map[string]Spec, len(Registry))
	for _, spec := range Registry {
		registryByKey[spec.Key] = spec
	}
}

// Lookup returns the registry Spec for a canonical key.
func Lookup(key string) (Spec, bool) {
	spec, ok := registryByKey[key]
	return spec, ok
}

// Groups returns registry groups in declaration order.
func Groups() []Group {
	seen := make(map[Group]bool, 2)
	out := make([]Group, 0, 2)
	for _, spec := range Registry {
		if seen[spec.Group] {
			continue
		}
		seen[spec.Group] = true
		out = append(out, spec.Group)
	}
	return out
}

// CategoryByKey returns canonical key -> category for settings reconciliation.
func CategoryByKey() map[string]string {
	out := make(map[string]string, len(Registry))
	for _, spec := range Registry {
		out[spec.Key] = string(spec.Group)
	}
	return out
}

// ValidateAppValue checks that key is a ScopeApp registry entry and value parses.
func ValidateAppValue(key, value string) error {
	spec, ok := Lookup(key)
	if !ok {
		return fmt.Errorf("unknown key %q", key)
	}
	if spec.Scope != ScopeApp {
		return fmt.Errorf("%s is infrastructure config; set it via .env or environment", key)
	}
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s: empty value; use unset to clear", key)
	}
	var tmp Config
	if err := spec.Set(&tmp, value); err != nil {
		return err
	}
	return nil
}

// MaskSecret replaces secret values for display.
func MaskSecret(value string) string {
	if value == "" {
		return ""
	}
	return "****"
}

func durationSetter(key string, assign func(*Config, time.Duration)) func(*Config, string) error {
	return func(c *Config, v string) error {
		d, err := parsePositiveDuration(key, v)
		if err != nil {
			return err
		}
		assign(c, d)
		return nil
	}
}

func parsePositiveDuration(key, raw string) (time.Duration, error) {
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("invalid %s: must be > 0", key)
	}
	return d, nil
}

func splitCSV(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
