package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/admin"
	"github.com/usuario/commander-companion-backend/internal/auth"
	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/config"
	"github.com/usuario/commander-companion-backend/internal/deckresync"
	"github.com/usuario/commander-companion-backend/internal/decks"
	"github.com/usuario/commander-companion-backend/internal/email"
	"github.com/usuario/commander-companion-backend/internal/friends"
	gameactions "github.com/usuario/commander-companion-backend/internal/game-actions"
	"github.com/usuario/commander-companion-backend/internal/games"
	"github.com/usuario/commander-companion-backend/internal/inbound"
	"github.com/usuario/commander-companion-backend/internal/moxfield"
	"github.com/usuario/commander-companion-backend/internal/moxfieldimport"
	"github.com/usuario/commander-companion-backend/internal/notify"
	"github.com/usuario/commander-companion-backend/internal/playgroups"
	"github.com/usuario/commander-companion-backend/internal/statistics"
	"github.com/usuario/commander-companion-backend/internal/sync"
	"github.com/usuario/commander-companion-backend/internal/tournaments"
	"github.com/usuario/commander-companion-backend/internal/users"
	"github.com/usuario/commander-companion-backend/internal/websocket"
)

const (
	migrationsDir = "migrations"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// Captured before the migrations below, which can take seconds: /health
	// reports when the process started, not when it finished booting.
	startedAt := time.Now().UTC()

	// 1. Load configuration and connect to DB
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log.Printf("Client IP for rate limits: %s.", cfg.ClientIP)

	sentryEnabled, err := initSentry(&cfg)
	if err != nil {
		return err
	}
	if sentryEnabled {
		defer sentry.Flush(sentryFlushTimeout)
	}

	// Migrations before opening the app's pool (see common.RunMigrations):
	// brings the schema up to date in any environment, including those that
	// don't offer a separate "release/pre-deploy command" hook.
	if migrateErr := common.RunMigrations(cfg.DBURL, migrationsDir); migrateErr != nil {
		return migrateErr
	}
	log.Println("Migraciones aplicadas correctamente.")

	db, err := common.NewDB(cfg.DBURL)
	if err != nil {
		return fmt.Errorf("no se pudo conectar a la base de datos: %w", err)
	}
	defer db.Close()
	log.Println("Conectado a PostgreSQL exitosamente.")

	// Reap moxfield_import_jobs/deck_resync_jobs left pending/in_progress by a
	// previous process run (crash, redeploy, Render free tier sleeping the
	// service) — see moxfieldimport.ReapStaleJobs and deckresync.ReapStaleJobs.
	// Safe to run unconditionally here: in this single-instance deployment, a
	// freshly starting process can't have any of these jobs actually in flight yet.
	if reapErr := reapStaleBackgroundJobs(context.Background(), db.Pool); reapErr != nil {
		return reapErr
	}

	// 2. Initialize Fiber
	app := fiber.New(serverConfig(defaultServerTimeouts()))

	// Middlewares
	app.Use(newRequestLogger(os.Stdout))
	app.Use(recover.New())
	// Before Sentry: it strips the BFF secret header, which must never end up
	// in a Sentry event's request headers.
	app.Use(common.ResolveClientIP(cfg.ClientIP))
	if sentryEnabled {
		useSentry(app)
	}
	app.Use(cors.New(cors.Config{
		AllowOrigins: cfg.CORSAllowedOrigins,
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET,POST,PUT,PATCH,DELETE,OPTIONS",
	}))
	// The API host is public (api.<domain>), so keep it out of search indexes.
	app.Use(common.NoIndex)

	common.RegisterHealthRoute(app, db, common.BuildInfo{
		Commit:    cfg.GitCommit,
		StartedAt: startedAt,
	})
	common.RegisterRobotsRoute(app)
	registerModules(app, db, &cfg)

	// 4. Start Server
	log.Printf("Iniciando servidor en el puerto %s...", cfg.Port)
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", ":"+cfg.Port)
	if err != nil {
		return fmt.Errorf("error al abrir el listener: %w", err)
	}
	if err := app.Listener(&noDelayListener{ln}); err != nil {
		return fmt.Errorf("error al arrancar el servidor: %w", err)
	}
	return nil
}

// reapStaleBackgroundJobs clears out moxfield_import_jobs/deck_resync_jobs rows left
// pending/in_progress by a previous process run, see moxfieldimport.ReapStaleJobs
// and deckresync.ReapStaleJobs for why that's both necessary and safe to do
// unconditionally at startup.
func reapStaleBackgroundJobs(ctx context.Context, pool *pgxpool.Pool) error {
	reapedImports, err := moxfieldimport.ReapStaleJobs(ctx, pool)
	if err != nil {
		return err
	}
	if reapedImports > 0 {
		log.Printf("moxfieldimport: %d job(s) atascado(s) de una corrida anterior marcados como failed.", reapedImports)
	}

	reapedResyncs, err := deckresync.ReapStaleJobs(ctx, pool)
	if err != nil {
		return err
	}
	if reapedResyncs > 0 {
		log.Printf("deckresync: %d job(s) atascado(s) de una corrida anterior marcados como failed.", reapedResyncs)
	}

	return nil
}

// serverTimeouts bound how long a connection may hold the server without making
// progress. Without them fasthttp waits forever, so a client that opens a
// connection and trickles its headers (slowloris) keeps a goroutine and a
// socket busy indefinitely.
type serverTimeouts struct {
	// Read bounds reading one whole request, headers and body. Request bodies
	// here are small JSON documents.
	Read time.Duration
	// Write bounds writing the response. fasthttp starts it after the handler
	// returns, so a slow handler (a Moxfield import) is not affected.
	Write time.Duration
	// Idle bounds how long a keep-alive connection may wait for its next
	// request. Kept above the 60 s idle timeout common in load balancers, so
	// the proxy in front closes an idle connection before the server does and
	// never reuses one the server already dropped. Render doesn't document its
	// own value; 60 s is an assumption.
	Idle time.Duration
}

const (
	serverReadTimeout  = 10 * time.Second
	serverWriteTimeout = 10 * time.Second
	serverIdleTimeout  = 75 * time.Second
)

func defaultServerTimeouts() serverTimeouts {
	return serverTimeouts{Read: serverReadTimeout, Write: serverWriteTimeout, Idle: serverIdleTimeout}
}

// serverConfig is the Fiber configuration of the API. WebSocket connections are
// not cut by these timeouts: fasthttp clears the connection's deadlines before
// handing it to the upgrade handler, and the websocket package manages its own
// (auth timeout, pong wait) from there.
func serverConfig(timeouts serverTimeouts) fiber.Config {
	return fiber.Config{
		ErrorHandler: common.ErrorHandler,
		AppName:      "TapeandoCartones API v0.1",
		ReadTimeout:  timeouts.Read,
		WriteTimeout: timeouts.Write,
		IdleTimeout:  timeouts.Idle,
	}
}

// newRequestLogger logs every request except the health check, which Render
// probes every few seconds (the interval isn't configurable): left in, it
// would be most of the log. A failing health check still shows up, as a
// Render alert and a restart.
func newRequestLogger(out io.Writer) fiber.Handler {
	return logger.New(logger.Config{
		Output: out,
		Next: func(c *fiber.Ctx) bool {
			return c.Path() == common.HealthPath
		},
	})
}

// registerModules instantiates repositories, services, and handlers, and
// registers the routes of every module under /api/v1 (public and JWT-protected).
func registerModules(app *fiber.App, db *common.DB, cfg *config.Config) {
	api := app.Group("/api/v1")

	// The rate limits are passed to each endpoint instead of being mounted as
	// group middleware: in Fiber, a Group with middleware applies to everything
	// that shares the prefix, and each limit bounds specific routes only.
	limits := newAPIRateLimits()

	// emailClient sends the account mails (verification, password reset). Without
	// RESEND_API_KEY (dev without a Resend account) it logs the links to the console
	// instead of sending them (see internal/email).
	emailClient := email.NewResendClient(cfg.Email)
	// signupNotifier posts a Slack message for every new account. Without
	// SLACK_SIGNUP_WEBHOOK_URL it does nothing (see internal/notify).
	signupNotifier := notify.NewSlackNotifier(cfg.SlackSignupWebhookURL)
	usersService := users.NewService(
		db.Pool, emailClient, signupNotifier, cfg.WebAppURL, cfg.RequireEmailVerification,
	)
	usersHandler := users.NewHandler(usersService)
	// POST /auth/register, verify-email, resend-verification, forgot-password, reset-password
	usersHandler.RegisterRoutes(api, users.PublicRateLimits{
		Credentials:  limits.credentials,
		AccountEmail: limits.accountEmail,
	})

	authService := auth.NewService(db.Pool, usersService, cfg.Auth)
	authHandler := auth.NewHandler(authService)
	// login, google, refresh, logout
	authHandler.RegisterPublicRoutes(api, auth.PublicRateLimits{
		Credentials: limits.credentials,
		FailedLogin: limits.failedLogin,
		Refresh:     limits.refresh,
	})

	// The activity tracker runs on every authenticated request (after RequireAuth,
	// which gives it the user ID) to keep users.last_seen_at fresh for the admin
	// overview's online_users — see users.ActivityTracker and ADR-0018's addendum.
	activityTracker := users.NewActivityTracker(usersService)
	protected := api.Group("", auth.RequireAuth(cfg.Auth.JWTSecret), activityTracker.Middleware())
	authHandler.RegisterProtectedRoutes(protected) // GET /auth/me
	// GET /users/search, PATCH /users/:id, POST /users/:id/password.
	usersHandler.RegisterProtectedRoutes(protected, limits.search)

	moxfieldClient := moxfield.NewClient()
	decksService := decks.NewService(db.Pool, moxfieldClient)
	decks.NewHandler(decksService).RegisterRoutes(protected, limits.moxfieldDeck)

	playgroupsService := playgroups.NewService(db.Pool)
	playgroups.NewHandler(playgroupsService).RegisterRoutes(protected)

	statisticsService := statistics.NewService(db.Pool, playgroupsService)
	statistics.NewHandler(statisticsService).RegisterRoutes(protected)

	// wsHub relays a game's game_actions live to every client connected to it
	// (see ADR-0005). It's injected into games/game-actions as a
	// Broadcaster, without those packages depending on internal/websocket (same
	// pattern as statisticsService as StatisticsRecalculator). The WebSocket
	// route registration is deferred until gamesService exists below (it's
	// injected back as a websocket.MembershipChecker) — it's still public (no
	// auth.RequireAuth): it authenticates via the initial message, not via header.
	wsHub := websocket.NewHub()

	gamesService := games.NewService(db.Pool, statisticsService, wsHub, playgroupsService)
	games.NewHandler(gamesService).RegisterRoutes(protected)
	websocket.RegisterRoutes(api, wsHub, cfg.Auth.JWTSecret, gamesService)

	gameActionsService := gameactions.NewService(db.Pool, wsHub)
	gameactions.NewHandler(gameActionsService).RegisterRoutes(protected)

	// sync doesn't talk to the DB or to Moxfield on its own: it delegates to
	// decks, which owns the table and the client (see internal/sync/service.go).
	syncService := sync.NewService(decksService)
	sync.NewHandler(syncService).RegisterRoutes(protected, limits.moxfieldDeck)

	// moxfieldimport: bulk background import of a Moxfield username's public
	// decks (see internal/moxfieldimport).
	moxfieldImportService := moxfieldimport.NewService(db.Pool, usersService, decksService, moxfieldClient)
	moxfieldimport.NewHandler(moxfieldImportService).RegisterRoutes(protected, limits.moxfieldBulk)

	// deckresync: resynchronizes in the background ALL decks already imported
	// with moxfield_id (unlike moxfieldimport, which brings in new decks by username).
	deckResyncService := deckresync.NewService(db.Pool, decksService)
	deckresync.NewHandler(deckResyncService).RegisterRoutes(protected, limits.moxfieldBulk)

	// tournaments: standalone Swiss-format Commander tournaments (see
	// internal/tournaments and ADR-0016), not tied to a playgroup.
	tournamentsService := tournaments.NewService(db.Pool, decksService)
	tournaments.NewHandler(tournamentsService).RegisterRoutes(protected)

	// friends: friend requests between app users (see internal/friends and
	// ADR-0017), distinct from playgroups (game groups, not friendship relations).
	friendsService := friends.NewService(db.Pool)
	friends.NewHandler(friendsService).RegisterRoutes(protected)

	// admin: user management, global stats overview and unfinished-game cleanup
	// for the admin dashboard (see internal/admin and ADR-0018). RequireAdmin is
	// chained after RequireAuth (protected already has it) and checks is_admin
	// fresh from the DB per request.
	adminGroup := protected.Group("/admin", auth.RequireAdmin(usersService))
	adminService := admin.NewService(db.Pool, wsHub)
	admin.NewHandler(adminService).RegisterRoutes(adminGroup)

	// inbound: forwards the mail Resend receives on the domain to a personal
	// inbox (see internal/inbound). Registered on api, not protected: Resend
	// authenticates with the webhook signature, not a JWT. Without its settings
	// the route doesn't exist at all.
	if cfg.Inbound.Enabled() {
		inboundForwarder := inbound.NewResendForwarder(cfg.Inbound)
		inbound.NewHandler(cfg.Inbound.WebhookSecret, inboundForwarder).RegisterRoutes(api)
	}
}
