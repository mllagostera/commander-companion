# ADR-0015: Deployment infrastructure — Render + Vercel + Supabase

**Status:** Accepted (2026-07-30)

## Context

The project ran exclusively via `docker-compose.yml` locally; there was no
actual deployed environment. [ROADMAP.md](../roadmap/ROADMAP.md#deployment-infrastructure)
proposed, as Option 1 (recommended for a fast, zero-cost MVP), a modern/serverless
PaaS: frontend on Vercel or Cloudflare Pages, backend on Fly.io or
Render, database on Neon or Supabase — but it remained pending a decision,
with no ADR.

The automatic migrations PR (`internal/common/migrate.go`, see
`docs/roadmap/TASKS.md`, Stage 1 / Infra) was already built specifically with
Render's free tier in mind (which does not offer a
pre-deploy/release command hook), and `backend/README.md` already documents the env vars
needed for Render + Supabase + Vercel. This ADR formalizes that decision,
which had already been made de facto in the code but not recorded.

## Decision

Within ROADMAP Option 1, the following is chosen:

- **Backend:** [Render](https://render.com), as a Docker web service (using the
  existing `backend/Dockerfile`, which already includes `migrations/` so the
  binary can apply them on startup). The free tier does not offer a separate "Pre-Deploy
  Command"; that's why migrations run embedded in the binary itself
  (`common.RunMigrations`, `cmd/api/main.go`) instead of relying on
  that hook.
- **Frontend:** [Vercel](https://vercel.com), for the Nuxt client (`web/`).
  Nuxt/Nitro auto-detects the Vercel preset at build time without any
  additional explicit configuration in `nuxt.config.ts` — no `vercel.json` is
  needed for the standard case.
- **Database:** [Supabase](https://supabase.com) (managed
  PostgreSQL). Use the **Session pooler**, not the Transaction pooler: the
  backend uses `pgx` with prepared statements, which the Transaction pooler
  doesn't support correctly.

Minimum environment variables to configure in Render (see
[`backend/README.md`](../../backend/README.md#despliegue-render) for the
full detail): `DB_URL` (Supabase Session pooler connection string), `JWT_SECRET` (a new
one, not the dev default), `GOOGLE_CLIENT_ID`,
`CORS_ALLOWED_ORIGINS`, `WEB_APP_URL` (the frontend's domain on Vercel).

### Known limitation

If the backend were ever to run with more than one replica on Render, all
instances would run `goose up` in parallel at boot. Goose is idempotent
(`goose_db_version`) but does not serialize those concurrent runs with a
lock. No effect as long as the deployment is a single instance (the
current case).

## Out of scope for this ADR

- `render.yaml` is not version-controlled, nor is explicit Vercel configuration
  versioned as IaC yet — both platforms are configured manually via their
  dashboards. If it's decided to version that configuration later, that's a
  separate change.
- There is no CI workflow that deploys automatically (the 4 workflows in
  `.github/workflows/` are for quality — lint/test/build —, none of them
  deploys). Render/Vercel deploy through their own GitHub integration
  (push to `main`), outside Actions' control.

## References

- [`backend/README.md`](../../backend/README.md#despliegue-render) — env vars
  and startup details on Render.
- `backend/internal/common/migrate.go` — migrations embedded at startup.
- [ROADMAP.md](../roadmap/ROADMAP.md#deployment-infrastructure) — the original
  Option 1 that this ADR closes out.
