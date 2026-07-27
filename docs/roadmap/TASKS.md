# Commander Companion — Lista de tareas

Checklist operativa de todo el trabajo pendiente, organizada por las **Etapas** definidas en [ROADMAP.md](ROADMAP.md). Este documento es la fuente de verdad del progreso día a día: el ROADMAP explica el *qué* y el *porqué*, este archivo trackea el *estado*.

**Cómo mantenerlo actualizado:**
- Marca `[x]` cuando una tarea esté terminada y funcionalmente completa (no solo "compila").
- Si algo está a medias (scaffolding sin lógica real, stub, dummy data), déjalo en `[ ]` y añade una nota entre paréntesis explicando qué falta.
- Añade tareas nuevas según aparezcan; no borres las completadas, son historial útil.
- Actualiza la fecha de "Última revisión" cada vez que se audite el estado real del código.

**Última revisión:** 2026-07-27 (cierre de los 4 entregables pendientes de Stage 0: casos de uso, wireframes, diagramas de flujo/estado y ADRs fundacionales — ver Stage 0 más abajo. + Android conectado al backend real de auth: `LoginScreen`/`LoginViewModel` autentican de verdad contra `POST /auth/login`/`POST /auth/google` (Credential Manager + Google Identity Services), `NetworkModule` (Retrofit/OkHttp) nuevo, `SessionManager` persiste la sesión en DataStore, interceptor+authenticator de refresh-on-401, y logout real con `clearCredentialState` — pendiente solo el paso manual externo de crear el Web Client ID de Google Cloud, código ya listo para recibirlo + cierre de Stage 2 "base de datos": índices explícitos (`00003_indices.sql`), `CHECK` constraints de `games.status`/`game_actions.action_type` (`00004_status_constraints.sql`) y diagrama ER exportado (`docs/diagrams/er-diagram.md`), todo probado up/down/up contra Postgres real + esqueleto inicial del cliente web Nuxt con login email/password + Google + Stage 6: diseño ([ADR-0005](../decisions/0005-websocket-protocol.md)) e implementación real del servidor de sincronización en vivo por WebSocket, `internal/websocket/`, wireado a `games`/`game-actions` sin dependencia dura. Revisión anterior, 2026-07-26: auditoría inicial + detalle de auth con Google OAuth + generación de slices playgroups/games/game-actions y fix de tooling sqlc/lint + quality gates de GitHub Actions + repo vinculado y branch protection activo en `main` + implementación real de auth email/password + Google + CORS + herramienta de test manual `tools/auth-test/` + wiring real de `decks` e import de Moxfield + primeros tests de integración de backend en `auth`/`decks`/`moxfield` + wiring real de `games`/`game-actions`, el motor de partida + wiring real de `playgroups` y `statistics` (recálculo al finalizar partida) — **backend 100% conectado a la base de datos real**, sin módulos dummy pendientes + life tracker local de Android completo con persistencia en Room + `PreGameScreen` con randomizador de turno y tracking de mulligans)

---

## Stage 0: Definición funcional

- [x] Roadmap general (`docs/roadmap/ROADMAP.md`)
- [x] Documento de arquitectura y principios (`docs/architecture/ARCHITECTURE.md`)
- [x] Casos de uso detallados (flujos de usuario paso a paso: crear partida, unirse, trackear vida, finalizar, ver stats) — [`docs/ux/casos-de-uso.md`](../ux/casos-de-uso.md), basado en el código real de `games`/`game-actions` (backend) y de las pantallas de setup/pregame/tracker/history (Android), documentando explícitamente dónde diverge "hoy" (Android 100% local) de "objetivo" (flujo contra el backend)
- [x] Wireframes de las pantallas Android (`docs/ux/` está vacío) — [`docs/ux/wireframes.md`](../ux/wireframes.md): wireframes ASCII + jerarquía de componentes de `LoginScreen`, `DashboardScreen`, `PlayerSetupScreen`, `PreGameScreen`, `GameTrackerScreen` y `HistoryScreen`, leídos del Compose real de cada pantalla
- [x] Diagramas adicionales de flujo/estado (`docs/diagrams/` está vacío; solo existen los 2 diagramas de arquitectura embebidos en el ROADMAP) — [`docs/diagrams/game-state-machine.md`](../diagrams/game-state-machine.md) (máquina de estados `pending → active → finished` con los guards exactos de `games/service.go`, más el sub-estado de eliminación de jugador de `game-actions/service.go`) y [`docs/diagrams/android-navigation-flow.md`](../diagrams/android-navigation-flow.md) (grafo real de `AppNavigation.kt`/`Routes.kt`)
- [x] ADRs de las decisiones fundacionales del proyecto (anteriores a esta sesión, sin contexto propio para redactarlas honestamente): elección de Go+Fiber, PostgreSQL, sqlc+goose, Android nativo (Kotlin/Compose) vs. cross-platform, monolito modular vs. microservicios — redactadas retroactivamente como "decisión heredada, contexto reconstruido" (2026-07-27), numeradas 0006-0010 para evitar colisión con un `0005-websocket-protocol.md` en curso en paralelo en otro worktree:
  - [ADR-0006](../decisions/0006-go-fiber-backend.md): Go + Fiber para el backend
  - [ADR-0007](../decisions/0007-postgresql.md): PostgreSQL como base de datos principal
  - [ADR-0008](../decisions/0008-sqlc-goose.md): sqlc para acceso a datos tipado + goose para migraciones
  - [ADR-0009](../decisions/0009-android-nativo-vs-crossplatform.md): Android nativo (Kotlin + Compose) vs. cross-platform
  - [ADR-0010](../decisions/0010-monolito-modular-vs-microservicios.md): monolito modular vs. microservicios
- [x] ADRs de las decisiones técnicas tomadas en esta sesión, en `docs/decisions/`:
  - [ADR-0001](../decisions/0001-auth-jwt-refresh-token-strategy.md): estrategia de auth (JWT HS256 + refresh token opaco rotativo)
  - [ADR-0002](../decisions/0002-google-sign-in.md): Google Sign-In (auto-vinculación por email verificado, librería `go-oidc`)
  - [ADR-0003](../decisions/0003-cors-permisivo-en-dev.md): CORS abierto por defecto en dev
  - [ADR-0004](../decisions/0004-web-client-nuxt.md): segundo cliente web con Nuxt 4 (esqueleto de auth iniciado)
  - [ADR-0005](../decisions/0005-websocket-protocol.md): protocolo de sincronización en vivo por WebSocket (servidor implementado; ver Stage 6)

## Stage 1: Backend (base del proyecto)

- [x] Proyecto Go inicializado (`go.mod`, `cmd/api/main.go`, Fiber)
- [x] Estructura modular en `internal/` (auth, users, decks, games, game-actions, playgroups, statistics, sync, websocket, common)
- [x] Slices `playgroups`, `games` y `game-actions` generados (query.sql + dto/service/handler esqueleto, CRUD y flujo create→join→leave→start→finish→actions→timeline) y registrados en `cmd/api/main.go`. Los tres módulos ya tienen lógica real (ver Stage 1 y nota de wiring más abajo).
- [x] `sqlc generate` corriendo para los 6 módulos con queries (`users`, `decks`, `playgroups`, `games`, `game-actions`, `statistics`) — se corrigió `sqlc.yaml`, que no tenía `sql_package: "pgx/v5"` y generaba un `DBTX` incompatible con `pgxpool.Pool` (no compilaba)
- [x] **Módulo `auth` real** — `internal/auth/` implementado: JWT (access token) + refresh token opaco con rotación, middleware Fiber, login/refresh/logout/me/google

### Auth — email/password
- [x] Hash de contraseñas con bcrypt (`internal/users/service.go: RegisterUser`, `bcrypt.GenerateFromPassword`/`CompareHashAndPassword`)
- [x] `POST /auth/login` real: verifica credenciales contra la BD (`users.VerifyCredentials`) y emite access token (JWT HS256) + refresh token
- [x] `POST /auth/refresh` real: valida el refresh token contra `refresh_tokens` (hash SHA-256 persistido, el token en claro nunca se guarda), lo **revoca y rota** (nuevo refresh token en cada uso) y emite un nuevo access token
- [x] `POST /auth/logout` real: revoca el refresh token indicado (`revoked_at`) contra la tabla `refresh_tokens`
- [x] `GET /auth/me` real: resuelve el usuario desde el `user_id` que dejó el middleware en el contexto (`common.UserIDKey`)
- [x] Middleware Fiber (`auth.RequireAuth`) que valida el Bearer JWT y mete el `user_id` en `c.Locals(common.UserIDKey)`
- [x] Middleware aplicado a todas las rutas protegidas vía grupo `api.Group("", auth.RequireAuth(...))` en `main.go`; eliminados los `"dummy-user-id"` hardcodeados de `decks/handler.go` y `statistics/handler.go` (ahora leen `common.UserIDKey`)
- [x] Expiración y firma de tokens: **HS256 simétrico** con `JWT_SECRET` (env var, default inseguro solo de dev con `//nolint:gosec` justificado) — decisión: RS256 no aporta valor en un monolito sin verificadores externos del token; TTLs configurables por env (`ACCESS_TOKEN_TTL` default 15m, `REFRESH_TOKEN_TTL` default 720h/30d). Detalle y alternativas consideradas: [ADR-0001](../decisions/0001-auth-jwt-refresh-token-strategy.md)

### Auth — Google OAuth (Sign-In)
- [x] ADR: decisión de añadir Google como proveedor OAuth junto a email/password (alcance: solo login social, no reemplaza al password) — [ADR-0002](../decisions/0002-google-sign-in.md)
- [ ] **Crear credenciales OAuth en Google Cloud Console** (Web Client ID + Android Client ID) — paso manual externo, requiere acceso a una cuenta de Google Cloud del proyecto; el backend ya está listo para recibirlo vía `GOOGLE_CLIENT_ID`. Sin configurar, `POST /auth/google` responde `501 Not Implemented` en vez de crashear
- [x] Cambios de esquema en BD (ver Stage 2): `google_id` en `users`, `password_hash` nullable, `CHECK (password_hash IS NOT NULL OR google_id IS NOT NULL)` (migración `00002_auth.sql`, probada up/down/up contra Postgres real)
- [x] `POST /auth/google`: recibe `id_token`, lo verifica contra Google (issuer + audience = `GOOGLE_CLIENT_ID` + firma vía JWKS) y emite el mismo par access/refresh token que `/auth/login`
- [x] Verificación del `id_token` en Go — se usó `github.com/coreos/go-oidc/v3` en vez de `google.golang.org/api/idtoken`: mismo resultado (discovery + JWKS + validación de issuer/audience/firma) con una huella de dependencias mucho más liviana (evita arrastrar todo `google.golang.org/api` + gRPC + OpenTelemetry solo para verificar un token). El discovery document de Google se resuelve de forma perezosa en el primer login con Google, no al arrancar el servidor
- [x] Lógica de alta/vinculación de cuenta (`users.FindOrCreateGoogleUser`): busca por `google_id` → si no existe, busca por email y **vincula automáticamente** si Google confirma `email_verified` → si tampoco existe, crea usuario nuevo sin password (username derivado del local-part del email, con reintento sufijado ante colisión)
- [x] Variable de entorno `GOOGLE_CLIENT_ID` documentada en `.env.example`
- [x] Manejo de errores específicos: token inválido/expirado → 401, `email_verified: false` → 400, `GOOGLE_CLIENT_ID` no configurado → 501

### Games / game-actions — motor de partida
- [x] `games` conectado a `Queries` reales (sqlc): `CreateGame`, `GetGame`/`ListGames` (con `players` embebido en el detalle), `JoinGame`, `LeaveGame`, `StartGame`, `FinishGame`
- [x] Máquina de estados `pending → active → finished` aplicada server-side: `join`/`leave` solo en `pending`, `start` requiere ≥2 jugadores (`minPlayersToStart`) y solo desde `pending`, `finish` solo desde `active`, cualquier transición inválida devuelve 409
- [x] `JoinGame` ya no acepta `user_id` en el body (lo tomaba de ahí, permitiendo que cualquiera anotara jugadores a nombre de otro usuario): el jugador es siempre el usuario autenticado vía JWT; el body solo lleva `deck_id`. Se verifica ownership del deck (mismo criterio que `decks`: 404 "deck not found" tanto si no existe como si es de otro usuario, para no revelar cuál de los dos casos es) y que el usuario no esté ya sentado en esa partida (409)
- [x] `game-actions` conectado a `Queries` reales: valida `action_type` contra el vocabulario fijo (`LifeChange`, `CombatDamage`, `CommanderDamage`, `PoisonCounter`, `TurnStart`, `TurnEnd`, `Elimination`), resuelve actor/target como `game_players` de esa partida (no `users`) y solo permite registrar acciones si la partida está `active`
- [x] Los actions mutan el estado real del jugador afectado (no solo quedan en el log): `LifeChange`/`CombatDamage`/`CommanderDamage` ajustan `life_total`, `PoisonCounter` ajusta `poison_counters`, `Elimination` marca `is_eliminated`; auto-eliminación server-side cuando `life_total <= 0` o `poison_counters >= 10` (reglas estándar de Commander)
- [x] Tests de integración (`internal/games/service_test.go`, `internal/game-actions/service_test.go`, Postgres real) cubriendo la máquina de estados completa y los efectos de cada `action_type`, además del smoke test manual end-to-end por curl (ver Transversal)
- [ ] `CommanderDamage` no distingue la fuente del daño por oponente (regla real de Commander: 21 de daño de comandante de una misma fuente elimina), porque el esquema (`game_players`) solo trackea `life_total` agregado, sin una tabla de daño por par jugador-comandante — hoy se comporta igual que `CombatDamage`. Requeriría una migración nueva, fuera de alcance de esta pasada
- [ ] `TurnStart`/`TurnEnd` quedan como marcadores de solo-log: el esquema no tiene columna de "de quién es el turno actual" en `games`
- [x] `openapi.yaml` actualizado: `CreateGameRequest`, `JoinGameRequest`, `Game.players`/`Game.finished_at`, enum de `action_type` y responses 400/404/409 en los endpoints de `/games/*`

### Playgroups — grupos de juego
- [x] `playgroups` conectado a `Queries` reales: `CreatePlaygroup` auto-une al creador como primer miembro; `GetPlaygroup`/`ListPlaygroups` quedan **acotados a membresía** (un usuario solo ve/lista los grupos de los que es miembro; no hay listado público de todos los grupos de la plataforma)
- [x] `GetPlaygroup` de un grupo del que no sos miembro devuelve 404 sin distinguir de "no existe" (mismo criterio que `decks`/`games`), para no revelar grupos ajenos por ID
- [x] `AddMember` requiere que quien invita ya sea miembro del grupo (404 si no lo es, mismo criterio de no-revelar) y que el usuario a añadir exista (404 si no) y no esté ya en el grupo (409)
- [x] Tests de integración (`internal/playgroups/service_test.go`, Postgres real): auto-join del creador, scoping por membresía, y las 3 validaciones de `AddMember`

### Statistics — recálculo real y consultas
- [x] `games/service.go: FinishGame` ya no tiene el TODO — dispara `statistics.RecalculateForGame(gameID)` de verdad tras finalizar (vía una interfaz `games.StatisticsRecalculator` para poder mockearla en tests; wiring real en `main.go` con `statistics.NewService` inyectado a `games.NewService`)
- [x] `RecalculateForGame` recorre a los jugadores y acciones de la partida ya finalizada y actualiza (upsert incremental, `ON CONFLICT DO UPDATE`) `user_statistics_summary` y `deck_statistics_summary` para cada participante:
  - `games_played`: +1 para todos los participantes
  - `games_won`: +1 solo si hay un único jugador no eliminado al finalizar (`sole survivor`); si la partida se cortó a mano con 2+ sobrevivientes, no se acredita victoria a nadie (pero sí se cuenta `games_played`)
  - `total_damage_dealt`/`total_commander_damage_dealt`: suma de los `amount` de `CombatDamage`/`CommanderDamage` atribuidos al actor
  - `highest_life_total_achieved` (por deck): recalculado **repitiendo el log de acciones** en orden cronológico desde el baseline de 40, no solo tomando el valor final de `life_total`
  - `total_eliminations`: cuenta únicamente acciones `Elimination` explícitas con un target distinto del actor — las auto-eliminaciones por vida/veneno (ver `game-actions`) no quedan atribuidas a un actor específico en el log, así que no suman acá (documentado como límite conocido, no como bug)
- [x] `GetUserStats`/`GetDeckStats` reales: leen de las tablas de resumen; si un usuario/deck nunca terminó una partida no hay fila (no es un error) y se devuelven ceros en vez de 404
- [x] `GetDeckStats` con ownership check (404 si el deck no es del usuario autenticado, mismo criterio que `decks`)
- [x] `GetPlaygroupStats` (antes `501 Not Implemented` hardcodeado) implementado con agregación **en vivo** (no hay tabla de resumen por grupo): partidas finalizadas del grupo + jugadas/ganadas por miembro, con el mismo criterio de "único sobreviviente" para determinar el ganador
- [x] Tests de integración (`internal/statistics/service_test.go`, Postgres real, arman partidas completas vía `games`+`game-actions` reales): ganador acreditado con daño correcto, sin ganador cuando hay 2+ sobrevivientes, acumulación entre partidas del mismo usuario, ownership de `GetDeckStats`, ceros por defecto, y agregación de `GetPlaygroupStats`
- [x] `openapi.yaml` actualizado: `CreatePlaygroupRequest`/`AddMemberRequest`/`Playgroup.members`, y `UserStatsResponse`/`DeckStatsResponse`/`PlaygroupStatsResponse` documentados (antes no existían en el spec)

### Infra / configuración
- [ ] `configs/` poblado con configuración por entorno (hoy está vacío; todo se lee de variables de entorno sueltas en `main.go`)
- [x] `.env.example` documentando las variables requeridas (`DB_URL`, `PORT`, `JWT_SECRET`, `ACCESS_TOKEN_TTL`, `REFRESH_TOKEN_TTL`, `GOOGLE_CLIENT_ID`, `CORS_ALLOWED_ORIGINS`); `docker-compose.yml` actualizado con las mismas (y corregido: usaba `DB_HOST`/`DB_USER`/etc. sueltas que `main.go` nunca leyó — ahora usa `DB_URL` real)
- [x] CORS habilitado (`github.com/gofiber/fiber/v2/middleware/cors`), configurable vía `CORS_ALLOWED_ORIGINS` (vacío = `*`, razonable en dev porque la API solo usa Bearer tokens, nunca cookies). Necesario para que cualquier frontend web (incluida `tools/auth-test/` y el futuro cliente Nuxt) pueda llamar a la API desde el navegador. Detalle: [ADR-0003](../decisions/0003-cors-permisivo-en-dev.md)
- [x] `tools/auth-test/`: página HTML de un solo archivo, sin build, para probar manualmente el flujo de auth completo (password + Google Sign-In real vía Google Identity Services) contra una instancia corriendo del backend. No es parte del producto, ver su `README.md`
- [ ] Revisar `Dockerfile` y `docker-compose.yml` (hoy viven en `backend/`, pero `docker/` en la raíz está vacío — decidir dónde centralizar)
- [x] Limpieza: carpeta residual `backend;C` en la raíz del repo (eliminada, 2026-07-26)

## Stage 2: Base de datos

- [x] Esquema DBML inicial (`docs/database/schema.dbml`)
- [x] Migración inicial coherente con el DBML (`migrations/00001_initial_schema.sql`)
- [x] Índices explícitos más allá de las PK: `decks.moxfield_id`, `game_actions.game_id`, `game_players.game_id` para lecturas frecuentes del timeline/estado (`migrations/00003_indices.sql`, `docs/database/schema.dbml` actualizado con los bloques `indexes {}` correspondientes). Probado up/down/up contra Postgres real (Docker); confirmados con `\d` que los índices quedan creados
- [x] Constraints adicionales: `CHECK (status IN ('pending', 'active', 'finished'))` en `games` y `CHECK (action_type IN ('LifeChange', 'CombatDamage', 'CommanderDamage', 'PoisonCounter', 'TurnStart', 'TurnEnd', 'Elimination'))` en `game_actions` (`migrations/00004_status_constraints.sql`), en vez de convertir a enum de Postgres para no romper el tipo Go (`string`) que sqlc genera para esas columnas — se optó por `CHECK` sobre el `varchar` existente por ser el cambio menos invasivo. Probado up/down/up contra Postgres real y verificado que un `INSERT` con un `status` inválido es rechazado por la constraint; `sqlc generate` (versión pineada en CI, `1.27.0`) corrido después sin diffs pendientes
- [x] Migración: `users.google_id varchar unique nullable` + `users.password_hash` nullable (`migrations/00002_auth.sql`, `docs/database/schema.dbml` actualizado) + tabla `refresh_tokens` (`token_hash` único, `expires_at`, `revoked_at`)
- [x] Constraint a nivel de BD: `CHECK (password_hash IS NOT NULL OR google_id IS NOT NULL)` en `users`
- [x] Diagrama ER visual exportado a `docs/diagrams/` (`docs/diagrams/er-diagram.md`, Mermaid `erDiagram` con las 11 tablas del esquema real, atributos, PK/FK/UK y notas de los índices/constraints de `00003`/`00004`; sintaxis validada renderizando con `@mermaid-js/mermaid-cli`)
- [ ] Estrategia de migraciones futuras (naming, cómo versionar cambios de estadísticas pre-calculadas)

## Stage 3: API (contrato OpenAPI)

- [x] Esqueleto OpenAPI 3.1 con los paths principales
- [x] `requestBody` + schemas completos de auth: `RegisterRequest`, `LoginRequest`, `GoogleLoginRequest`, `RefreshRequest`, `LogoutRequest`, `TokenResponse` (con sus códigos de error: 401/400/409/501 según corresponda)
- [x] `requestBody` + schemas completos de `/decks` (`CreateDeckRequest`) y `/decks/import/moxfield` (`ImportMoxfieldRequest`), con 400/404
- [x] `requestBody` + schemas completos de `/games` (`CreateGameRequest`) y `/games/{id}/join` (`JoinGameRequest`), con 400/404/409
- [x] `requestBody` + schemas completos de `/playgroups` (`CreatePlaygroupRequest`) y `/playgroups/{id}/members` (`AddMemberRequest`, path que ni siquiera estaba documentado antes), con 400/404/409
- [x] Documentar `POST /auth/google` en el OpenAPI (request `{ id_token: string }`, respuesta igual a `/auth/login`: `TokenResponse`)
- [x] Schemas de respuesta de estadísticas: `UserStatsResponse`, `DeckStatsResponse`, `PlaygroupStatsResponse`/`PlaygroupMemberStats`, ya modelados y referenciados en los 3 endpoints de `/statistics/*`
- [x] Endpoint `/statistics/playgroup/{id}` implementado de verdad (antes devolvía `501` hardcodeado sin importar qué dijera el servicio)
- [ ] Paginación cursor-based en listados (`/games`, `/decks`) — mencionada en el ROADMAP pero no existe ni en el spec ni en el código
- [x] Lint del spec con Spectral (`.spectral.yaml`, corre en `docs-ci.yml`) — 0 errores; quedan 93 warnings preexistentes en todo el spec (`operationId`/`tags`/`description` faltantes en casi todos los paths), no bloqueantes, pendientes como deuda de documentación

## Stage 4b: Cliente Web (Nuxt) — esqueleto iniciado

- [x] Decisión tomada 2026-07-26, actualizada 2026-07-27: agregar un segundo cliente, un **frontend web con Nuxt 4 + Tailwind**, 100% desacoplado del backend (solo consume la API REST, igual que Android), para import de decks de Moxfield y ver estadísticas post-partida. Vive en `web/` en la raíz del repo (al lado de `android/` y `backend/`), con **npm** como gestor de paquetes, en modo **SSR** (no SPA pura — requiere su propio proceso Node/Nitro corriendo en runtime, a diferencia de `tools/auth-test/` que es estático). ADR formal: [ADR-0004](../decisions/0004-web-client-nuxt.md) (Nuxt 3 → Nuxt 4: criterio del proyecto de arrancar cada dependencia nueva en su versión mayor más actualizada)
- [x] **Esqueleto inicial scaffoldeado** (2026-07-27, migrado de Nuxt 3 a Nuxt 4 el mismo día), a pedido explícito del usuario adelantando el orden original (el ADR proponía esperar a que el backend de partidas/estadísticas estuviera terminado; se decidió arrancar ya solo con auth para poder levantar el proyecto):
  - Nuxt 4.5.0 + `@nuxtjs/tailwindcss`, npm, SSR (`nuxt build`/`nuxt dev` verificados localmente, sin SPA). Código de la app bajo `web/app/` (srcDir por defecto de Nuxt 4)
  - `app/pages/login.vue`: form email/password + botón de Google Sign-In (Google Identity Services, mismo flujo que `tools/auth-test/` pero como composable Vue: `app/composables/useGoogleIdentity.ts`)
  - `app/pages/index.vue`: única pantalla protegida por ahora, muestra el usuario autenticado (`GET /auth/me`) y permite cerrar sesión
  - `app/composables/useAuth.ts`: `login`/`loginWithGoogle`/`logout`/`fetchMe` contra `POST /auth/login`, `/auth/google`, `/auth/logout`, `GET /auth/me`; sesión (access/refresh token) en cookies vía `useCookie` (no `httpOnly` todavía — pendiente de endurecer si hace falta mitigar XSS)
  - `app/middleware/auth.global.ts`: redirige a `/login` sin sesión y fuera de `/login` con sesión, verificado en SSR (`GET /` con cookies vacías responde 200 tras redirect a `/login`)
  - Fuera de alcance de esta pasada: import de decks de Moxfield y estadísticas (siguen esperando el motor de partida + estadísticas reales, ver nota original más abajo), registro de usuario nuevo (solo login, no hay `pages/register.vue`), refresh automático de token expirado (el composable expone `logout`/`fetchMe` pero no reintenta con `/auth/refresh` todavía)
- [ ] Orden original acordado con el usuario (ya adelantado parcialmente arriba, solo para auth): antes de construir import de Moxfield/estadísticas falta el motor de partida + estadísticas reales (ver nota de alcance en Stage 7 y en "Wiring real" más abajo) para que esas pantallas tengan datos reales que mostrar
- [ ] Registro de usuario nuevo (`POST /auth/register`) — no tiene pantalla todavía, solo login
- [ ] Refresh automático de sesión expirada (`POST /auth/refresh`) — no está conectado desde el frontend
- [ ] Endurecer almacenamiento de tokens (cookies `httpOnly` vía endpoint propio de Nitro, en vez de cookies legibles desde JS) si se decide que hace falta mitigar XSS antes de producción

## Stage 4: Cliente Android (base)

- [x] Proyecto inicializado: Compose, Material 3, Navigation, Hilt, Room, Retrofit, kotlinx.serialization
- [x] Theming base (`Color.kt`, `Theme.kt`, `Type.kt`) y navegación con rutas (`AppNavigation.kt`, `Routes.kt`)
- [x] **Life tracker local completo y funcional** (2026-07-26), 100% en memoria + Room, sin depender del backend:
  - `PlayerSetupScreen`: elegir 2-6 jugadores, nombre y color (paleta WUBRG + incoloro) por jugador
  - `GameTrackerScreen`: grid dinámico (filas de hasta 2 cartas, funciona para 2-6 jugadores en vez del layout fijo de 4 anterior), contador de turno, vida ±, panel de daño de comandante por oponente (`PlayerCard`) que también resta vida
  - `GameViewModel`: fin de partida automático cuando solo queda 1 jugador con vida > 0, o manual con botón "Finalizar" + diálogo de confirmación; detecta ganador (o empate si hay más de un jugador con la vida máxima)
  - Persistencia real en Room: `GameEntity`/`PlayerResultEntity`/`GameWithPlayers` (`DatabaseModule` nuevo en Hilt) — la partida se guarda al crearse (`IN_PROGRESS`) y se actualiza al finalizar (`FINISHED` + vida final + ganador)
  - `HistoryScreen`: lista de partidas pasadas (fecha, nº de jugadores, estado, vida final y color de cada jugador) leída de Room vía `HistoryViewModel`
  - Verificado end-to-end en emulador (`Pixel_10_Pro`): setup → tracker → daño de comandante → finalizar → historial persistido
  - Deliberadamente fuera de alcance de esta pasada: recuperación de partida en curso tras kill del proceso (el estado de vida vive solo en memoria hasta finalizar), autenticación, y cualquier llamada al backend — sigue siendo 100% local
- [x] **Pantalla de pre-partida: sorteo de turno + mulligans** (`PreGameScreen`, 2026-07-26), insertada entre `PlayerSetupScreen` y `GameTrackerScreen`:
  - Randomizador "¿Quién empieza?": elige al azar uno de los jugadores configurados (`Random.nextInt`), lo resalta con su color y el resultado viaja a `GameTrackerRoute` (`startingPlayerSeat`) para mostrar el badge "· empieza" en su `PlayerCard` durante toda la partida
  - Contador de mulligans por jugador (± , sin límite superior, mínimo 0) antes de la primera mano; se guarda en `PlayerState.mulligans` y se persiste en `PlayerResultEntity.mulligans` (columna nueva) — visible tanto en el tracker (badge bajo el nombre si > 0) como en el historial (`"vida (Nm)"`)
  - `PlayerConfig`/`PlayerConfigCodec` extendidos con un tercer campo `mulligans` (formato `name|colorKey|mulligans`, decodificación retrocompatible con encodes de 2 campos) para llevar el dato de `PlayerSetupScreen` → `PreGameScreen` → `GameTrackerScreen` sin más estado compartido que la propia ruta
  - **Gotcha real encontrado y corregido**: añadir la columna `mulligans` a `PlayerResultEntity` sin subir `CommanderDatabase` de `version = 1` a `version = 2` crashea la app en cualquier dispositivo con una instalación previa (`Room cannot verify the data integrity` — `fallbackToDestructiveMigration` solo se dispara ante un cambio de versión, no ante un cambio de esquema en la misma versión). Bump de versión aplicado; queda como recordatorio para futuros cambios de entidades Room
  - Verificado end-to-end en emulador: sorteo → mulligans → tracker (badges correctos) → finalizar → historial con mulligans persistidos
- [x] Pantallas placeholder: `DashboardScreen` (ahora con navegación real a setup/historial), componente `PlayerCard` (ahora con lógica real de daño de comandante)
- [x] **Flujo de navegación de la app definido** (decisión, 2026-07-26; actualizado con `PreGameRoute`): `LoginRoute → DashboardRoute → (PlayerSetupRoute → PreGameRoute → GameTrackerRoute → vuelta a Dashboard) / HistoryRoute`. `LoginRoute` es ahora el `startDestination` del `NavHost`. Decisiones tomadas:
  - Se implementa el login como **shell de navegación primero, sin autenticar todavía**, para fijar la estructura de rutas antes de conectar el backend real — evita tener que reestructurar el grafo de navegación (y las pantallas que ya asumen "usuario logueado") más adelante
  - Los dos métodos de auth que ya soporta el backend (`POST /auth/login` y `POST /auth/google`) tienen **callbacks separados desde ya** en `LoginScreen` (`onLoginWithPassword`, `onLoginWithGoogle`), diferenciados también visualmente (botón sólido vs. botón outline + separador "o"), aunque hoy ambos solo navegan a `DashboardRoute` — conectar cada uno a su endpoint real será un cambio acotado en el callback, no un rediseño de pantalla
  - Al llegar a `DashboardRoute` se hace `popUpTo(LoginRoute) { inclusive = true }`: una vez "dentro" de la app el botón atrás no debe volver al login
  - El life tracker sigue siendo 100% local (Room) independientemente del login: no hay ninguna llamada al backend todavía, ni desde el login ni desde la partida — son dos piezas construidas en paralelo que se conectarán juntas cuando `CommanderApi.kt` tenga endpoints reales
- [x] Pantallas de autenticación (login/registro) — `LoginScreen` ahora autentica de verdad contra el backend vía `LoginViewModel` (2026-07-27):
  - [x] Dependencia Credential Manager + Google Identity Services (`androidx.credentials`, `androidx.credentials:credentials-play-services-auth`, `com.google.android.libraries.identity.googleid`)
  - [x] `onLoginWithGoogle` conectado al flujo real de Credential Manager (`GoogleAuthClient.getIdToken`) para obtener el `id_token` — **caveat**: las credenciales OAuth reales de Google Cloud (Web Client ID) todavía no existen (paso manual externo, ver Stage 1); `BuildConfig.GOOGLE_WEB_CLIENT_ID` es un placeholder documentado en `app/build.gradle.kts`, así que el flujo está completo e implementado correctamente pero no se pudo verificar end-to-end contra Google real. En cuanto se cree el Client ID real, basta con pasarlo por `-PGOOGLE_WEB_CLIENT_ID=...` o `gradle.properties`, sin tocar código
  - [x] `onLoginWithPassword`/`onLoginWithGoogle` conectados a `POST /auth/login` / `POST /auth/google` (`AuthApi.kt` nuevo, separado de `CommanderApi.kt`) y los tokens devueltos se guardan en `SessionManager` (DataStore)
  - [x] Manejo de estado en `LoginViewModel`/`LoginScreen`: credenciales inválidas (401), usuario cancela el picker de Google (`GoogleSignInCancelledException`, no muestra error), sin cuenta Google en el dispositivo (`NoGoogleAccountException`), backend rechaza el token de Google (400/501) y error de red (`IOException`)
  - [x] Flujo de "Cerrar sesión" (`DashboardScreen` → `DashboardViewModel.logout()` → `SessionManager.logout()`): revoca el refresh token contra `POST /auth/logout` (best-effort), limpia DataStore y llama `clearCredentialState` de Google
- [ ] Capa de dominio (`domain/` con use cases e interfaces de repositorio) — no existe, se salta directo de UI a datos (el life tracker inyecta `GameDao` directo en el `ViewModel`, y ahora el login inyecta `AuthApi`/`SessionManager` directo en `LoginViewModel` con el mismo criterio; aceptable para el alcance actual, revisar si se justifica más adelante)
- [ ] Repositorios reales en `data/repository/` — no existe todavía; hoy `GameDao` se inyecta directo en `GameViewModel`/`HistoryViewModel`, y `AuthApi`/`SessionManager` directo en `LoginViewModel`/`DashboardViewModel`
- [x] DI de Room: `DatabaseModule` (Hilt) provee `CommanderDatabase` y `GameDao`. **Módulo de red agregado** (2026-07-27): `NetworkModule` (Hilt) provee Retrofit/OkHttp — `AppModule` seguía proveyendo solo `Context`, ahora la red vive en su propio módulo
- [x] `GameViewModel.kt` — ya no está vacío: vida, turno, daño de comandante y persistencia del resultado en Room (ver nota del life tracker más arriba)
- [ ] `CommanderApi.kt` — sigue teniendo solo `GET /health` (deliberadamente no tocado en esta pasada, ver nota de `AuthApi.kt` abajo); faltan los endpoints reales de decks/games/game-actions/statistics — necesario para conectar el life tracker a partidas reales del backend en vez de jugarse 100% local. Ya tiene un provider Hilt en `NetworkModule` (cliente autenticado), listo para cuando se agreguen esos endpoints
- [x] `AuthApi.kt` nuevo (`data/remote/api/AuthApi.kt`), separado de `CommanderApi.kt` a propósito (evitar conflictos de merge con quien extienda decks/games/etc. en paralelo): cubre `login`/`google`/`refresh`/`logout`/`me` de `docs/api/openapi.yaml`, con sus DTOs en `data/remote/dto/AuthDto.kt`
- [x] `GameState.kt` — modela vida, turno y daño de comandante por oponente para N jugadores (2-6). Veneno/energía/experiencia siguen sin modelar (no forman parte del alcance de esta pasada)

## Stage 5: Integración Android ↔ Backend

- [x] Interceptor de autenticación (2026-07-27): `AuthInterceptor` adjunta el Bearer access token a las requests del cliente HTTP autenticado (`NetworkModule`, usado por `CommanderApi`); `AuthAuthenticator` intercepta los 401, llama a `POST /auth/refresh` una vez y reintenta la request original con el token nuevo. Válido tanto para sesiones de password como de Google (ambas guardan el mismo `TokenResponse` en `SessionManager`). Si el refresh falla, `SessionManager` fuerza logout (limpia DataStore + `clearCredentialState`) y emite un evento (`forcedLogoutEvents`) que `AppNavigation`/`SessionViewModel` escuchan para volver a `LoginRoute` desde cualquier pantalla. Nota de diseño: `AuthApi` (login/google/refresh/logout) usa un cliente OkHttp *sin* este interceptor a propósito — esos endpoints son públicos (`security: []` en el spec) y reusar el cliente autenticado para el propio refresh causaría una recursión
- [x] Persistencia de sesión con DataStore (2026-07-27): `SessionManager` (`data/session/SessionManager.kt`) guarda access token, refresh token y expiry (`androidx.datastore:datastore-preferences`, agregado a `app/build.gradle.kts`)
- [ ] Flujo end-to-end: registro/login (password o Google) → dashboard → crear/unirse a partida → tracker de vida en tiempo real → finalizar partida → ver resultado
- [ ] Room como caché offline-first (partidas vistas, decks propios) con estrategia de sincronización

## Stage 6: Sincronización (Websocket)

- [x] Diseño del protocolo de mensajes (qué eventos de `game_actions` se retransmiten en vivo y su formato) — [ADR-0005](../decisions/0005-websocket-protocol.md): retransmite las 7 acciones de `game_actions` sin filtrar (mismo DTO `GameActionResponse` que ya usa REST) + un evento de ciclo de vida `game_finished`; sobre JSON común (`type`/`game_id`/`actor_id`/`payload`/`timestamp`); autenticación por mensaje `{"type":"auth","token":"..."}` como primer frame tras conectar (no JWT por query param, para no filtrarlo en los logs de acceso de Fiber; no subprotocolo, por el uso semánticamente incorrecto del campo); documenta explícitamente fuera de alcance: replay/garantías de entrega al reconectar, autorización por membership del `game_id`, multi-proceso/pub-sub externo, presencia, canal cliente→servidor, heartbeat applicativo
- [x] Implementación del servidor en `internal/websocket/` (antes carpeta vacía): `Hub` en memoria (registro/broadcast/cierre de sala por `game_id`, protegido con `sync.RWMutex`), `Client` que envuelve la conexión de `github.com/gofiber/websocket/v2` (ya usado por `gofiber/fiber`, dependencia nueva agregada), handler de upgrade + autenticación (`auth.VerifyAccessToken`, nueva función exportada que reusa la verificación de JWT ya existente en `internal/auth/token.go` sin duplicarla) en `GET /api/v1/ws/games/:id` (ruta pública, sin `auth.RequireAuth`, ya que el handshake no puede llevar el header `Authorization`). Wireado sin dependencia dura: `games.Broadcaster`/`gameactions.Broadcaster` son interfaces definidas del lado del consumidor (mismo patrón que `games.StatisticsRecalculator`) que `*websocket.Hub` satisface; `game-actions.RecordAction` retransmite cada acción tras persistirla, `games.FinishGame` retransmite `game_finished` y cierra la sala. Tests unitarios puros del `Hub` (`internal/websocket/hub_test.go`: register/unregister, broadcast solo a la sala correcta, sala desconocida es no-op, `CloseRoom` cierra y limpia) sin necesidad de Postgres ni de un socket real (fake `Conn` en memoria). `go build`/`go vet`/`golangci-lint run` limpios (los 34 "File is not properly formatted (gofmt)" que golangci-lint reporta en el repo son un artefacto preexistente del checkout en Windows con `core.autocrlf` — afectan también archivos jamás tocados en esta pasada, ej. `internal/moxfield/client.go`; `internal/websocket/` en particular da 0 issues)
- [ ] Cliente websocket en Android (conexión, reconexión con backoff, aplicar eventos entrantes al `GameState`) — pendiente; consume el protocolo de [ADR-0005](../decisions/0005-websocket-protocol.md)

## Stage 7: Estadísticas

- [x] Lógica real de recálculo al finalizar partida — `games/service.go: FinishGame` dispara `statistics.RecalculateForGame`; ver detalle completo en Stage 1 ("Statistics — recálculo real y consultas")
- [x] Queries de agregación para `user_statistics_summary` y `deck_statistics_summary` (`UpsertUserStatistics`/`UpsertDeckStatistics`, upsert incremental con `ON CONFLICT DO UPDATE`)
- [x] `internal/statistics` service real: `GetUserStats`/`GetDeckStats` consultan la BD de verdad (con ownership check en deck y ceros por defecto si nunca jugó)
- [x] Servicio y endpoint de estadísticas por playgroup (`GetPlaygroupStats`, agregación en vivo sobre partidas finalizadas)
- [ ] UI de estadísticas en Android (no hay pantalla) — bloqueado además por que Android todavía no habla con el backend (ver Stage 4/5)

## Stage 8: Importación Moxfield

- [x] Investigar la API pública/no oficial de Moxfield a integrar — `GET https://api2.moxfield.com/v3/decks/all/{publicId}`. Está detrás de Cloudflare: bloquea requests sin un `User-Agent` que parezca un navegador real (probado con `curl` — sin headers da la página de challenge de Cloudflare; con `User-Agent` de Chrome responde JSON normal, incluso sin `Referer`). El `publicId` es el segmento final de la URL del deck (`moxfield.com/decks/{publicId}`); los comandantes están en `boards.commanders.cards[*].card.name`.
- [x] Cliente HTTP hacia Moxfield en el backend — `internal/moxfield` (`Client.GetDeck`, `ExtractPublicID` para aceptar URL completa o solo el ID)
- [ ] Reemplazar el stub de `internal/sync/service.go` (hoy devuelve `"queued"`/`"in_progress"` sin hacer nada real) por lógica de sincronización real — fuera de alcance de esta pasada (era solo import puntual, no re-sync de decks ya importados)
- [x] Endpoint `POST /decks/import/moxfield` implementado end-to-end: resuelve el ID (URL o bare ID), llama a Moxfield, crea el deck con `name`/`commander` reales y `moxfield_id` seteado. 404 si el deck de Moxfield no existe, 400 si no tiene comandante (deck no es de formato Commander). Probado contra la API real (dos decks públicos distintos, uno vía URL completa y otro vía ID)
- [ ] Manejo de errores, rate limiting y reintentos ante la API externa — hoy cualquier error de red/parseo de Moxfield se propaga como 500 genérico; no hay retry ni backoff

## Transversal (calidad, infraestructura, seguridad)

- [ ] Tests unitarios de backend — arrancado: tests de integración (Postgres real, `internal/testutil`) para `auth` (login, rotación/expiración/revocación de refresh tokens, cuentas Google-only, `Me`), `decks` (ownership check en `GetDeck`/`DeleteDeck`, listado por usuario, import de Moxfield con cliente mockeado), `games` (máquina de estados completa: join/leave solo en pending, ownership de deck al unirse, mínimo de jugadores para iniciar, transiciones inválidas → 409), `game-actions` (validación de `action_type`, mutación real de `life_total`/`poison_counters`, auto-eliminación a 0 de vida y a 10 de veneno, orden del timeline, partida no-activa → 409), `playgroups` (auto-join del creador, scoping por membresía, validaciones de `AddMember`) y `statistics` (recálculo end-to-end jugando partidas reales: ganador, sin-ganador, acumulación entre partidas, ownership, agregación por playgroup), más tests unitarios puros de `moxfield.ExtractPublicID`. `go test` corre con `-p 1` (los tests comparten la misma base y hacen `TRUNCATE` entre sí). Falta únicamente `users` (cubierto indirectamente por los tests de los demás módulos, que lo usan para crear usuarios de fixture, pero sin tests propios)
- [ ] Tests de Android (0 tests actualmente)
- [x] CI/CD con GitHub Actions — quality gates creados en `.github/workflows/`:
  - `backend-ci.yml`: gofmt + go vet, golangci-lint, `sqlc generate` sin diffs pendientes, build + `go test -race` + migraciones goose contra Postgres real (servicio en el job), hadolint sobre `backend/Dockerfile`
  - `android-ci.yml`: Android Lint (`lintDebug`), tests unitarios (`testDebugUnitTest`), `assembleDebug`, publica reportes como artifact
  - `docs-ci.yml`: valida las fuentes de verdad — Spectral lint sobre `docs/api/openapi.yaml` (ruleset en `.spectral.yaml`) y `docs/database/schema.dbml` compilando a SQL con `@dbml/cli`. Se corrigió el step `dbml2sql`: `npx --yes @dbml/cli dbml2sql ...` fallaba con "could not determine executable to run" (con npm 10.8.2 el paquete expone 3 bins — `db2dbml`, `dbml2sql`, `sql2dbml` — y npx no puede desambiguar cuál correr sin `-p`); se cambió a `npx --yes -p @dbml/cli dbml2sql ...`
  - Repo vinculado a [github.com/mllagostera/commander-companion](https://github.com/mllagostera/commander-companion), `main` como rama por defecto
  - Los 3 workflows se refactorizaron a patrón `changes` (job con `dorny/paths-filter`) + `needs`/`if` por job, en vez de filtrar por `paths:` a nivel de `on:` — así siempre reportan un check (success/failure/skipped) sin importar qué archivos toque el PR. Necesario porque un *required check* que nunca se dispara (por el filtro de path) deja el PR bloqueado para siempre
  - [x] **Branch protection activado en `main`** vía `gh api PUT .../branches/main/protection`: exige los 8 checks a nivel de job (`gofmt / go vet`, `golangci-lint`, `sqlc generate (sin diffs pendientes)`, `build, test y migraciones contra PostgreSQL`, `hadolint (Dockerfile)`, `lint, unit tests y assembleDebug`, `Lint OpenAPI (Spectral)`, `Validar DBML (compila a SQL)`), rama actualizada (`strict: true`), `enforce_admins: true`, sin force-push ni delete de `main`. No se exige aprobación de PR (proyecto de un solo mantenedor)
  - Pendiente: decidir si se limpia `.github/modernize/java-upgrade` (no es de este proyecto; hoy no se sube al repo porque tiene su propio `.gitignore` con `**/*`)
  - Nota: el job `lint` ya debería pasar en verde — el backlog de golangci-lint que exponía se resolvió (ver ítem siguiente)
- [x] `golangci-lint` ejecutable localmente vía Docker y en **0 issues** en todo el repo — la config `.golangci.yml` (v1) era incompatible con `go 1.25.0` del `go.mod` (ninguna imagen v1.x de golangci-lint, ni compilada desde código fuente, soporta ese toolchain). Se migró la config a formato v2 (`golangci-lint migrate`) y se repinó `Makefile:lint-docker` a `golangci/golangci-lint:v2.12.2` (compilado con go1.26.2). Se corrigió el backlog completo expuesto al arrancar la herramienta (68 issues: `revive` — comentarios doc en símbolos exportados de `users`/`decks`/`statistics`/`sync`/`common`, `godot`, `gosec` — DSN de dev local y log de `port` marcados con `//nolint` justificado, `mnd`, `err113`/`errorlint`, `gocritic` `exitAfterDefer` en `main.go` refactorizado a patrón `run() error`, stutter rename `sync.SyncRequest/Response` → `sync.Request/Response`). Los 2 TODO intencionales de `games`/`game-actions` (lógica diferida a fase de refinamiento) quedan con `//nolint:godox` justificado en vez de eliminarse.
- [x] Wiring real de los `service.go` a sus `Queries` generadas por sqlc — **completo**: `users`, `auth`, `decks`, `games`, `game-actions`, `playgroups` y `statistics` están todos conectados de verdad a Postgres, sin módulos dummy pendientes. `decks` incluye ownership check (`GetDeck`/`DeleteDeck` devuelven 404 si el deck no es del usuario autenticado); `games`/`game-actions` son el motor de partida completo; `playgroups` auto-une al creador y acota listado/detalle a membresía; `statistics` recalcula de verdad al finalizar cada partida (ver Stage 1 y 7 para el detalle de cada uno)
- [ ] Manejo de errores consistente (mapear errores de dominio a códigos HTTP de forma uniforme en todos los módulos)
- [ ] Rate limiting en endpoints de auth
- [ ] Registrar ADRs en `docs/decisions/` a medida que se tomen decisiones técnicas nuevas — práctica iniciada 2026-07-26 (ver ADR-0001 a 0004 en Stage 0), mantenerla para decisiones futuras
- [ ] Limpieza de carpetas vacías/residuales: `docker/`, `scripts/`

---

## Orden de trabajo sugerido

1. ~~**Higiene rápida**: ADRs mínimos de lo ya decidido~~ — hecho (ADR-0001 a 0004). Sigue pendiente decidir destino de `docker/`/`scripts/`.
2. ~~**Auth real** (Stage 1): JWT + bcrypt + middleware~~ — hecho (email/password + Google). Pendiente solo el paso manual externo: crear las credenciales OAuth en Google Cloud Console y setear `GOOGLE_CLIENT_ID`.
3. ~~**Conectar los servicios a la base de datos real** (Transversal): sacar los dummies de `users`, `decks`, `games`, `game-actions`, `playgroups`, `statistics`~~ — hecho, backend 100% wireado.
4. ~~**Estadísticas reales** (Stage 7): recálculo al finalizar partida~~ — hecho.
5. **Completar el contrato OpenAPI** (Stage 3) para que coincida con lo implementado — hecho el wiring de requests/responses de los módulos nuevos; sigue pendiente la paginación cursor-based.
6. ~~**Android: auth real (Stage 4-5)**: Credential Manager + Google, `POST /auth/*`, interceptor de sesión, persistencia con DataStore~~ — hecho 2026-07-27 (`LoginViewModel`, `NetworkModule`, `SessionManager`, `AuthInterceptor`/`AuthAuthenticator`); pendiente el paso manual externo (Web Client ID de Google Cloud) para probar el flujo de Google end-to-end. **Sigue pendiente**: capas de dominio/repositorio en Android (Stage 4, decisión consciente de posponerla) y `CommanderApi.kt` real (decks/games/etc., necesario para wiring real de `GameViewModel` contra el backend en vez de solo Room local) — esta es ahora la mayor brecha del proyecto.
7. **Websocket** (Stage 6) una vez el flujo síncrono funciona de punta a punta — diseño ([ADR-0005](../decisions/0005-websocket-protocol.md)) y servidor (`internal/websocket/`) ya hechos; falta el cliente Android.
8. **Integración Moxfield** (Stage 8) — es la pieza más aislada, puede ir en paralelo o al final.
9. **Tests + CI** — idealmente no se dejan para el final; introducir tests a medida que se reemplazan los stubs en el punto 3 evita tener que rehacerlos después.
