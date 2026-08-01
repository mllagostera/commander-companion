// Package config centralizes reading the server configuration from
// environment variables. Before this package, cmd/api/main.go read
// loose os.Getenv calls in 4 different places (DB connection, port, auth, CORS)
// with no struct grouping them.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/usuario/commander-companion-backend/internal/auth"
	"github.com/usuario/commander-companion-backend/internal/email"
)

var (
	// ErrJWTSecretRequiredInProduction is returned by Load when APP_ENV=production and
	// JWT_SECRET isn't set: refuses to start with the public dev default, which is
	// visible in this repo's own git history.
	ErrJWTSecretRequiredInProduction = errors.New("JWT_SECRET must be set explicitly when APP_ENV=production")
	// ErrCORSOriginsRequiredInProduction is returned by Load when APP_ENV=production and
	// CORS_ALLOWED_ORIGINS isn't set: refuses to start wide open to any origin.
	ErrCORSOriginsRequiredInProduction = errors.New("CORS_ALLOWED_ORIGINS must be set explicitly when APP_ENV=production")
)

const (
	defaultAccessTokenTTL  = 15 * time.Minute
	defaultRefreshTokenTTL = 30 * 24 * time.Hour
	defaultPort            = "8080"
	defaultWebAppURL       = "http://localhost:3000"
	defaultAppEnv          = "development"

	// AppEnvProduction is the APP_ENV value that turns on the fail-fast checks in
	// loadAuthConfig/corsAllowedOrigins: with it set, starting up without an explicit
	// JWT_SECRET or CORS_ALLOWED_ORIGINS is refused instead of silently falling back to
	// the public dev defaults (see ADR... none yet, but see the "insecure defaults with
	// no environment gate" finding from the 2026-08-01 security audit, docs/roadmap/TASKS.md).
	AppEnvProduction = "production"
)

// Config groups all of the server configuration read from environment
// variables.
type Config struct {
	AppEnv             string
	DBURL              string
	Port               string
	CORSAllowedOrigins string
	// WebAppURL is the public URL of the web client (Nuxt), used to build links that
	// are sent in transactional mails (e.g. email verification).
	WebAppURL string
	// RequireEmailVerification controls whether registration requires confirming the
	// email before being able to log in (see ADR-0012). Default false: in the alpha
	// phase there's no point sending the mail or blocking login over this, so new
	// accounts are created already verified and RegisterUser neither generates the
	// token nor calls the mailer.
	RequireEmailVerification bool
	Auth                     auth.Config
	Email                    email.Config
}

// Load reads the full configuration from environment variables, with the
// same defaults cmd/api/main.go used before this package existed.
func Load() (Config, error) {
	env := appEnv()

	authCfg, err := loadAuthConfig(env)
	if err != nil {
		return Config{}, err
	}

	corsOrigins, err := corsAllowedOrigins(env)
	if err != nil {
		return Config{}, err
	}

	return Config{
		AppEnv:                   env,
		DBURL:                    dbURL(),
		Port:                     port(),
		CORSAllowedOrigins:       corsOrigins,
		WebAppURL:                webAppURL(),
		RequireEmailVerification: boolEnv("REQUIRE_EMAIL_VERIFICATION", false),
		Auth:                     authCfg,
		Email:                    loadEmailConfig(),
	}, nil
}

// appEnv reads APP_ENV, defaulting to "development". It's the only thing
// AppEnvProduction gates: nothing in this package behaves differently for
// other values, there just isn't a third state to distinguish yet.
func appEnv() string {
	if v := os.Getenv("APP_ENV"); v != "" {
		return v
	}
	return defaultAppEnv
}

func dbURL() string {
	if v := os.Getenv("DB_URL"); v != "" {
		return v
	}
	// Default local development credential; overridden with DB_URL in any other environment.
	return "postgres://postgres:postgres@localhost:5432/commander?sslmode=disable"
}

func port() string {
	if v := os.Getenv("PORT"); v != "" {
		return v
	}
	return defaultPort
}

// corsAllowedOrigins reads the origins allowed for CORS. By default, in
// development any origin is allowed (no cookies/credentials are used,
// only Bearer tokens); with APP_ENV=production it must be restricted
// explicitly instead of silently opening up to any origin.
func corsAllowedOrigins(env string) (string, error) {
	if v := os.Getenv("CORS_ALLOWED_ORIGINS"); v != "" {
		return v, nil
	}
	if env == AppEnvProduction {
		return "", ErrCORSOriginsRequiredInProduction
	}
	return "*", nil
}

func webAppURL() string {
	if v := os.Getenv("WEB_APP_URL"); v != "" {
		return v
	}
	return defaultWebAppURL
}

// loadEmailConfig reads the Resend configuration. If RESEND_API_KEY is empty (dev
// without a Resend account), email.NewResendClient uses a console mailer instead
// (see internal/email), so there's nothing to validate here.
func loadEmailConfig() email.Config {
	return email.Config{
		APIKey:                os.Getenv("RESEND_API_KEY"),
		FromAddress:           os.Getenv("EMAIL_FROM"),
		VerifyEmailTemplateID: os.Getenv("RESEND_VERIFY_EMAIL_TEMPLATE_ID"),
	}
}

func loadAuthConfig(env string) (auth.Config, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		if env == AppEnvProduction {
			return auth.Config{}, ErrJWTSecretRequiredInProduction
		}
		// Default local development secret; overridden with JWT_SECRET in any other environment.
		//nolint:gosec // dev-only default, not a real secret
		secret = "dev-insecure-jwt-secret-change-me"
	}

	accessTTL, err := parseDurationEnv("ACCESS_TOKEN_TTL", defaultAccessTokenTTL)
	if err != nil {
		return auth.Config{}, err
	}

	refreshTTL, err := parseDurationEnv("REFRESH_TOKEN_TTL", defaultRefreshTokenTTL)
	if err != nil {
		return auth.Config{}, err
	}

	return auth.Config{
		JWTSecret:       []byte(secret),
		AccessTokenTTL:  accessTTL,
		RefreshTokenTTL: refreshTTL,
		GoogleClientID:  os.Getenv("GOOGLE_CLIENT_ID"),
	}, nil
}

// boolEnv reads a boolean environment variable ("true"/"false", case-insensitive via
// strconv.ParseBool which also accepts "1"/"0"). Any value that fails to parse falls
// back to the default instead of failing startup: it's not worth blocking the server
// over a typo in a flag like this.
func boolEnv(name string, fallback bool) bool {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return v
}

func parseDurationEnv(name string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback, nil
	}

	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("parsing %s: %w", name, err)
	}
	return d, nil
}
