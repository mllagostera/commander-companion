package config_test

import (
	"strings"
	"testing"

	"github.com/usuario/commander-companion-backend/internal/config"
)

// clearProductionSecrets ensures a clean slate: os.Getenv-based config reads
// leak between tests via the process environment if a previous test (or the
// developer's own shell) left one of these set.
func clearProductionSecrets(t *testing.T) {
	t.Helper()
	for _, key := range []string{"APP_ENV", "JWT_SECRET", "CORS_ALLOWED_ORIGINS"} {
		t.Setenv(key, "")
	}
}

func TestLoad_DevelopmentDefault_FallsBackToInsecureDefaults(t *testing.T) {
	clearProductionSecrets(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.AppEnv != "development" {
		t.Fatalf("AppEnv = %q, want %q", cfg.AppEnv, "development")
	}
	if cfg.CORSAllowedOrigins != "*" {
		t.Fatalf("CORSAllowedOrigins = %q, want %q", cfg.CORSAllowedOrigins, "*")
	}
	if len(cfg.Auth.JWTSecret) == 0 {
		t.Fatalf("Auth.JWTSecret is empty, want the dev fallback")
	}
}

func TestLoad_Production_WithoutJWTSecret_ReturnsError(t *testing.T) {
	clearProductionSecrets(t)
	t.Setenv("APP_ENV", config.AppEnvProduction)
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.commandercompanion.com")

	_, err := config.Load()
	if err == nil {
		t.Fatal("Load() error = nil, want an error for missing JWT_SECRET in production")
	}
	if !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("Load() error = %q, want it to mention JWT_SECRET", err)
	}
}

func TestLoad_Production_WithoutCORSAllowedOrigins_ReturnsError(t *testing.T) {
	clearProductionSecrets(t)
	t.Setenv("APP_ENV", config.AppEnvProduction)
	t.Setenv("JWT_SECRET", "a-real-production-secret")

	_, err := config.Load()
	if err == nil {
		t.Fatal("Load() error = nil, want an error for missing CORS_ALLOWED_ORIGINS in production")
	}
	if !strings.Contains(err.Error(), "CORS_ALLOWED_ORIGINS") {
		t.Fatalf("Load() error = %q, want it to mention CORS_ALLOWED_ORIGINS", err)
	}
}

func TestLoad_Production_WithBothSet_Succeeds(t *testing.T) {
	clearProductionSecrets(t)
	t.Setenv("APP_ENV", config.AppEnvProduction)
	t.Setenv("JWT_SECRET", "a-real-production-secret")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.commandercompanion.com")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if string(cfg.Auth.JWTSecret) != "a-real-production-secret" {
		t.Fatalf("Auth.JWTSecret = %q, want the explicitly configured secret", cfg.Auth.JWTSecret)
	}
	if cfg.CORSAllowedOrigins != "https://app.commandercompanion.com" {
		t.Fatalf("CORSAllowedOrigins = %q, want the explicitly configured origin", cfg.CORSAllowedOrigins)
	}
}
