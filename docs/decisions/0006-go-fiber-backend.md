# ADR-0006: Backend in Go with Fiber as the HTTP framework

**Status:** Accepted and implemented — **inherited decision, context
reconstructed**. This decision predates the start of the project's ADR
history (0001-0004, made 2026-07-26); there's no contemporary record of why
it was chosen. This document is written retroactively (2026-07-27) based on
what's already built and confirmed in the code (`backend/go.mod`,
`backend/cmd/api/main.go`), not from a real discussion that was witnessed.

## Context

The ROADMAP (`docs/roadmap/ROADMAP.md`) sets "Go, Gin/Fiber, PostgreSQL,
sqlc, goose, Docker" as the Stage 1 stack from the outset, and "scalable
architecture" and "speed during the game" as product goals. A language and
HTTP framework had to be chosen for a stateless, versioned REST API
(`/api/v1`), serving both the native Android client and, later, a web
client (see ADR-0004).

## Decision

- **Language: Go** (`go 1.25.0` in `go.mod`).
- **HTTP framework: Fiber v2** (`github.com/gofiber/fiber/v2 v2.52.14`),
  built on `fasthttp` instead of the standard `net/http` router or Gin (the
  other option the ROADMAP itself left open with "Gin/Fiber").

## Alternatives considered

- **Gin**: the ROADMAP mentions it as an explicit alternative
  ("Gin/Fiber"). Both are mature frameworks with comparable performance in
  the Go ecosystem; Fiber was probably chosen for its Express-inspired API
  (`c.Params`, `c.BodyParser`, middlewares with the same signature as
  Node/Express), which reduces onboarding friction if the team comes from a
  JS/TS stack — the project itself ends up adding a Nuxt client (ADR-0004)
  and an HTML test tool (`tools/auth-test/`), consistent with JS
  familiarity on the team.
- **`net/http` + a standard router (`chi`, `gorilla/mux`)**: more
  "dependency-free" and closer to the standard library, but requires
  manually assembling things Fiber already includes (route grouping, CORS
  middleware already used via `github.com/gofiber/fiber/v2/middleware/cors`,
  body parsing). For a small team shipping fast, Fiber's ready-made surface
  pays for itself.
- **Another language (Node/TypeScript, Python)**: given that the main
  client is native Android (not a shared JS framework like React Native),
  there was no pressure to share domain code with the client; Go offers
  simple deployment binaries (a single executable + lightweight Docker),
  static typing, and good concurrency support — reasonable for an API that,
  in later phases (see `ROADMAP.md`, "in later phases"), grows into a Match
  Engine + Statistics Engine with WebSocket.

## Consequences

- Fiber is built on `fasthttp`, not on `net/http` — some standard Go
  ecosystem libraries that assume `http.Handler` aren't directly
  compatible and require an adapter or Fiber's native equivalent (in
  practice this hasn't been a problem: auth, CORS, and the routes of all
  modules use Fiber's native API).
- The choice is already deeply rooted in the code (`main.go` registers all
  modules as Fiber route groups, `fiber.NewError` is used as the standard
  mechanism to map domain errors to HTTP codes across all `service.go`
  files) — reverting to another HTTP framework today would mean touching
  the error-handling code of all eight `internal/` modules.
- Go as the language fixes the rest of the backend toolchain: `sqlc` to
  generate typed data-access code (see ADR-0008), `golangci-lint` for
  linting, and Go's own concurrency model for when the Stage 6 WebSocket is
  implemented.

## References

- `backend/go.mod`
- `backend/cmd/api/main.go`
- `docs/roadmap/ROADMAP.md`, "Stage 1: Backend" section
