# Commander Companion — Web (Nuxt)

Cliente web, desacoplado del backend (solo consume la API REST vía HTTP, ver
`docs/api/openapi.yaml`). Ver [ADR-0004](../docs/decisions/0004-web-client-nuxt.md)
para el contexto de la decisión.

Estado actual: registro y login (email/password + Google Sign-In), sesión con
refresh automático, import de decks de Moxfield (con thumbnail del art crop
del comandante, y botón para re-sincronizar un deck ya importado contra
Moxfield) y pantallas de estadísticas (globales del usuario y por deck).

## Stack

- Nuxt 4 (SSR), Tailwind CSS (`@nuxtjs/tailwindcss`), ESLint (`@nuxt/eslint`), npm.

## Setup

```bash
cd web
npm install
cp .env.example .env   # completar NUXT_PUBLIC_API_BASE / NUXT_PUBLIC_GOOGLE_CLIENT_ID
npm run dev            # http://localhost:3000
```

Necesita el backend corriendo (ver `backend/README.md` o la sección de Docker
más abajo) y `CORS_ALLOWED_ORIGINS` en el backend debe incluir el origin de
este cliente (o dejarlo vacío en dev) — aunque con el flujo normal del
cliente web ya no hace falta CORS, ver la sección de Nitro más abajo.

Para que el botón de Google funcione, en Google Cloud Console → **Credentials**
→ el Web Application OAuth Client → **Authorized JavaScript origins**, agregá
el origin donde corre este cliente (ej. `http://localhost:3000`).

Scripts: `npm run dev`, `npm run build`, `npm run lint`, `npm run typecheck`.

## Sesión: por qué hay una capa de Nitro en el medio

El navegador **nunca** habla con la API Go directamente ni ve los tokens.
Todas las llamadas pasan por endpoints propios de Nitro:

- `/api/auth/{register,login,google,logout,session}` — los únicos que tocan
  cookies de sesión.
- `/api/backend/**` — proxy autenticado hacia la API Go (`/api/backend/decks`
  → `GET {API}/decks`). Los paths `auth/*` están bloqueados en el proxy: van
  por `/api/auth/*` para que ningún camino desde JS devuelva un token.

Cookies que maneja Nitro:

| Cookie             | `httpOnly` | Contenido                          |
| ------------------ | ---------- | ---------------------------------- |
| `cc_access_token`  | sí         | JWT de acceso                      |
| `cc_refresh_token` | sí         | refresh token                      |
| `cc_session`       | no         | solo el marcador `"1"`, sin valor sensible |

`cc_session` existe porque el middleware de rutas tiene que decidir si hay
sesión tanto en SSR como en el cliente, y las cookies `httpOnly` no se leen
desde JS. No es un credencial: falsificarla solo consigue que la página
renderice y que el primer request a la API devuelva 401.

Efecto lateral: como el navegador ya no llama a la API Go, **no hace falta
CORS** para el flujo normal del cliente web.

### Refresh automático

`server/utils/backend.ts` (`backendFetch`) agrega el `Authorization: Bearer`
desde la cookie y, si la API responde 401, canjea el refresh token contra
`POST /auth/refresh`, actualiza las cookies y reintenta **una vez**. Mismo
espíritu que el `AuthAuthenticator` de OkHttp del cliente Android.

Dos detalles que importan porque el backend **rota** el refresh token (revoca
el anterior en cada refresh):

- Los refresh concurrentes se deduplican en memoria (`inFlightRefresh`), así
  dos requests que dan 401 a la vez no se pisan revocándose el token.
- En SSR cada llamada interna corre en su propio `H3Event`, así que
  `app/composables/useNitroFetch.ts` mantiene un cookie jar por request: copia
  los `Set-Cookie` a la respuesta que sí ve el navegador **y** los aplica a las
  llamadas siguientes del mismo render (si no, la segunda llamada iría con un
  refresh token ya revocado).

## Probar contra el backend con Docker

Para levantar la web + la API + Postgres juntos sin instalar nada más que
Docker, desde la raíz del repo:

```bash
docker compose up --build
```

Esto expone la web en `http://localhost:3000` y la API en
`http://localhost:8080`. La primera vez hay que aplicar las migraciones
(no corren solas dentro del contenedor):

```bash
cd backend
make migrate-up   # requiere goose local, o correrlo vía Docker (ver Makefile)
```

Notas sobre las variables de entorno de `docker-compose.yml`:

- `NUXT_PUBLIC_API_BASE`: URL de la API que usa el **navegador** (llamadas
  hechas desde el cliente, ej. el submit del login) → `http://localhost:8080/api/v1`.
- `NUXT_API_BASE`: URL de la API que usa el **servidor Nitro dentro del
  contenedor** (llamadas SSR, ej. `GET /auth/me` al cargar `/`) →
  `http://api:8080/api/v1`, el hostname interno del servicio `api` en la red
  de Compose. Sin esta variable separada, el render en servidor intentaría
  resolver `localhost:8080` dentro del propio contenedor `web` y fallaría.

## Estructura

```
web/
├── server/                       # capa Nitro (BFF); nunca llega al navegador
│   ├── utils/backend.ts          # cookies httpOnly, refresh + retry, errores
│   └── api/
│       ├── auth/                 # register, login, google, logout, session
│       └── backend/[...path].ts  # proxy autenticado hacia la API Go
└── app/                          # srcDir de Nuxt 4 — código de la app
    ├── app.vue                   # NuxtLayout + NuxtPage
    ├── layouts/default.vue       # shell con nav + logout
    ├── pages/
    │   ├── login.vue             # email/password + Google Sign-In
    │   ├── register.vue          # alta de usuario (registra y loguea)
    │   ├── index.vue             # dashboard: usuario, resumen y decks
    │   ├── decks.vue             # import de Moxfield + listado de decks
    │   └── statistics.vue        # stats globales + por deck
    ├── components/StatCard.vue
    ├── composables/
    │   ├── useAuth.ts            # register/login/loginWithGoogle/logout/fetchSession
    │   ├── useNitroFetch.ts      # fetch a /api/* con cookie jar por request (SSR)
    │   ├── useApi.ts             # cliente del proxy + helpers de error
    │   ├── useDecks.ts           # listado, import y re-sync (POST /sync/moxfield) de Moxfield
    │   ├── useStatistics.ts      # /statistics/user y /statistics/deck/{id}
    │   └── useGoogleIdentity.ts  # script de Google Identity Services
    ├── plugins/session.ts        # hidrata el usuario antes del middleware
    ├── middleware/auth.global.ts # gating de rutas (/login y /register públicas)
    └── types/
        ├── api.ts                # Deck (con image_url), SyncResponse, UserStats, DeckStats, PlaygroupStats
        └── google-identity.d.ts  # tipado mínimo de window.google
```

## Notas

- `GET /statistics/playgroup/{id}` está expuesto en `useStatistics()` pero no
  tiene pantalla: todavía no hay UI de playgroups de donde sacar un id. Queda
  como mejora futura.
- No hay lógica compartida con `tools/auth-test/` (esa es una herramienta de
  desarrollo standalone, no un cliente real) ni con el cliente Android — cada
  uno implementa el mismo contrato REST por su cuenta.
