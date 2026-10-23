# ADR-0030: An MCP server over the REST API, for personal use

**Status:** Accepted (2026-10-23)

## Context

The owner wants to ask an AI client (Claude Desktop, Claude Code) questions
about their own Commander data: "how did my bracket 3 decks do this month",
"who eliminates me most", "how is the Tuesday group's standings". The answers
are already in the API, spread over about 85 operations. The web dashboard
shows a fixed selection of them; the model can combine them freely.

Three facts shaped the decision:

1. **Only one user, the owner.** It is not a product feature, so there is no
   multi-tenant auth, consent flow or hosting to build.
2. **The API has no machine credentials.** Sessions are short-lived JWTs plus
   rotating refresh tokens, and a replayed refresh token revokes the whole
   session family (ADR-0001, ADR-0028). There are no API keys or personal
   access tokens. Failed logins are limited per account (ADR-0025).
3. **The 2-second rule** (ROADMAP). At the table, tapping is faster than
   talking to an agent. An MCP server belongs to the analysis that happens
   after playing, not to the life tracker.

## Decision

**A separate binary, `backend/cmd/mcp`, that is a client of the REST API.**
`internal/mcpserver` holds the client, the tools, the prompts and one resource,
built on the official Go SDK (`github.com/modelcontextprotocol/go-sdk`). It
imports no other slice and never touches the database: every call goes through
the same handlers, validation and authorization as the web and Android clients.
It lives in the backend module so it gets the existing gofmt, vet, golangci-lint
and test gates for free. The Dockerfile builds only `cmd/api`, so the MCP
server is never deployed.

**Transport: stdio. Auth: the owner's email and password**, read from
`COMMANDER_EMAIL`/`COMMANDER_PASSWORD`. The client logs in on first use, keeps
both tokens in memory only, and handles a 401 with one refresh (or a fresh
login if the refresh token is gone too) and one retry. Renewal runs under a
mutex, so concurrent tool calls never replay a refresh token. A login the API
rejects (401, 403, 429) is not retried until the process restarts, because each
failure counts against the per-account limit and would lock the owner out of
the apps too. On exit the server calls `POST /auth/logout`, so a run leaves no
live session behind.

**Phase 1 is read-only: 11 tools, 3 prompts, 1 resource.** The tools are
shaped around questions, not endpoints:

| Tool | Endpoints it combines |
|---|---|
| `get_overview` | `/statistics/dashboard` |
| `query_games` | `/statistics/games`, paged, plus `since` and `opponent` filters applied in the client |
| `get_breakdown` | `/statistics/breakdown` |
| `get_head_to_head` | `/statistics/opponents` |
| `list_decks` | `/decks` (every page) joined with `/statistics/decks` |
| `get_deck` | `/decks/{id}`, `/statistics/deck/{id}`, the deck's games found in `/statistics/games` |
| `list_playgroups` | `/playgroups` joined with `/statistics/playgroups` |
| `get_playgroup` | `/playgroups/{id}` joined with `/statistics/playgroup/{id}` |
| `get_game` | `/games/{id}`, `/games/{id}/timeline`, the playgroup roster for usernames |
| `list_tournaments` | `/tournaments` |
| `get_tournament` | `/tournaments/{id}`, or `/tournaments/lookup` for a join code |

Decks, playgroups and tournaments can be named instead of passing an ID. A name
that matches several returns the candidates with their IDs, so the model can
ask. Answers drop what a model can't use (image URLs, Moxfield IDs, override
flags) and add what it would otherwise compute badly (`win_rate_pct`, usernames
in place of seat IDs). Every tool is annotated `readOnlyHint` and has an output
schema.

The prompts (`weekly-recap`, `deck-review`, `rivalries`) only phrase a request.
The resource `commander://reference/brackets` defines brackets, color identity,
the per-seat snapshot and what counts as a win, so the model doesn't guess.

**The contract guard.** Every path the client calls is a `pathXxx` constant in
`internal/mcpserver/endpoints.go`. `check-architecture.sh` fails when one of
them is not in `openapi.yaml`, so renaming a route can't break the MCP server
silently.

### Alternatives considered

- **In-process, calling the `Service`s directly.** Faster, with no auth to
  solve, but it bypasses the handlers' validation and authorization and becomes
  a second contract next to `openapi.yaml`. Rejected.
- **One tool per endpoint, generated from `openapi.yaml`.** About 85 tools,
  most of them auth flows or writes, and multi-step questions left to the
  model. Tool definitions alone would fill the context. Rejected.
- **A client generated with `oapi-codegen`.** Suggested in the first analysis
  and dropped when building it: the contract is OpenAPI 3.1, which
  `oapi-codegen` supports only partially, and it would generate all 85
  operations to use 20. A thin hand-written client plus the path check above
  gives the same drift protection.
- **Personal access tokens, or remote HTTP with OAuth 2.1.** These are the
  right answers for a scoped, revocable credential or for more than one user.
  For one owner on their own machine they are a migration, a middleware and an
  authorization server too early. They are deferred, not ruled out.
- **TypeScript (the reference SDK).** It would add a second toolchain to the
  backend for no capability the Go SDK lacks.

## Consequences

- **The password sits in the MCP client's config file**
  (`claude_desktop_config.json` or the Claude Code equivalent), in plain text
  on the owner's machine. That is acceptable for personal use on a trusted
  machine, and it is the first thing to replace (with a read-scoped personal
  access token) if this ever runs anywhere else.
- **Google-only accounts can't use it.** They have no password, and
  `/auth/login` answers 401 for them.
- **Third-party data reaches the model provider.** Opponents' usernames, their
  commanders and the owner's record against them are sent to the LLM along with
  the owner's own data. That is fine for the owner's own use. It would need a
  privacy-policy line before becoming a feature for other users.
- **Bounded crawling.** Lists are paged at 100 items and at most 10 pages per
  call. `get_deck` finds a deck's recent games among the last 200 finished
  games, because the API has no per-deck game filter. A heavy user would need
  that filter server-side.
- **A cold API is slow.** Render's free tier sleeps when idle (ADR-0015), so the
  request timeout is 90 s. The first question after a quiet spell can take tens
  of seconds.
- **Writes are phase 2, and the API isn't ready for the most useful one.**
  Logging a finished game after the fact takes five calls today (create, join
  per seat, start, eliminations, finish) and can't be backdated. If it fails
  midway it leaves an `active` game behind. The right fix is a
  `POST /games/completed` endpoint in the contract, not orchestration in the
  MCP server. Moxfield imports and tournament results are simpler candidates.
