# Commander Companion — Lista de tareas

Checklist operativa de todo el trabajo pendiente, organizada por las **Etapas** definidas en [ROADMAP.md](ROADMAP.md). Este documento es la fuente de verdad del progreso día a día: el ROADMAP explica el *qué* y el *porqué*, este archivo trackea el *estado*.

**Cómo mantenerlo actualizado:**
- Marca `[x]` cuando una tarea esté terminada y funcionalmente completa (no solo "compila").
- Si algo está a medias (scaffolding sin lógica real, stub, dummy data), déjalo en `[ ]` y añade una nota entre paréntesis explicando qué falta.
- Añade tareas nuevas según aparezcan; no borres las completadas, son historial útil.
- Actualiza la fecha de "Última revisión" cada vez que se audite el estado real del código.

**Última revisión:** 2026-07-26 (auditoría inicial + detalle de auth con Google OAuth + generación de slices playgroups/games/game-actions y fix de tooling sqlc/lint + quality gates de GitHub Actions + repo vinculado y branch protection activo en `main` + implementación real de auth email/password + Google + life tracker local de Android completo con persistencia en Room)

---

## Stage 0: Definición funcional

- [x] Roadmap general (`docs/roadmap/ROADMAP.md`)
- [x] Documento de arquitectura y principios (`docs/architecture/ARCHITECTURE.md`)
- [ ] Casos de uso detallados (flujos de usuario paso a paso: crear partida, unirse, trackear vida, finalizar, ver stats)
- [ ] Wireframes de las pantallas Android (`docs/ux/` está vacío)
- [ ] Diagramas adicionales de flujo/estado (`docs/diagrams/` está vacío; solo existen los 2 diagramas de arquitectura embebidos en el ROADMAP)
- [ ] ADRs iniciales (`docs/decisions/` está vacío) — al menos: elección de Go+Fiber, PostgreSQL, sqlc+goose, Android nativo (Kotlin/Compose) vs. cross-platform, monolito modular vs. microservicios, email/password + Google OAuth como métodos de login

## Stage 1: Backend (base del proyecto)

- [x] Proyecto Go inicializado (`go.mod`, `cmd/api/main.go`, Fiber)
- [x] Estructura modular en `internal/` (auth, users, decks, games, game-actions, playgroups, statistics, sync, websocket, common)
- [x] Slices `playgroups`, `games` y `game-actions` generados (query.sql + dto/service/handler esqueleto, CRUD y flujo create→join→leave→start→finish→actions→timeline) y registrados en `cmd/api/main.go`. Lógica interna sigue siendo dummy (ver nota de wiring más abajo).
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
- [x] Expiración y firma de tokens: **HS256 simétrico** con `JWT_SECRET` (env var, default inseguro solo de dev con `//nolint:gosec` justificado) — decisión: RS256 no aporta valor en un monolito sin verificadores externos del token; TTLs configurables por env (`ACCESS_TOKEN_TTL` default 15m, `REFRESH_TOKEN_TTL` default 720h/30d)

### Auth — Google OAuth (Sign-In)
- [ ] ADR: decisión de añadir Google como proveedor OAuth junto a email/password (alcance: solo login social, no reemplaza al password) — la decisión ya está tomada e implementada, falta redactar el ADR formal en `docs/decisions/`
- [ ] **Crear credenciales OAuth en Google Cloud Console** (Web Client ID + Android Client ID) — paso manual externo, requiere acceso a una cuenta de Google Cloud del proyecto; el backend ya está listo para recibirlo vía `GOOGLE_CLIENT_ID`. Sin configurar, `POST /auth/google` responde `501 Not Implemented` en vez de crashear
- [x] Cambios de esquema en BD (ver Stage 2): `google_id` en `users`, `password_hash` nullable, `CHECK (password_hash IS NOT NULL OR google_id IS NOT NULL)` (migración `00002_auth.sql`, probada up/down/up contra Postgres real)
- [x] `POST /auth/google`: recibe `id_token`, lo verifica contra Google (issuer + audience = `GOOGLE_CLIENT_ID` + firma vía JWKS) y emite el mismo par access/refresh token que `/auth/login`
- [x] Verificación del `id_token` en Go — se usó `github.com/coreos/go-oidc/v3` en vez de `google.golang.org/api/idtoken`: mismo resultado (discovery + JWKS + validación de issuer/audience/firma) con una huella de dependencias mucho más liviana (evita arrastrar todo `google.golang.org/api` + gRPC + OpenTelemetry solo para verificar un token). El discovery document de Google se resuelve de forma perezosa en el primer login con Google, no al arrancar el servidor
- [x] Lógica de alta/vinculación de cuenta (`users.FindOrCreateGoogleUser`): busca por `google_id` → si no existe, busca por email y **vincula automáticamente** si Google confirma `email_verified` → si tampoco existe, crea usuario nuevo sin password (username derivado del local-part del email, con reintento sufijado ante colisión)
- [x] Variable de entorno `GOOGLE_CLIENT_ID` documentada en `.env.example`
- [x] Manejo de errores específicos: token inválido/expirado → 401, `email_verified: false` → 400, `GOOGLE_CLIENT_ID` no configurado → 501

### Infra / configuración
- [ ] `configs/` poblado con configuración por entorno (hoy está vacío; todo se lee de variables de entorno sueltas en `main.go`)
- [x] `.env.example` documentando las variables requeridas (`DB_URL`, `PORT`, `JWT_SECRET`, `ACCESS_TOKEN_TTL`, `REFRESH_TOKEN_TTL`, `GOOGLE_CLIENT_ID`); `docker-compose.yml` actualizado con las mismas (y corregido: usaba `DB_HOST`/`DB_USER`/etc. sueltas que `main.go` nunca leyó — ahora usa `DB_URL` real)
- [ ] Revisar `Dockerfile` y `docker-compose.yml` (hoy viven en `backend/`, pero `docker/` en la raíz está vacío — decidir dónde centralizar)
- [x] Limpieza: carpeta residual `backend;C` en la raíz del repo (eliminada, 2026-07-26)

## Stage 2: Base de datos

- [x] Esquema DBML inicial (`docs/database/schema.dbml`)
- [x] Migración inicial coherente con el DBML (`migrations/00001_initial_schema.sql`)
- [ ] Índices explícitos más allá de las PK (p. ej. `decks.moxfield_id`, `game_actions.game_id`, `game_players.game_id` para lecturas frecuentes del timeline)
- [ ] Constraints adicionales (`games.status` como enum/check en vez de `varchar` libre; `action_type` igual)
- [x] Migración: `users.google_id varchar unique nullable` + `users.password_hash` nullable (`migrations/00002_auth.sql`, `docs/database/schema.dbml` actualizado) + tabla `refresh_tokens` (`token_hash` único, `expires_at`, `revoked_at`)
- [x] Constraint a nivel de BD: `CHECK (password_hash IS NOT NULL OR google_id IS NOT NULL)` en `users`
- [ ] Diagrama ER visual exportado a `docs/diagrams/`
- [ ] Estrategia de migraciones futuras (naming, cómo versionar cambios de estadísticas pre-calculadas)

## Stage 3: API (contrato OpenAPI)

- [x] Esqueleto OpenAPI 3.1 con los paths principales
- [x] `requestBody` + schemas completos de auth: `RegisterRequest`, `LoginRequest`, `GoogleLoginRequest`, `RefreshRequest`, `LogoutRequest`, `TokenResponse` (con sus códigos de error: 401/400/409/501 según corresponda)
- [ ] `requestBody` + schemas completos donde faltan: `/decks` (POST), `/decks/import/moxfield`, `/games` (POST), `/games/{id}/join`, `/playgroups` (POST), `/playgroups/{id}/members`
- [x] Documentar `POST /auth/google` en el OpenAPI (request `{ id_token: string }`, respuesta igual a `/auth/login`: `TokenResponse`)
- [ ] Schemas de respuesta de estadísticas (`UserStatsResponse`, `DeckStatsResponse`, stats de playgroup) — hoy no están modelados en el YAML
- [ ] Endpoint `/statistics/playgroup/{id}` no tiene módulo/servicio real detrás (está documentado pero no implementado)
- [ ] Paginación cursor-based en listados (`/games`, `/decks`) — mencionada en el ROADMAP pero no existe ni en el spec ni en el código
- [x] Lint del spec con Spectral (`.spectral.yaml`, corre en `docs-ci.yml`) — 0 errores; quedan 93 warnings preexistentes en todo el spec (`operationId`/`tags`/`description` faltantes en casi todos los paths), no bloqueantes, pendientes como deuda de documentación

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
- [x] Pantallas placeholder: `DashboardScreen` (ahora con navegación real a setup/historial), componente `PlayerCard` (ahora con lógica real de daño de comandante)
- [ ] Pantallas de autenticación (login/registro) — no existen
  - [ ] Dependencia Credential Manager + Google Identity Services (`androidx.credentials`, `androidx.credentials:credentials-play-services-auth`, `com.google.android.libraries.identity.googleid`)
  - [ ] Botón "Continuar con Google" que dispara el flujo de Credential Manager y obtiene el `id_token`
  - [ ] Enviar el `id_token` a `POST /auth/google` y guardar los tokens devueltos igual que en el login normal
  - [ ] Manejo de estado: usuario cancela el picker de cuentas, no tiene cuenta Google configurada en el dispositivo, o el backend rechaza el token
  - [ ] Flujo de "Cerrar sesión" que también limpia el estado de credenciales de Google (`clearCredentialState`)
- [ ] Capa de dominio (`domain/` con use cases e interfaces de repositorio) — no existe, se salta directo de UI a datos (el life tracker inyecta `GameDao` directo en el `ViewModel`, sin capa de repositorio; aceptable para el alcance actual, revisar si se justifica al integrar el backend)
- [ ] Repositorios reales en `data/repository/` — no existe todavía; hoy `GameDao` se inyecta directo en `GameViewModel`/`HistoryViewModel`
- [x] DI de Room: `DatabaseModule` (Hilt) provee `CommanderDatabase` y `GameDao`. Sigue pendiente el módulo de red (Retrofit/OkHttp) — `AppModule` solo provee `Context`
- [x] `GameViewModel.kt` — ya no está vacío: vida, turno, daño de comandante y persistencia del resultado en Room (ver nota del life tracker más arriba)
- [ ] `CommanderApi.kt` — solo tiene un `GET /health`; faltan todos los endpoints reales (auth, decks, games, game-actions, statistics) — necesario para conectar el life tracker a partidas reales del backend en vez de jugarse 100% local
- [x] `GameState.kt` — modela vida, turno y daño de comandante por oponente para N jugadores (2-6). Veneno/energía/experiencia siguen sin modelar (no forman parte del alcance de esta pasada)

## Stage 5: Integración Android ↔ Backend

- [ ] Interceptor de autenticación (adjuntar JWT en Retrofit, manejar 401 → refresh/logout), válido tanto para sesiones de password como de Google
- [ ] Persistencia de sesión con DataStore (está en el stack previsto por el ROADMAP pero no añadido aún a las dependencias)
- [ ] Flujo end-to-end: registro/login (password o Google) → dashboard → crear/unirse a partida → tracker de vida en tiempo real → finalizar partida → ver resultado
- [ ] Room como caché offline-first (partidas vistas, decks propios) con estrategia de sincronización

## Stage 6: Sincronización (Websocket)

- [ ] Diseño del protocolo de mensajes (qué eventos de `game_actions` se retransmiten en vivo y su formato)
- [ ] Implementación del servidor en `internal/websocket/` (carpeta vacía hoy)
- [ ] Cliente websocket en Android (conexión, reconexión con backoff, aplicar eventos entrantes al `GameState`)

## Stage 7: Estadísticas

- [ ] Lógica real de recálculo al finalizar partida (`games/service.go: FinishGame` solo tiene un comentario `// TODO: lanzar recálculo de estadísticas`)
- [ ] Queries/triggers de agregación para `user_statistics_summary` y `deck_statistics_summary` (las tablas existen, pero nada las escribe)
- [ ] Implementar `internal/statistics` service real (hoy `GetUserStats`/`GetDeckStats` devuelven objetos vacíos, no consultan la BD)
- [ ] Servicio y endpoint de estadísticas por playgroup (no existe aún)
- [ ] UI de estadísticas en Android (no hay pantalla)

## Stage 8: Importación Moxfield

- [ ] Investigar la API pública/no oficial de Moxfield a integrar
- [ ] Cliente HTTP hacia Moxfield en el backend
- [ ] Reemplazar el stub de `internal/sync/service.go` (hoy devuelve `"queued"`/`"in_progress"` sin hacer nada real) por lógica de sincronización real
- [ ] Endpoint `/decks/import/moxfield` en `internal/decks` (documentado en OpenAPI, no implementado en el handler)
- [ ] Manejo de errores, rate limiting y reintentos ante la API externa

## Transversal (calidad, infraestructura, seguridad)

- [ ] Tests unitarios de backend (0 tests en todo el repo actualmente)
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
- [ ] Wiring real de los `service.go` a sus `Queries` generadas por sqlc — `users` y `auth` ya están conectados de verdad (auth completo, probado end-to-end contra Postgres: register → login → me → decks protegidos → refresh con rotación → logout, y 401 al reusar un refresh token ya rotado). Siguen dummy: `decks`, `games`, `game-actions`, `playgroups`, `statistics` (aunque ya reciben el `user_id` real del JWT donde corresponde, p. ej. `decks.CreateDeck`)
- [ ] Manejo de errores consistente (mapear errores de dominio a códigos HTTP de forma uniforme en todos los módulos)
- [ ] Rate limiting en endpoints de auth
- [ ] Registrar ADRs en `docs/decisions/` a medida que se tomen decisiones técnicas nuevas
- [ ] Limpieza de carpetas vacías/residuales: `docker/`, `scripts/`

---

## Orden de trabajo sugerido

1. **Higiene rápida**: limpiar `backend;C`, decidir destino de `docker/`/`scripts/`, ADRs mínimos de lo ya decidido.
2. ~~**Auth real** (Stage 1): JWT + bcrypt + middleware~~ — hecho (email/password + Google). Pendiente solo el paso manual externo: crear las credenciales OAuth en Google Cloud Console y setear `GOOGLE_CLIENT_ID`.
3. **Conectar los servicios a la base de datos real** (Transversal): sacar los dummies de `users`, `decks`, `playgroups`, `games`, `game-actions` — es la mayor brecha entre "parece terminado" y "funciona".
4. **Estadísticas reales** (Stage 7): recálculo al finalizar partida.
5. **Completar el contrato OpenAPI** (Stage 3) para que coincida con lo implementado, incluida paginación.
6. **Android: capas de dominio/datos + auth + wiring real de `GameViewModel`** (Stage 4-5).
7. **Websocket** (Stage 6) una vez el flujo síncrono funciona de punta a punta.
8. **Integración Moxfield** (Stage 8) — es la pieza más aislada, puede ir en paralelo o al final.
9. **Tests + CI** — idealmente no se dejan para el final; introducir tests a medida que se reemplazan los stubs en el punto 3 evita tener que rehacerlos después.
