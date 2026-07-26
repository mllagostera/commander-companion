# Commander Companion — Lista de tareas

Checklist operativa de todo el trabajo pendiente, organizada por las **Etapas** definidas en [ROADMAP.md](ROADMAP.md). Este documento es la fuente de verdad del progreso día a día: el ROADMAP explica el *qué* y el *porqué*, este archivo trackea el *estado*.

**Cómo mantenerlo actualizado:**
- Marca `[x]` cuando una tarea esté terminada y funcionalmente completa (no solo "compila").
- Si algo está a medias (scaffolding sin lógica real, stub, dummy data), déjalo en `[ ]` y añade una nota entre paréntesis explicando qué falta.
- Añade tareas nuevas según aparezcan; no borres las completadas, son historial útil.
- Actualiza la fecha de "Última revisión" cada vez que se audite el estado real del código.

**Última revisión:** 2026-07-26 (auditoría inicial + detalle de auth con Google OAuth + generación de slices playgroups/games/game-actions y fix de tooling sqlc/lint + quality gates de GitHub Actions + repo vinculado y branch protection activo en `main`)

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
- [ ] **Módulo `auth` real** — la carpeta `internal/auth/` existe pero está vacía. No hay JWT, no hay login/refresh/logout, no hay middleware de autenticación

### Auth — email/password
- [ ] Hash de contraseñas con bcrypt (`internal/users/service.go` usa un hash dummy: `"hashed_" + password`)
- [ ] `POST /auth/login` real: verificar credenciales, emitir access token (JWT corto) + refresh token
- [ ] `POST /auth/refresh` real: rotar/validar refresh token y emitir nuevo access token
- [ ] `POST /auth/logout` real: invalidar refresh token (tabla/almacén de refresh tokens o revocation list)
- [ ] `GET /auth/me` real: resolver el usuario a partir del JWT del request
- [ ] Middleware Fiber que valida `bearerAuth` y mete el `user_id` en el contexto de la request
- [ ] Aplicar el middleware a todas las rutas protegidas y eliminar los `"dummy-user-id"` hardcodeados (`decks/handler.go` y cualquier otro handler que los use)
- [ ] Definir expiración de tokens y estrategia de firma (secret simétrico HS256 vs. par de claves RS256)

### Auth — Google OAuth (Sign-In)
- [ ] ADR: decisión de añadir Google como proveedor OAuth junto a email/password (alcance: solo login social, no reemplaza al password)
- [ ] Crear credenciales OAuth en Google Cloud Console: un **Web Client ID** (para verificar el `id_token` en el backend) y un **Android Client ID** (para la app, ligado al `applicationId` + SHA-1 de firma)
- [ ] Cambios de esquema en BD (ver Stage 2): `google_id` en `users`, `password_hash` pasa a nullable, unique index en `google_id`
- [ ] Nuevo endpoint `POST /auth/google`: recibe el `id_token` de Google emitido en el cliente, lo verifica contra Google (issuer, audience = Web Client ID, firma) y emite el mismo par access/refresh token que el login normal
- [ ] Verificación del `id_token` en Go (librería `google.golang.org/api/idtoken` o validación manual de JWKS de Google)
- [ ] Lógica de alta/vinculación de cuenta: si el email de Google ya existe con cuenta password, decidir si se vincula automáticamente o se exige confirmación; si no existe, crear usuario nuevo con `google_id` y sin `password_hash`
- [ ] Variable de entorno `GOOGLE_CLIENT_ID` (audience esperada) documentada en `.env.example`
- [ ] Manejo de errores específicos (token expirado, audience inválida, email no verificado por Google)

### Infra / configuración
- [ ] `configs/` poblado con configuración por entorno (hoy está vacío; todo se lee de variables de entorno sueltas en `main.go`)
- [ ] `.env.example` documentando las variables requeridas (`DB_URL`, `PORT`, `JWT_SECRET`, `GOOGLE_CLIENT_ID`, futuras de Moxfield)
- [ ] Revisar `Dockerfile` y `docker-compose.yml` (hoy viven en `backend/`, pero `docker/` en la raíz está vacío — decidir dónde centralizar)
- [x] Limpieza: carpeta residual `backend;C` en la raíz del repo (eliminada, 2026-07-26)

## Stage 2: Base de datos

- [x] Esquema DBML inicial (`docs/database/schema.dbml`)
- [x] Migración inicial coherente con el DBML (`migrations/00001_initial_schema.sql`)
- [ ] Índices explícitos más allá de las PK (p. ej. `decks.moxfield_id`, `game_actions.game_id`, `game_players.game_id` para lecturas frecuentes del timeline)
- [ ] Constraints adicionales (`games.status` como enum/check en vez de `varchar` libre; `action_type` igual)
- [ ] Migración: añadir `users.google_id varchar unique nullable` + índice único, y volver `users.password_hash` nullable (cuentas Google no tienen password) — actualizar también `docs/database/schema.dbml`
- [ ] Constraint/validación a nivel de servicio: un usuario debe tener `password_hash` o `google_id` (no ambos nulos)
- [ ] Diagrama ER visual exportado a `docs/diagrams/`
- [ ] Estrategia de migraciones futuras (naming, cómo versionar cambios de estadísticas pre-calculadas)

## Stage 3: API (contrato OpenAPI)

- [x] Esqueleto OpenAPI 3.1 con los paths principales
- [ ] `requestBody` + schemas completos donde faltan: `/auth/register`, `/auth/login`, `/decks` (POST), `/decks/import/moxfield`, `/games` (POST), `/games/{id}/join`, `/playgroups` (POST), `/playgroups/{id}/members`
- [ ] Documentar `POST /auth/google` en el OpenAPI (request `{ id_token: string }`, respuesta igual a `/auth/login`: tokens + `User`)
- [ ] Schemas de respuesta de estadísticas (`UserStatsResponse`, `DeckStatsResponse`, stats de playgroup) — hoy no están modelados en el YAML
- [ ] Endpoint `/statistics/playgroup/{id}` no tiene módulo/servicio real detrás (está documentado pero no implementado)
- [ ] Paginación cursor-based en listados (`/games`, `/decks`) — mencionada en el ROADMAP pero no existe ni en el spec ni en el código
- [ ] Lint del spec (p. ej. Spectral) y proceso para mantenerlo sincronizado con el código

## Stage 4: Cliente Android (base)

- [x] Proyecto inicializado: Compose, Material 3, Navigation, Hilt, Room, Retrofit, kotlinx.serialization
- [x] Theming base (`Color.kt`, `Theme.kt`, `Type.kt`) y navegación con rutas (`AppNavigation.kt`, `Routes.kt`)
- [x] Pantallas placeholder: `DashboardScreen`, `GameTrackerScreen`, componente `PlayerCard`
- [ ] Pantallas de autenticación (login/registro) — no existen
  - [ ] Dependencia Credential Manager + Google Identity Services (`androidx.credentials`, `androidx.credentials:credentials-play-services-auth`, `com.google.android.libraries.identity.googleid`)
  - [ ] Botón "Continuar con Google" que dispara el flujo de Credential Manager y obtiene el `id_token`
  - [ ] Enviar el `id_token` a `POST /auth/google` y guardar los tokens devueltos igual que en el login normal
  - [ ] Manejo de estado: usuario cancela el picker de cuentas, no tiene cuenta Google configurada en el dispositivo, o el backend rechaza el token
  - [ ] Flujo de "Cerrar sesión" que también limpia el estado de credenciales de Google (`clearCredentialState`)
- [ ] Capa de dominio (`domain/` con use cases e interfaces de repositorio) — no existe, se salta directo de UI a datos
- [ ] Repositorios reales en `data/repository/` — hoy solo hay un `GameDao` + `GameEntity` sueltos, sin repositorio que los use
- [ ] DI completo: `AppModule` solo provee el `Context`; faltan módulos de red (Retrofit/OkHttp), base de datos (Room) y bindings de repositorios
- [ ] `GameViewModel.kt` — el archivo existe pero está **vacío**, sin lógica de negocio
- [ ] `CommanderApi.kt` — solo tiene un `GET /health`; faltan todos los endpoints reales (auth, decks, games, game-actions, statistics)
- [ ] `GameState.kt` — revisar que modele correctamente vida, veneno, energía, experiencia, daño de comandante por oponente

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
  - `docs-ci.yml`: valida las fuentes de verdad — Spectral lint sobre `docs/api/openapi.yaml` (ruleset en `.spectral.yaml`) y `docs/database/schema.dbml` compilando a SQL con `@dbml/cli`
  - Repo vinculado a [github.com/mllagostera/commander-companion](https://github.com/mllagostera/commander-companion), `main` como rama por defecto
  - Los 3 workflows se refactorizaron a patrón `changes` (job con `dorny/paths-filter`) + `needs`/`if` por job, en vez de filtrar por `paths:` a nivel de `on:` — así siempre reportan un check (success/failure/skipped) sin importar qué archivos toque el PR. Necesario porque un *required check* que nunca se dispara (por el filtro de path) deja el PR bloqueado para siempre
  - [x] **Branch protection activado en `main`** vía `gh api PUT .../branches/main/protection`: exige los 8 checks a nivel de job (`gofmt / go vet`, `golangci-lint`, `sqlc generate (sin diffs pendientes)`, `build, test y migraciones contra PostgreSQL`, `hadolint (Dockerfile)`, `lint, unit tests y assembleDebug`, `Lint OpenAPI (Spectral)`, `Validar DBML (compila a SQL)`), rama actualizada (`strict: true`), `enforce_admins: true`, sin force-push ni delete de `main`. No se exige aprobación de PR (proyecto de un solo mantenedor)
  - Pendiente: decidir si se limpia `.github/modernize/java-upgrade` (no es de este proyecto; hoy no se sube al repo porque tiene su propio `.gitignore` con `**/*`)
  - Nota: el job `lint` ya debería pasar en verde — el backlog de golangci-lint que exponía se resolvió (ver ítem siguiente)
- [x] `golangci-lint` ejecutable localmente vía Docker y en **0 issues** en todo el repo — la config `.golangci.yml` (v1) era incompatible con `go 1.25.0` del `go.mod` (ninguna imagen v1.x de golangci-lint, ni compilada desde código fuente, soporta ese toolchain). Se migró la config a formato v2 (`golangci-lint migrate`) y se repinó `Makefile:lint-docker` a `golangci/golangci-lint:v2.12.2` (compilado con go1.26.2). Se corrigió el backlog completo expuesto al arrancar la herramienta (68 issues: `revive` — comentarios doc en símbolos exportados de `users`/`decks`/`statistics`/`sync`/`common`, `godot`, `gosec` — DSN de dev local y log de `port` marcados con `//nolint` justificado, `mnd`, `err113`/`errorlint`, `gocritic` `exitAfterDefer` en `main.go` refactorizado a patrón `run() error`, stutter rename `sync.SyncRequest/Response` → `sync.Request/Response`). Los 2 TODO intencionales de `games`/`game-actions` (lógica diferida a fase de refinamiento) quedan con `//nolint:godox` justificado en vez de eliminarse.
- [ ] Wiring real de los `service.go` a sus `Queries` generadas por sqlc — hoy **todos** los servicios (`users`, `decks`, `games`, `game-actions`, `playgroups`, `statistics`) devuelven datos dummy/hardcodeados en vez de usar el `repo` que reciben inyectado
- [ ] Manejo de errores consistente (mapear errores de dominio a códigos HTTP de forma uniforme en todos los módulos)
- [ ] Rate limiting en endpoints de auth
- [ ] Registrar ADRs en `docs/decisions/` a medida que se tomen decisiones técnicas nuevas
- [ ] Limpieza de carpetas vacías/residuales: `docker/`, `scripts/`

---

## Orden de trabajo sugerido

1. **Higiene rápida**: limpiar `backend;C`, decidir destino de `docker/`/`scripts/`, ADRs mínimos de lo ya decidido.
2. **Auth real** (Stage 1): JWT + bcrypt + middleware — todo lo demás depende de saber quién es el usuario (hoy se usa `"dummy-user-id"` en todos lados).
3. **Conectar los servicios a la base de datos real** (Transversal): sacar los dummies de `users`, `decks`, `playgroups`, `games`, `game-actions` — es la mayor brecha entre "parece terminado" y "funciona".
4. **Estadísticas reales** (Stage 7): recálculo al finalizar partida.
5. **Completar el contrato OpenAPI** (Stage 3) para que coincida con lo implementado, incluida paginación.
6. **Android: capas de dominio/datos + auth + wiring real de `GameViewModel`** (Stage 4-5).
7. **Websocket** (Stage 6) una vez el flujo síncrono funciona de punta a punta.
8. **Integración Moxfield** (Stage 8) — es la pieza más aislada, puede ir en paralelo o al final.
9. **Tests + CI** — idealmente no se dejan para el final; introducir tests a medida que se reemplazan los stubs en el punto 3 evita tener que rehacerlos después.
