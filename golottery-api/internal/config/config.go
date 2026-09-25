package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const minSessionSecretLen = 32

// Config holds the resolved runtime configuration.
type Config struct {
	DatabaseURL          string
	RedisURL             string
	AppHost              string
	AppPort              int
	SessionSecret        string
	TrustedProxies       []string
	PlatformUser         string
	PlatformPasswordHash string
	ConsoleSessionTTL    time.Duration
	HostSessionTTL       time.Duration
	PlatformSessionTTL   time.Duration
	LoginMaxFailures     int
	LoginWindow          time.Duration

	bootstrapValues map[string]ResolvedValue
}

// Source is the layer Bootstrap found a ScopeApp value in.
type Source string

const (
	// SourceEnv means .env or a process environment variable supplied the value.
	SourceEnv Source = "env"
	// SourceDefault means the registry default was used.
	SourceDefault Source = "default"
)

// ResolvedValue is one ScopeApp key as resolved before settings overrides.
type ResolvedValue struct {
	Value  string
	Source Source
}

// BootstrapValues returns the pre-settings value and source of every ScopeApp key.
func (c *Config) BootstrapValues() map[string]ResolvedValue {
	out := make(map[string]ResolvedValue, len(c.bootstrapValues))
	for key, value := range c.bootstrapValues {
		out[key] = value
	}
	return out
}

// Bootstrap resolves keys from defaults, .env and environment variables.
func Bootstrap() (*Config, error) {
	if err := overloadDotenv(); err != nil {
		return nil, err
	}

	cfg := &Config{bootstrapValues: make(map[string]ResolvedValue, len(Registry))}
	for _, spec := range Registry {
		if raw, ok := lookupNonEmpty(spec.Key); ok {
			if err := spec.Set(cfg, raw); err != nil {
				return nil, err
			}
			cfg.recordBootstrap(spec, raw, SourceEnv)
			continue
		}
		if spec.Default != "" {
			if err := spec.Set(cfg, spec.Default); err != nil {
				return nil, err
			}
			cfg.recordBootstrap(spec, spec.Default, SourceDefault)
		}
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) recordBootstrap(spec Spec, value string, source Source) {
	if spec.Scope != ScopeApp {
		return
	}
	c.bootstrapValues[spec.Key] = ResolvedValue{Value: value, Source: source}
}

// Apply overlays settings values onto ScopeApp keys.
// Unknown keys and ScopeInfra keys are reported through warn and ignored.
func (c *Config) Apply(overrides map[string]string, warn func(string, ...any)) error {
	if warn == nil {
		warn = func(string, ...any) {}
	}
	for key, value := range overrides {
		if value == "" {
			continue
		}
		spec, ok := Lookup(key)
		if !ok {
			warn("settings: ignoring unknown key %s", key)
			continue
		}
		if spec.Scope != ScopeApp {
			warn("settings: ignoring infrastructure key %s", key)
			continue
		}
		if err := spec.Set(c, value); err != nil {
			return fmt.Errorf("settings %s: %w", key, err)
		}
	}
	return nil
}

// Validate reports missing required keys and rejects a short SESSION_SECRET.
func (c *Config) Validate() error {
	var missing []string
	if strings.TrimSpace(c.DatabaseURL) == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if strings.TrimSpace(c.RedisURL) == "" {
		missing = append(missing, "REDIS_URL")
	}
	if strings.TrimSpace(c.SessionSecret) == "" {
		missing = append(missing, "SESSION_SECRET")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required config: %s", strings.Join(missing, ", "))
	}
	if len(c.SessionSecret) < minSessionSecretLen {
		return fmt.Errorf("SESSION_SECRET must be at least %d characters", minSessionSecretLen)
	}
	return nil
}

// Addr returns the HTTP listen address.
func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.AppHost, c.AppPort)
}

func overloadDotenv() error {
	err := godotenv.Overload()
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return fmt.Errorf("load .env: %w", err)
}

func lookupNonEmpty(key string) (string, bool) {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return "", false
	}
	return value, true
}
