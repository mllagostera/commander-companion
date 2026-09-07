# ADR-0020: `GET /health` reports which build is answering

**Status:** Accepted (2026-09-06)

## Context

Render and Vercel each deploy off their own GitHub integration on a push to
`main`, in parallel and unordered (see
[ADR-0015](0015-deployment-infrastructure.md)). The new frontend can therefore
serve against the old backend for as long as Render takes to build — the only
open item on [TASKS.md](../roadmap/TASKS.md) that can produce a broken
production.

The obvious fix — have the frontend deploy wait for the backend to be live —
had no way to be implemented: `/health` answered

```json
{ "status": "ok", "db": "ok" }
```

which is byte-for-byte identical before and after a deploy. The old binary
answers it exactly like the new one, so "wait for the backend" could not be
resolved by polling. Nothing else in the process exposed a version either:
`openapi.yaml`'s `info.version` and Fiber's `AppName` are hand-maintained
strings that have read `0.1` since the repository was created, and the
repository carries no git tags.

This ADR covers only the marker. **Sequencing the two deploys is deliberately
left open** and stays on TASKS.md: this is the missing input it needs, not the
fix itself.

## Decision

`GET /health` gains two fields, on both the 200 and the 503 branch:

```json
{
  "status": "ok",
  "db": "ok",
  "commit": "b9516c596e3399402aa5e4756e0157eb797d4cbd",
  "started_at": "2026-09-06T19:26:48Z"
}
```

- **`commit`** — the full git SHA the running binary was built from, or
  `unknown`. This is the field a poller compares against the SHA it just
  pushed.
- **`started_at`** — RFC 3339, UTC, captured at the top of `run()` before the
  migrations (which take seconds). It answers "did the process restart?" when
  `commit` is `unknown`, and it distinguishes a redeploy that carried no code
  change.

They are served on the 503 branch too. Which build is live does not depend on
Postgres being reachable, and a caller waiting on a deploy still needs the
answer when the database is down — a 503 that omitted them would be read as
"still the old build".

**Where the commit comes from**, in descending order of authority
(`internal/config/buildinfo.go`):

1. **The linker.** `-ldflags "-X …/internal/config.buildCommit=$SHA"`, fed by
   the new `GIT_COMMIT` build argument in `backend/Dockerfile`. Describes this
   exact binary and works on any platform.
2. **The binary's VCS stamp.** `debug.ReadBuildInfo()`'s `vcs.revision`, which
   the Go toolchain records for a `go build` inside a git checkout. Free, and
   it makes `make build` / `make run` report a real SHA with no extra step.
3. **`RENDER_GIT_COMMIT`.** Render sets it on every service deployed from a Git
   repo. It describes the *deploy* rather than the binary, which is why it
   comes last — but it is what makes the marker work on the current deployment
   with no change to how Render builds: Render builds `backend/Dockerfile` with
   `backend/` as its context, so the repository's `.git` (one directory up) is
   not there and step 2 cannot fire.
4. Otherwise `unknown`.

The value is read in `internal/config` rather than in `internal/common`, so
every environment variable this process reads still comes from one package,
and the health handler stays a pure function of what it is handed
(`common.BuildInfo`). It is sanitized before being served: 7–64 lowercase hex
characters, anything else becomes `unknown`. `/health` is public and the value
can come from an environment variable, so the response body stays bounded and
predictable instead of echoing whatever was set.

`/health` is now in [`openapi.yaml`](../api/openapi.yaml) — it never was,
despite being a real client-facing endpoint. It carries a path-level `servers`
override because it lives at the root, not under `/api/v1`. The route check in
`.github/scripts/check-architecture.sh` was extended to see routes registered
on the Fiber app itself (`app.Get`) and not only on a slice's `router`; that
blind spot is why the omission lasted this long.

### Alternatives considered

- **A separate `/version` endpoint.** One more public unauthenticated route,
  and a poller would then have to hit two endpoints to learn "the new build is
  up *and* it can reach its database". Rejected.
- **A `version` field alongside `commit`.** With no git tags and no release
  process, it could only have carried the hand-maintained `0.1` — a third copy
  of a string that is already wrong in two places and that cannot distinguish
  two deploys. Rejected; if tagging ever starts, adding it is additive.
- **Only `RENDER_GIT_COMMIT`.** Simplest, but it makes the marker a property of
  one hosting provider: `docker compose up` and a local `make run` would both
  report `unknown`, and so would any future CI-built image. The three-step
  resolution costs ~40 lines and keeps the endpoint honest everywhere.

## Consequences

- The deploy race can now be sequenced: whatever drives it (a workflow, a
  Vercel `ignoreCommand`, a manual check) can poll `/health` until `commit`
  equals the SHA being deployed. **That work is not done here** — it remains
  open on TASKS.md, and it is now unblocked rather than solved.
- The commit SHA is exposed publicly and unauthenticated. It reveals that a
  deploy happened and its SHA; it grants nothing on a repository a reader
  cannot already clone, and it is what makes the endpoint useful to an external
  poller that holds no JWT. If the repository's SHAs ever become something
  worth hiding, the fix is to gate the two fields behind a header or a shared
  secret, not to remove them.
- `RegisterHealthRoute` takes a third argument. One call site (`cmd/api`) and
  the two existing tests.
- On Render, `commit` is only as truthful as `RENDER_GIT_COMMIT`: it describes
  what Render *deployed*, so a container kept alive across a failed deploy
  could in principle report a SHA whose binary never started. Passing
  `GIT_COMMIT` as a Docker build argument removes that gap entirely and is the
  documented upgrade path — Render exposing build arguments is the only reason
  it is not the default here.
- `unknown` is a legitimate answer (a plain `go build` outside a checkout). Any
  poller must treat it as "cannot tell", never as "not deployed yet", or it
  will wait forever.
