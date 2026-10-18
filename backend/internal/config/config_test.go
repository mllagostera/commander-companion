package config_test

import (
	"errors"
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

func TestLoad_SentryDSN_ReadFromEnvironment(t *testing.T) {
	clearProductionSecrets(t)
	const dsn = "https://public@o0.ingest.sentry.io/0"
	t.Setenv("SENTRY_DSN", dsn)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.SentryDSN != dsn {
		t.Fatalf("SentryDSN = %q, want %q", cfg.SentryDSN, dsn)
	}
}

func TestLoad_SentryDSN_EmptyByDefault(t *testing.T) {
	clearProductionSecrets(t)
	t.Setenv("SENTRY_DSN", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.SentryDSN != "" {
		t.Fatalf("SentryDSN = %q, want empty (Sentry disabled)", cfg.SentryDSN)
	}
}

func TestLoad_SentryTracesSampleRate_DefaultsToZero(t *testing.T) {
	clearProductionSecrets(t)
	t.Setenv("SENTRY_TRACES_SAMPLE_RATE", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.SentryTracesSampleRate != 0 {
		t.Fatalf("SentryTracesSampleRate = %v, want 0", cfg.SentryTracesSampleRate)
	}
}

func TestLoad_SentryTracesSampleRate_ReadFromEnvironment(t *testing.T) {
	clearProductionSecrets(t)
	t.Setenv("SENTRY_TRACES_SAMPLE_RATE", "0.25")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.SentryTracesSampleRate != 0.25 {
		t.Fatalf("SentryTracesSampleRate = %v, want 0.25", cfg.SentryTracesSampleRate)
	}
}

func TestLoad_SentryTracesSampleRate_InvalidValues_ReturnError(t *testing.T) {
	for _, raw := range []string{"abc", "-0.1", "1.5"} {
		t.Run(raw, func(t *testing.T) {
			clearProductionSecrets(t)
			t.Setenv("SENTRY_TRACES_SAMPLE_RATE", raw)

			if _, err := config.Load(); err == nil {
				t.Fatalf("Load() error = nil, want an error for SENTRY_TRACES_SAMPLE_RATE=%q", raw)
			}
		})
	}
}

func TestLoad_ClientIP_DisabledByDefault(t *testing.T) {
	clearProductionSecrets(t)
	t.Setenv("BFF_SHARED_SECRET", "")
	t.Setenv("CLIENT_IP_HEADER", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.ClientIP.BFFSharedSecret != "" || cfg.ClientIP.TrustedHeader != "" {
		t.Fatalf("ClientIP = %+v, want the zero value (trust only the TCP peer)", cfg.ClientIP)
	}
}

func TestLoad_ClientIP_ReadFromEnvironment(t *testing.T) {
	clearProductionSecrets(t)
	const secret = "0123456789abcdef0123456789abcdef"
	t.Setenv("BFF_SHARED_SECRET", secret)
	t.Setenv("CLIENT_IP_HEADER", "True-Client-IP")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.ClientIP.BFFSharedSecret != secret || cfg.ClientIP.TrustedHeader != "True-Client-IP" {
		t.Fatalf("ClientIP = %+v, want both settings read from the environment", cfg.ClientIP)
	}
}

func TestLoad_ClientIP_ShortBFFSecret_ReturnsError(t *testing.T) {
	clearProductionSecrets(t)
	t.Setenv("BFF_SHARED_SECRET", "too-short")

	if _, err := config.Load(); !errors.Is(err, config.ErrBFFSecretTooShort) {
		t.Fatalf("Load() error = %v, want ErrBFFSecretTooShort", err)
	}
}

// Environment variables and sample values for the inbound forwarding tests.
const (
	envWebhookSecret  = "RESEND_WEBHOOK_SECRET"  //nolint:gosec // an env var name, not a credential
	envInboundAPIKey  = "RESEND_INBOUND_API_KEY" //nolint:gosec // an env var name, not a credential
	envForwardTo      = "INBOUND_FORWARD_TO"
	envForwardFrom    = "INBOUND_FORWARD_FROM"
	envResendAPIKey   = "RESEND_API_KEY" //nolint:gosec // an env var name, not a credential
	envEmailFrom      = "EMAIL_FROM"
	sampleWebhookSig  = "whsec_dGVzdA=="
	samplePersonal    = "me@gmail.com"
	sampleSiteSender  = "TapeandoCartones <no-reply@tapeandocartones.es>"
	samplePrivacyAddr = "privacidad@tapeandocartones.es"
	sampleAPIKey      = "re_x"
)

// clearInboundEnv unsets every variable loadInboundConfig reads, plus the
// email ones it falls back to.
func clearInboundEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		envWebhookSecret, envInboundAPIKey, envForwardTo, envForwardFrom, envResendAPIKey, envEmailFrom,
	} {
		t.Setenv(key, "")
	}
}

func TestLoad_Inbound_DisabledByDefault(t *testing.T) {
	clearProductionSecrets(t)
	clearInboundEnv(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.Inbound.Enabled() {
		t.Fatalf("Inbound.Enabled() = true, want false without configuration")
	}
}

func TestLoad_Inbound_FallsBackToEmailSettings(t *testing.T) {
	clearProductionSecrets(t)
	clearInboundEnv(t)
	t.Setenv(envWebhookSecret, sampleWebhookSig)
	t.Setenv(envForwardTo, samplePersonal)
	t.Setenv(envResendAPIKey, "re_full")
	t.Setenv(envEmailFrom, sampleSiteSender)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if !cfg.Inbound.Enabled() {
		t.Fatal("Inbound.Enabled() = false, want true")
	}
	if cfg.Inbound.APIKey != "re_full" {
		t.Errorf("Inbound.APIKey = %q, want the RESEND_API_KEY fallback", cfg.Inbound.APIKey)
	}
	if cfg.Inbound.ForwardFrom != sampleSiteSender {
		t.Errorf("Inbound.ForwardFrom = %q, want the EMAIL_FROM fallback", cfg.Inbound.ForwardFrom)
	}
}

func TestLoad_Inbound_PrefersDedicatedKeyAndSender(t *testing.T) {
	clearProductionSecrets(t)
	clearInboundEnv(t)
	t.Setenv(envWebhookSecret, sampleWebhookSig)
	t.Setenv(envForwardTo, samplePersonal)
	t.Setenv(envResendAPIKey, "re_sending")
	t.Setenv(envInboundAPIKey, "re_full")
	t.Setenv(envEmailFrom, sampleSiteSender)
	t.Setenv(envForwardFrom, samplePrivacyAddr)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.Inbound.APIKey != "re_full" || cfg.Inbound.ForwardFrom != samplePrivacyAddr {
		t.Errorf("Inbound = %+v, want the dedicated key and sender", cfg.Inbound)
	}
}

func TestLoad_Inbound_PartialConfig_ReturnsError(t *testing.T) {
	tests := []struct {
		name, webhookSecret, forwardTo, apiKey string
	}{
		{name: "secret without destination", webhookSecret: sampleWebhookSig, apiKey: sampleAPIKey},
		{name: "destination without secret", forwardTo: samplePersonal, apiKey: sampleAPIKey},
		{name: "no API key at all", webhookSecret: sampleWebhookSig, forwardTo: samplePersonal},
		{
			name: "destination not an address", webhookSecret: sampleWebhookSig,
			forwardTo: "not an email", apiKey: sampleAPIKey,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearProductionSecrets(t)
			clearInboundEnv(t)
			t.Setenv(envWebhookSecret, tt.webhookSecret)
			t.Setenv(envForwardTo, tt.forwardTo)
			t.Setenv(envResendAPIKey, tt.apiKey)
			t.Setenv(envEmailFrom, sampleSiteSender)

			_, err := config.Load()
			if !errors.Is(err, config.ErrInboundConfigIncomplete) {
				t.Fatalf("Load() error = %v, want ErrInboundConfigIncomplete", err)
			}
		})
	}
}

func TestLoad_Inbound_ForwardToSameDomain_ReturnsError(t *testing.T) {
	clearProductionSecrets(t)
	clearInboundEnv(t)
	t.Setenv(envWebhookSecret, sampleWebhookSig)
	t.Setenv(envForwardTo, "otro@TapeandoCartones.es")
	t.Setenv(envResendAPIKey, "re_full")
	t.Setenv(envEmailFrom, sampleSiteSender)

	_, err := config.Load()
	if !errors.Is(err, config.ErrInboundForwardLoop) {
		t.Fatalf("Load() error = %v, want ErrInboundForwardLoop", err)
	}
}
