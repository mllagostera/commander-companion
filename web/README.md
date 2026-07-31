# Commander Companion — Web (Nuxt)

Web client, decoupled from the backend (it only consumes the REST API via HTTP, see
`docs/api/openapi.yaml`). See [ADR-0004](../docs/decisions/0004-web-client-nuxt.md)
for the context behind the decision.

Current status: registration and login (email/password + Google Sign-In), session with
automatic refresh, importing decks from Moxfield (with a thumbnail of the commander's
art crop, and a button to re-sync an already-imported deck against
Moxfield), and statistics screens (global user stats and per-deck stats).

## Stack

- Nuxt 4 (SSR), Tailwind CSS (`@nuxtjs/tailwindcss`), ESLint (`@nuxt/eslint`), npm.

## Setup

```bash
cd web
npm install
cp .env.example .env   # fill in NUXT_PUBLIC_API_BASE / NUXT_PUBLIC_GOOGLE_CLIENT_ID
npm run dev            # http://localhost:3000
```

Needs the backend running (see `backend/README.md` or the Docker
section below) and `CORS_ALLOWED_ORIGINS` in the backend must include this
client's origin (or be left empty in dev) — although with the web
client's normal flow CORS isn't needed anymore, see the Nitro section below.

For the Google button to work, in Google Cloud Console → **Credentials**
→ the Web Application OAuth Client → **Authorized JavaScript origins**, add
the origin this client runs on (e.g. `http://localhost:3000`).

Scripts: `npm run dev`, `npm run build`, `npm run lint`, `npm run typecheck`.

## Session: why there's a Nitro layer in the middle

The browser **never** talks to the Go API directly nor sees the tokens.
All calls go through Nitro's own endpoints:

- `/api/auth/{register,login,google,logout,session}` — the only ones that touch
  session cookies.
- `/api/backend/**` — authenticated proxy to the Go API (`/api/backend/decks`
  → `GET {API}/decks`). The `auth/*` paths are blocked in the proxy: they go
  through `/api/auth/*` so no path from JS ever returns a token.

Cookies managed by Nitro:

| Cookie             | `httpOnly` | Content                          |
| ------------------ | ---------- | ---------------------------------- |
| `cc_access_token`  | yes        | access JWT                      |
| `cc_refresh_token` | yes        | refresh token                      |
| `cc_session`       | no         | just the `"1"` marker, no sensitive value |

`cc_session` exists because the route middleware has to decide whether
there's a session both in SSR and on the client, and `httpOnly` cookies
can't be read from JS. It's not a credential: forging it only gets the
page to render and the first API request to return 401.

Side effect: since the browser no longer calls the Go API, **CORS isn't
needed** for the web client's normal flow.

### Automatic refresh

`server/utils/backend.ts` (`backendFetch`) adds the `Authorization: Bearer`
header from the cookie and, if the API responds with 401, exchanges the refresh token against
`POST /auth/refresh`, updates the cookies, and retries **once**. Same
spirit as the Android client's OkHttp `AuthAuthenticator`.

Two details that matter because the backend **rotates** the refresh token (revokes
the previous one on every refresh):

- Concurrent refreshes are deduplicated in memory (`inFlightRefresh`), so
  two requests that both get a 401 at the same time don't step on each other by revoking the token.
- In SSR each internal call runs in its own `H3Event`, so
  `app/composables/useNitroFetch.ts` keeps a cookie jar per request: it copies
  the `Set-Cookie` headers to the response the browser actually sees **and** applies
  them to subsequent calls in the same render (otherwise the second call would go out with an
  already-revoked refresh token).

## Testing against the backend with Docker

To bring up the web app + the API + Postgres together without installing anything
but Docker, from the repo root:

```bash
docker compose up --build
```

This exposes the web app on `http://localhost:3000` and the API on
`http://localhost:8080`. The first time, the migrations need to be applied
(they don't run automatically inside the container):

```bash
cd backend
make migrate-up   # requires local goose, or run it via Docker (see Makefile)
```

Notes on `docker-compose.yml`'s environment variables:

- `NUXT_PUBLIC_API_BASE`: API URL used by the **browser** (calls
  made from the client, e.g. submitting the login form) → `http://localhost:8080/api/v1`.
- `NUXT_API_BASE`: API URL used by the **Nitro server inside the
  container** (SSR calls, e.g. `GET /auth/me` when loading `/`) →
  `http://api:8080/api/v1`, the internal hostname of the `api` service on the
  Compose network. Without this separate variable, server-side rendering would try to
  resolve `localhost:8080` inside the `web` container itself and fail.

## Structure

```
web/
├── server/                       # Nitro layer (BFF); never reaches the browser
│   ├── utils/backend.ts          # httpOnly cookies, refresh + retry, errors
│   └── api/
│       ├── auth/                 # register, login, google, logout, session
│       └── backend/[...path].ts  # authenticated proxy to the Go API
└── app/                          # Nuxt 4 srcDir — the app's code
    ├── app.vue                   # NuxtLayout + NuxtPage
    ├── layouts/default.vue       # shell with nav + logout
    ├── pages/
    │   ├── login.vue             # email/password + Google Sign-In
    │   ├── register.vue          # user sign-up (registers and logs in)
    │   ├── index.vue             # dashboard: user, summary, and decks
    │   ├── decks.vue             # Moxfield import + deck listing
    │   └── statistics.vue        # global stats + per-deck stats
    ├── components/StatCard.vue
    ├── composables/
    │   ├── useAuth.ts            # register/login/loginWithGoogle/logout/fetchSession
    │   ├── useNitroFetch.ts      # fetch to /api/* with per-request cookie jar (SSR)
    │   ├── useApi.ts             # proxy client + error helpers
    │   ├── useDecks.ts           # listing, import, and re-sync (POST /sync/moxfield) with Moxfield
    │   ├── useStatistics.ts      # /statistics/user and /statistics/deck/{id}
    │   └── useGoogleIdentity.ts  # Google Identity Services script
    ├── plugins/session.ts        # hydrates the user before the middleware
    ├── middleware/auth.global.ts # route gating (/login and /register are public)
    └── types/
        ├── api.ts                # Deck (with image_url), SyncResponse, UserStats, DeckStats, PlaygroupStats
        └── google-identity.d.ts  # minimal typing for window.google
```

## Notes

- `GET /statistics/playgroup/{id}` is exposed in `useStatistics()` but has
  no screen: there's no playgroups UI yet to get an id from. Left
  as a future improvement.
- There's no shared logic with the Android client — each one implements
  the same REST contract on its own.
