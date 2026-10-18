// Package config centralizes reading the server configuration from
// environment variables. Before this package, cmd/api/main.go read
// loose os.Getenv calls in 4 different places (DB connection, port, auth, CORS)
// with no struct grouping them.
package config

import (
	"errors"
	"fmt"
	"net/mail"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/usuario/commander-companion-backend/internal/auth"
	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/email"
	"github.com/usuario/commander-companion-backend/internal/inbound"
)

var (
	// ErrJWTSecretRequiredInProduction is returned by Load when APP_ENV=production and
	// JWT_SECRET isn't set: refuses to start with the public dev default, which is
	// visible in this repo's own git history.
	ErrJWTSecretRequiredInProduction = errors.New("JWT_SECRET must be set explicitly when APP_ENV=production")
	// ErrCORSOriginsRequiredInProduction is returned by Load when APP_ENV=production and
	// CORS_ALLOWED_ORIGINS isn't set: refuses to start wide open to any origin.
	ErrCORSOriginsRequiredInProduction = errors.New("CORS_ALLOWED_ORIGINS must be set explicitly when APP_ENV=production")
	// ErrSampleRateOutOfRange is returned by Load when SENTRY_TRACES_SAMPLE_RATE
	// isn't a fraction between 0 and 1.
	ErrSampleRateOutOfRange = errors.New("sample rate out of range")
	// ErrBFFSecretTooShort is returned by Load when BFF_SHARED_SECRET is set but
	// shorter than minBFFSecretLen: it is all that stops anyone from choosing
	// their own rate-limit bucket, so a guessable value is refused at startup.
	ErrBFFSecretTooShort = errors.New("BFF_SHARED_SECRET is too short")
	// ErrInboundConfigIncomplete is returned by Load when only part of the inbound
	// mail forwarding is configured (see loadInboundConfig): half a setup would
	// either refuse every webhook or have nowhere to forward to, silently.
	ErrInboundConfigIncomplete = errors.New("inbound mail forwarding is partially configured")
	// ErrInboundForwardLoop is returned by Load when INBOUND_FORWARD_TO is on the
	// same domain as the sender of the copies: that domain's mail goes to Resend,
	// so every copy would come back as a new received email, forever.
	ErrInboundForwardLoop = errors.New("INBOUND_FORWARD_TO must not be on the forwarding domain")
)

const (
	defaultAccessTokenTTL  = 15 * time.Minute
	defaultRefreshTokenTTL = 30 * 24 * time.Hour
	defaultPort            = "8080"
	defaultWebAppURL       = "http://localhost:3000"
	// defaultPasswordResetTemplateID is the base alias of the Resend password reset
	// Templates (see passwordResetTemplateID).
	defaultPasswordResetTemplateID = "password-reset"
	defaultAppEnv                  = "development"
	// minBFFSecretLen is the shortest BFF_SHARED_SECRET accepted: 32 characters,
	// e.g. `openssl rand -hex 16`.
	minBFFSecretLen = 32

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
	// GitCommit identifies the source revision this binary was built from, so
	// GET /health can say which build is answering (see ADR-0020). It is
	// UnknownCommit when nothing identifies it. Unlike everything else here it
	// is not operator configuration: it comes from the linker, the binary's VCS
	// stamp or the platform, and is read here only because that keeps every
	// environment variable this process reads in one package.
	GitCommit string
	// SentryDSN is the Sentry project DSN errors and panics are reported to.
	// Empty (the default) disables Sentry entirely: local development and CI
	// don't need a Sentry project to run.
	SentryDSN string
	// SentryTracesSampleRate is the fraction (0..1) of requests recorded as
	// Sentry performance traces. 0 (the default) records none, but errors are
	// still linked to the web client's trace: the trace ID arrives in the
	// sentry-trace/baggage headers either way. A request that already carries a
	// sampling decision from the web client follows that decision instead.
	SentryTracesSampleRate float64
	// SlackSignupWebhookURL is the Slack incoming webhook that gets a message for
	// every new account. Empty (the default) disables those notifications.
	SlackSignupWebhookURL string
	// ClientIP says which headers identify the caller's IP for the rate limits
	// (BFF_SHARED_SECRET, CLIENT_IP_HEADER; see ADR-0024).
	ClientIP common.ClientIPConfig
	Auth     auth.Config
	Email    email.Config
	// Inbound forwards the mail Resend receives on the domain to a personal inbox
	// (see internal/inbound). Off unless configured.
	Inbound inbound.Config
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

	tracesSampleRate, err := parseSampleRateEnv("SENTRY_TRACES_SAMPLE_RATE")
	if err != nil {
		return Config{}, err
	}

	clientIP, err := loadClientIPConfig()
	if err != nil {
		return Config{}, err
	}

	emailCfg := loadEmailConfig()
	inboundCfg, err := loadInboundConfig(emailCfg)
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
		GitCommit:                gitCommit(),
		SentryDSN:                os.Getenv("SENTRY_DSN"),
		SentryTracesSampleRate:   tracesSampleRate,
		SlackSignupWebhookURL:    os.Getenv("SLACK_SIGNUP_WEBHOOK_URL"),
		ClientIP:                 clientIP,
		Auth:                     authCfg,
		Email:                    emailCfg,
		Inbound:                  inboundCfg,
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

// loadClientIPConfig reads which request data may identify the caller's IP.
// Both settings are optional: without them every request is keyed by its TCP
// peer, which is correct when nothing sits in front of the API (docker compose)
// and merely coarse otherwise — never a reason to refuse to start.
func loadClientIPConfig() (common.ClientIPConfig, error) {
	secret := os.Getenv("BFF_SHARED_SECRET")
	if secret != "" && len(secret) < minBFFSecretLen {
		return common.ClientIPConfig{}, fmt.Errorf("%w: need at least %d characters, got %d",
			ErrBFFSecretTooShort, minBFFSecretLen, len(secret))
	}
	return common.ClientIPConfig{
		BFFSharedSecret: secret,
		TrustedHeader:   os.Getenv("CLIENT_IP_HEADER"),
	}, nil
}

// loadEmailConfig reads the Resend configuration. If RESEND_API_KEY is empty (dev
// without a Resend account), email.NewResendClient uses a console mailer instead
// (see internal/email), so there's nothing to validate here.
func loadEmailConfig() email.Config {
	return email.Config{
		APIKey:                  os.Getenv("RESEND_API_KEY"),
		FromAddress:             os.Getenv("EMAIL_FROM"),
		VerifyEmailTemplateID:   os.Getenv("RESEND_VERIFY_EMAIL_TEMPLATE_ID"),
		PasswordResetTemplateID: passwordResetTemplateID(),
	}
}

// loadInboundConfig reads the inbound mail forwarding settings. Both
// RESEND_WEBHOOK_SECRET and INBOUND_FORWARD_TO unset means the feature is off
// (local development, CI); setting only one of them is refused.
//
// RESEND_INBOUND_API_KEY is meant to be a separate full-access key, so the
// sending-only RESEND_API_KEY keeps its narrower scope; when unset it falls back
// to RESEND_API_KEY, which then has to be full access. INBOUND_FORWARD_FROM
// falls back to EMAIL_FROM.
func loadInboundConfig(emailCfg email.Config) (inbound.Config, error) {
	cfg := inbound.Config{
		WebhookSecret: os.Getenv("RESEND_WEBHOOK_SECRET"),
		APIKey:        os.Getenv("RESEND_INBOUND_API_KEY"),
		ForwardTo:     os.Getenv("INBOUND_FORWARD_TO"),
		ForwardFrom:   os.Getenv("INBOUND_FORWARD_FROM"),
	}
	if cfg.WebhookSecret == "" && cfg.ForwardTo == "" {
		return inbound.Config{}, nil
	}
	if cfg.APIKey == "" {
		cfg.APIKey = emailCfg.APIKey
	}
	if cfg.ForwardFrom == "" {
		cfg.ForwardFrom = emailCfg.FromAddress
	}
	if err := validateInboundConfig(cfg); err != nil {
		return inbound.Config{}, err
	}
	return cfg, nil
}

// validateInboundConfig checks an enabled inbound configuration: every value
// present, both addresses parseable, and the copies not sent to the domain they
// are forwarded from.
func validateInboundConfig(cfg inbound.Config) error {
	var missing []string
	for name, value := range map[string]string{
		"RESEND_WEBHOOK_SECRET":                      cfg.WebhookSecret,
		"INBOUND_FORWARD_TO":                         cfg.ForwardTo,
		"RESEND_INBOUND_API_KEY (or RESEND_API_KEY)": cfg.APIKey,
		"INBOUND_FORWARD_FROM (or EMAIL_FROM)":       cfg.ForwardFrom,
	} {
		if value == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("%w: missing %s", ErrInboundConfigIncomplete, strings.Join(missing, ", "))
	}

	to, err := mail.ParseAddress(cfg.ForwardTo)
	if err != nil {
		return fmt.Errorf("%w: INBOUND_FORWARD_TO is not an email address: %w",
			ErrInboundConfigIncomplete, err)
	}
	from, err := mail.ParseAddress(cfg.ForwardFrom)
	if err != nil {
		return fmt.Errorf("%w: INBOUND_FORWARD_FROM is not an email address: %w",
			ErrInboundConfigIncomplete, err)
	}
	if strings.EqualFold(addressDomain(to.Address), addressDomain(from.Address)) {
		return ErrInboundForwardLoop
	}
	return nil
}

func addressDomain(address string) string {
	_, domain, _ := strings.Cut(address, "@")
	return domain
}

// passwordResetTemplateID defaults to the alias the reference templates are uploaded
// under (docs/decisions/0022-password-reset-template.*.html), unlike the verification
// one: the reset flow is always on, so it shouldn't depend on an extra env var to work.
func passwordResetTemplateID() string {
	if v := os.Getenv("RESEND_PASSWORD_RESET_TEMPLATE_ID"); v != "" {
		return v
	}
	return defaultPasswordResetTemplateID
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

// parseSampleRateEnv reads a sampling fraction between 0 and 1, defaulting to
// 0. Unlike boolEnv, an unparsable or out-of-range value fails startup: a typo
// here would otherwise silently turn tracing off (or on at full volume).
func parseSampleRateEnv(name string) (float64, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return 0, nil
	}

	rate, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing %s: %w", name, err)
	}
	if rate < 0 || rate > 1 {
		return 0, fmt.Errorf("%s must be between 0 and 1, got %v: %w", name, rate, ErrSampleRateOutOfRange)
	}
	return rate, nil
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
