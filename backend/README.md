# Commander Companion — Backend

Go API (Fiber + PostgreSQL + sqlc + goose). Single REST contract for the
Android and web clients, see [`docs/api/openapi.yaml`](../docs/api/openapi.yaml)
(source of truth — if an endpoint's behavior changes, that file
gets updated in the same change).

## Stack

Go + Fiber, PostgreSQL, `sqlc` (typed data access), `goose` (migrations).

## Setup

```bash
cd backend
cp .env.example .env   # fill in DB_URL, JWT_SECRET, etc.
make migrate-up        # apply migrations against your Postgres
make run                # http://localhost:8080
```

Environment variables documented in [`.env.example`](.env.example).

## Commands

```bash
make run                 # run the API locally
make test                # go test -race ./... (some are integration tests, require Postgres)
make lint                # golangci-lint (make lint-docker if you don't have it installed)
make generate-sql        # regenerate repositories with sqlc after editing query.sql
make migrate-up          # apply goose migrations
make migrate-down        # revert the last migration
```

## Testing the full stack with Docker

To bring up the API together with Postgres (and optionally the web client), see
`docker-compose.yml` at the repo root:

```bash
cd ..
docker compose up --build
```

The first time, the migrations need to be applied by hand (they don't run
automatically inside the container): `make migrate-up` with `DB_URL`
pointing at `localhost:5432` (the port Compose's `db` service publishes).

## Deployment (Render)

The `api` binary itself applies the goose migrations on startup, before
bringing up the HTTP server (see `common.RunMigrations` in
[`cmd/api/main.go`](cmd/api/main.go)). It doesn't depend on a "Pre-Deploy Command" or
any other platform hook — it runs the same way on Render (including the
free tier, which doesn't offer that hook), in Docker Compose, or locally. The
[`Dockerfile`](Dockerfile) image includes the `migrations/` directory alongside the
binary so this works at runtime.

Scaling note: `RunMigrations` takes a Postgres session-level advisory lock
(goose's `lock.NewPostgresSessionLocker()`) before applying migrations, so
if the service ever runs with more than one replica, the instance that
loses the race blocks until the winner finishes instead of both running
`goose up` concurrently.

Minimum environment variables to configure on the Render service: `APP_ENV=production`
(makes the two variables below mandatory at startup instead of silently
falling back to the dev defaults — see `internal/config/config.go`), `DB_URL`
(Supabase connection string — use the **Session pooler**, not the
Transaction pooler, because this backend uses prepared statements via pgx),
`JWT_SECRET` (a new one, not the dev default), `GOOGLE_CLIENT_ID`,
`CORS_ALLOWED_ORIGINS`, and `WEB_APP_URL` (the frontend's domain on Vercel). See
[`.env.example`](.env.example) for the rest.

### Knowing which build is live

`GET /health` reports the git SHA the running binary was built from, on both
its 200 and its 503 branch, so a deploy can be told apart from the one before
it (see [ADR-0020](../docs/decisions/0020-build-provenance-in-health.md)):

```json
{ "status": "ok", "db": "ok", "commit": "b9516c59…", "started_at": "2026-09-06T19:26:48Z" }
```

The SHA comes from the linker (`GIT_COMMIT` build argument), else from the
binary's VCS stamp, else from `RENDER_GIT_COMMIT` — which Render sets by
itself, so nothing needs configuring for the current deployment. Building the
image anywhere else, pass it explicitly:

```bash
docker build --build-arg GIT_COMMIT="$(git rev-parse HEAD)" backend/
```

`commit` reads `unknown` when none of the three applies (a plain `go build`
outside a git checkout). Treat that as "cannot tell", never as "not deployed
yet".

## Notes

- Much of the code is still actively evolving — before assuming
  something is finished, check [`docs/roadmap/TASKS.md`](../docs/roadmap/TASKS.md).
