package main

import (
	"fmt"
	"log"
	"time"

	"github.com/getsentry/sentry-go"
	sentryfiber "github.com/getsentry/sentry-go/fiber"
	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/config"
)

// sentryFlushTimeout bounds how long shutdown waits for buffered events to be
// sent, so a Sentry outage can't hang the process on its way out.
const sentryFlushTimeout = 2 * time.Second

// initSentry configures the global Sentry client from cfg. It reports whether
// Sentry is enabled: with no SENTRY_DSN it does nothing and returns false, so
// the caller skips mounting the Sentry middlewares.
//
// SendDefaultPII stays off (the SDK default): request bodies, cookies and the
// Authorization header are never sent, which matters because the auth
// endpoints carry passwords and tokens.
func initSentry(cfg *config.Config) (bool, error) {
	if cfg.SentryDSN == "" {
		return false, nil
	}

	// Tracing is enabled whenever Sentry is, even at a 0 sample rate: that way
	// requests coming from the web client, which carry their own sampling
	// decision in the sentry-trace header, still continue its trace, so
	// backend spans show up under the same trace as the browser's and Nitro's.
	opts := sentry.ClientOptions{
		Dsn:              cfg.SentryDSN,
		Environment:      cfg.AppEnv,
		EnableTracing:    true,
		TracesSampleRate: cfg.SentryTracesSampleRate,
	}
	if cfg.GitCommit != config.UnknownCommit {
		opts.Release = cfg.GitCommit
	}

	if err := sentry.Init(opts); err != nil {
		return false, fmt.Errorf("no se pudo inicializar Sentry: %w", err)
	}
	log.Println("Sentry habilitado.")
	return true, nil
}

// useSentry mounts the Sentry middlewares. It must run after recover.New():
// sentryfiber captures a panic with its stack trace and re-panics, so the
// outer recover middleware still turns it into a 500 through the usual
// ErrorHandler.
func useSentry(app *fiber.App) {
	app.Use(sentryfiber.New(sentryfiber.Options{Repanic: true}))
	app.Use(reportUnexpectedErrors)
}

// reportUnexpectedErrors sends to Sentry the errors a handler returns that
// ErrorHandler will turn into an opaque 500 (see common.IsUnexpected). Domain
// errors (4xx, 503) are expected behavior and aren't reported. Panics never
// reach this point as an error: sentryfiber reports them.
func reportUnexpectedErrors(c *fiber.Ctx) error {
	err := c.Next()
	if common.IsUnexpected(err) {
		if hub := sentryfiber.GetHubFromContext(c); hub != nil {
			hub.CaptureException(err)
		}
	}
	return err
}
