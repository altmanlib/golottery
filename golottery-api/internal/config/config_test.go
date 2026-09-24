package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, spec := range Registry {
		t.Setenv(spec.Key, "")
		_ = os.Unsetenv(spec.Key)
	}
}

func chdirTemp(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
}

func writeRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://postgres:secret@127.0.0.1:15436/golottery?sslmode=disable")
	t.Setenv("SESSION_SECRET", "0123456789abcdef0123456789abcdef")
}

func TestBootstrapMissingRequiredNamesTheKey(t *testing.T) {
	clearConfigEnv(t)
	chdirTemp(t)
	t.Setenv("SESSION_SECRET", "0123456789abcdef0123456789abcdef")

	_, err := Bootstrap()
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("error = %v, want DATABASE_URL", err)
	}
}

func TestBootstrapRejectsShortSessionSecret(t *testing.T) {
	clearConfigEnv(t)
	chdirTemp(t)
	t.Setenv("DATABASE_URL", "postgres://postgres:secret@127.0.0.1:15436/golottery?sslmode=disable")
	t.Setenv("SESSION_SECRET", "too-short")

	_, err := Bootstrap()
	if err == nil || !strings.Contains(err.Error(), "SESSION_SECRET") {
		t.Fatalf("error = %v", err)
	}
}

func TestBootstrapDefaults(t *testing.T) {
	clearConfigEnv(t)
	chdirTemp(t)
	writeRequiredEnv(t)

	cfg, err := Bootstrap()
	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	if cfg.AppHost != "127.0.0.1" || cfg.AppPort != 5568 {
		t.Fatalf("addr = %s", cfg.Addr())
	}
	if cfg.ConsoleSessionTTL != 12*time.Hour || cfg.HostSessionTTL != 12*time.Hour {
		t.Fatalf("session ttl console=%s host=%s", cfg.ConsoleSessionTTL, cfg.HostSessionTTL)
	}
	if cfg.PlatformSessionTTL != 8*time.Hour {
		t.Fatalf("platform ttl = %s", cfg.PlatformSessionTTL)
	}
	if cfg.LoginMaxFailures != 5 || cfg.LoginWindow != 15*time.Minute {
		t.Fatalf("login limit = %d / %s", cfg.LoginMaxFailures, cfg.LoginWindow)
	}
	if got := cfg.BootstrapValues()["CONSOLE_SESSION_TTL"]; got.Source != SourceDefault || got.Value != "12h" {
		t.Fatalf("bootstrap console ttl = %+v", got)
	}
}

func TestEnvOverridesDefault(t *testing.T) {
	clearConfigEnv(t)
	chdirTemp(t)
	writeRequiredEnv(t)
	t.Setenv("CONSOLE_SESSION_TTL", "2h")

	cfg, err := Bootstrap()
	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	if cfg.ConsoleSessionTTL != 2*time.Hour {
		t.Fatalf("ttl = %s", cfg.ConsoleSessionTTL)
	}
	if cfg.BootstrapValues()["CONSOLE_SESSION_TTL"].Source != SourceEnv {
		t.Fatalf("source = %s", cfg.BootstrapValues()["CONSOLE_SESSION_TTL"].Source)
	}
}

func TestApplySettingsOverridesEnvButNotInfra(t *testing.T) {
	clearConfigEnv(t)
	chdirTemp(t)
	writeRequiredEnv(t)
	t.Setenv("LOGIN_MAX_FAILURES", "3")

	cfg, err := Bootstrap()
	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	var warnings []string
	err = cfg.Apply(map[string]string{
		"LOGIN_MAX_FAILURES": "9",
		"DATABASE_URL":       "postgres://evil",
		"NOT_A_KEY":          "x",
	}, func(format string, args ...any) {
		warnings = append(warnings, format)
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if cfg.LoginMaxFailures != 9 {
		t.Fatalf("failures = %d, want settings override", cfg.LoginMaxFailures)
	}
	if strings.Contains(cfg.DatabaseURL, "evil") {
		t.Fatalf("infra key was overridden: %s", cfg.DatabaseURL)
	}
	if len(warnings) != 2 {
		t.Fatalf("warnings = %v", warnings)
	}
}

func TestValidateAppValueRejectsInfraAndUnknown(t *testing.T) {
	if err := ValidateAppValue("DATABASE_URL", "postgres://x"); err == nil {
		t.Fatal("infra key accepted")
	}
	if err := ValidateAppValue("NOPE", "1"); err == nil {
		t.Fatal("unknown key accepted")
	}
	if err := ValidateAppValue("LOGIN_WINDOW", "10m"); err != nil {
		t.Fatalf("valid app key: %v", err)
	}
}
