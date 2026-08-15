package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/auth"
	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/config"
	"github.com/usuario/commander-companion-backend/internal/deckresync"
	"github.com/usuario/commander-companion-backend/internal/decks"
	"github.com/usuario/commander-companion-backend/internal/email"
	"github.com/usuario/commander-companion-backend/internal/friends"
	gameactions "github.com/usuario/commander-companion-backend/internal/game-actions"
	"github.com/usuario/commander-companion-backend/internal/games"
	"github.com/usuario/commander-companion-backend/internal/moxfield"
	"github.com/usuario/commander-companion-backend/internal/moxfieldimport"
	"github.com/usuario/commander-companion-backend/internal/playgroups"
	"github.com/usuario/commander-companion-backend/internal/statistics"
	"github.com/usuario/commander-companion-backend/internal/sync"
	"github.com/usuario/commander-companion-backend/internal/tournaments"
	"github.com/usuario/commander-companion-backend/internal/users"
	"github.com/usuario/commander-companion-backend/internal/websocket"
)

const (
	// Rate limit for the public auth endpoints. 20 req/min per IP leaves
	// plenty of slack for a human mistyping their password or for the
	// automatic refresh of several devices behind the same NAT, but cuts
	// credential stuffing short.
	authRateLimitMax    = 20
	authRateLimitWindow = time.Minute

	// Rate limit for GET /users/search. It's authenticated (unlike the auth
	// endpoints above), but it does an exact-email lookup — without a bound, an
	// account could be used to check an arbitrary list of addresses at whatever
	// rate the client wants (see the 2026-08-01 security audit, docs/roadmap/TASKS.md).
	searchRateLimitMax    = 20
	searchRateLimitWindow = time.Minute

	migrationsDir = "migrations"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// 1. Load configuration and connect to DB
	cfg, err := config.Load()
	if err != nil {
		return err
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
	app := fiber.New(fiber.Config{
		ErrorHandler: common.ErrorHandler,
		AppName:      "Commander Companion API v0.1",
	})

	// Middlewares
	app.Use(logger.New())
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: cfg.CORSAllowedOrigins,
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET,POST,PUT,PATCH,DELETE,OPTIONS",
	}))

	common.RegisterHealthRoute(app, db)
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

// newAuthRateLimiter builds the per-IP rate limiting middleware for the
// public auth endpoints. The counter lives in the process's memory: enough
// for a single instance (the current deployment, see docker-compose.yml);
// with several replicas it would need to move to a shared Storage (Redis).
//
// Note: the IP comes from fiber.Ctx.IP(), which behind a proxy/ingress
// returns the proxy's IP unless ProxyHeader is configured in fiber.Config.
// When deploying behind one, it must be set or the limit would be global.
func newAuthRateLimiter() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        authRateLimitMax,
		Expiration: authRateLimitWindow,
		LimitReached: func(_ *fiber.Ctx) error {
			// Returning the error (instead of writing the response here) makes it
			// go through common.ErrorHandler, so a 429 has the same body as
			// any other API error.
			return fiber.NewError(fiber.StatusTooManyRequests, "too many authentication requests, try again later")
		},
	})
}

// newSearchRateLimiter builds the per-IP rate limiting middleware for
// GET /users/search (see the constant doc above). Same in-process-memory /
// ProxyHeader caveats as newAuthRateLimiter.
func newSearchRateLimiter() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        searchRateLimitMax,
		Expiration: searchRateLimitWindow,
		LimitReached: func(_ *fiber.Ctx) error {
			return fiber.NewError(fiber.StatusTooManyRequests, "too many search requests, try again later")
		},
	})
}

// registerModules instantiates repositories, services, and handlers, and
// registers the routes of every module under /api/v1 (public and JWT-protected).
func registerModules(app *fiber.App, db *common.DB, cfg *config.Config) {
	api := app.Group("/api/v1")

	// The rate limit is passed to each public auth endpoint instead of being
	// mounted as group middleware: in Fiber, a Group with middleware applies
	// to everything that shares the prefix (including /auth/me and the rest
	// of the API), and here we only want to bound the endpoints without JWT.
	authRateLimit := newAuthRateLimiter()

	// emailClient sends the account verification email. Without RESEND_API_KEY
	// (dev without a Resend account) it logs the link to the console instead of sending it (see internal/email).
	emailClient := email.NewResendClient(cfg.Email)
	usersService := users.NewService(db.Pool, emailClient, cfg.WebAppURL, cfg.RequireEmailVerification)
	usersHandler := users.NewHandler(usersService)
	usersHandler.RegisterRoutes(api, authRateLimit) // POST /auth/register, verify-email, resend-verification

	authService := auth.NewService(db.Pool, usersService, cfg.Auth)
	authHandler := auth.NewHandler(authService)
	authHandler.RegisterPublicRoutes(api, authRateLimit) // login, google, refresh, logout

	protected := api.Group("", auth.RequireAuth(cfg.Auth.JWTSecret))
	authHandler.RegisterProtectedRoutes(protected) // GET /auth/me
	// GET /users/search, PATCH /users/:id, POST /users/:id/password.
	usersHandler.RegisterProtectedRoutes(protected, newSearchRateLimiter())

	moxfieldClient := moxfield.NewClient()
	decksService := decks.NewService(db.Pool, moxfieldClient)
	decks.NewHandler(decksService).RegisterRoutes(protected)

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
	sync.NewHandler(syncService).RegisterRoutes(protected)

	// moxfieldimport: bulk background import of a Moxfield username's public
	// decks (see internal/moxfieldimport).
	moxfieldImportService := moxfieldimport.NewService(db.Pool, usersService, decksService, moxfieldClient)
	moxfieldimport.NewHandler(moxfieldImportService).RegisterRoutes(protected)

	// deckresync: resynchronizes in the background ALL decks already imported
	// with moxfield_id (unlike moxfieldimport, which brings in new decks by username).
	deckResyncService := deckresync.NewService(db.Pool, decksService)
	deckresync.NewHandler(deckResyncService).RegisterRoutes(protected)

	// tournaments: standalone Swiss-format Commander tournaments (see
	// internal/tournaments and ADR-0016), not tied to a playgroup.
	tournamentsService := tournaments.NewService(db.Pool, decksService)
	tournaments.NewHandler(tournamentsService).RegisterRoutes(protected)

	// friends: friend requests between app users (see internal/friends and
	// ADR-0017), distinct from playgroups (game groups, not friendship relations).
	friendsService := friends.NewService(db.Pool)
	friends.NewHandler(friendsService).RegisterRoutes(protected)
}
