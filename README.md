# Commander Companion

Aplicación para partidas de Magic: The Gathering — formato Commander. Backend en Go + cliente Android nativo (Kotlin/Compose), enfocados en velocidad durante la partida, buena UX, estadísticas y sincronización entre jugadores.

Este documento es el punto de entrada para cualquier persona o IA que empiece a trabajar en el repo. Léelo antes de tocar código.

---

## 1. Filosofía del proyecto

La prioridad NO es tener cientos de funcionalidades. La prioridad es que cualquier acción durante una partida pueda hacerse en menos de dos segundos. Todo gira en torno a tres pilares: **simplicidad, velocidad, datos**. Ver detalle en [docs/roadmap/ROADMAP.md](docs/roadmap/ROADMAP.md).

## 2. Estructura del repo

```
commander-companion/
├── backend/              # API en Go (Fiber + PostgreSQL + sqlc + goose)
│   ├── cmd/api/          # entrypoint (main.go)
│   ├── internal/         # módulos: auth, users, decks, games, game-actions,
│   │                     #   playgroups, statistics, sync, websocket, common
│   ├── migrations/       # migraciones goose
│   ├── configs/          # configuración por entorno (aún vacío)
│   ├── sqlc.yaml         # config de generación de repositorios
│   └── Makefile          # build, run, test, lint, migrate, generate-sql
├── android/              # Cliente Android nativo (Kotlin, Compose, Hilt, Room, Retrofit)
│   └── app/src/main/java/com/commandercompanion/
│       ├── data/         # remote (Retrofit) y local (Room)
│       ├── domain/       # casos de uso e interfaces de repos (por crear)
│       ├── presentation/ # screens, viewmodels, navegación, tema
│       └── core/         # DI (Hilt) y utilidades
├── docs/
│   ├── roadmap/          # ROADMAP.md (visión/etapas) y TASKS.md (checklist de progreso)
│   ├── architecture/     # ARCHITECTURE.md (principios y patrones)
│   ├── database/         # schema.dbml (fuente de verdad del modelo de datos)
│   ├── api/               # openapi.yaml (fuente de verdad del contrato REST)
│   ├── decisions/        # ADRs (decisiones técnicas)
│   ├── diagrams/         # diagramas Mermaid adicionales
│   ├── ux/                # wireframes
│   └── frontend/          # notas específicas de cliente
├── docker/               # (vacío por ahora; docker-compose vive en backend/)
└── scripts/              # (vacío por ahora)
```

## 3. Las 4 fuentes de verdad

Antes de asumir cómo funciona algo, consulta el documento correspondiente — no el código de otro módulo por analogía, y no memoria de conversaciones previas:

| Fuente | Ubicación | Qué define |
|---|---|---|
| DBML | `docs/database/schema.dbml` | Esquema de BD, tipos, relaciones |
| OpenAPI 3.1 | `docs/api/openapi.yaml` | Contrato único backend ↔ Android |
| Mermaid | `docs/architecture/`, `docs/diagrams/` | Arquitectura, flujos, estados |
| ADR | `docs/decisions/` | Decisiones técnicas y su porqué |

Regla: si vas a cambiar cómo se comunican backend y Android, edita primero `openapi.yaml`. Si cambias el modelo de datos, edita primero `schema.dbml` y luego crea la migración goose.

## 4. Cómo proceder (para agentes de IA)

1. **Lee `docs/roadmap/TASKS.md` primero.** Es la lista de tareas pendientes organizada por etapa, con el estado real auditado contra el código (no contra lo que "debería" existir).
2. **No confíes en que algo está terminado solo porque el archivo existe.** Gran parte del backend es scaffolding: los `service.go` de varios módulos devuelven datos dummy en vez de usar el repositorio inyectado. Verifica leyendo el código antes de asumir que una función hace lo que su nombre sugiere.
3. **Sigue el patrón de capas ya establecido:**
   - Backend: `Handler` (transporte HTTP/WS) → `Service` (lógica de negocio, sin dependencias de infraestructura) → `Repository` (sqlc, acceso a datos). Ver [docs/architecture/ARCHITECTURE.md](docs/architecture/ARCHITECTURE.md).
   - Android: Clean Architecture + MVVM + UDF — `presentation/` (Compose + ViewModel) → `domain/` (casos de uso) → `data/` (Retrofit/Room).
4. **Respeta el orden de trabajo sugerido** al final de `TASKS.md`, salvo indicación explícita del usuario de priorizar otra cosa.
5. **Si tomas una decisión técnica no trivial** (elegir una librería, un patrón, una estructura de datos que no estaba ya definida), regístrala como ADR en `docs/decisions/`.
6. **Si el trabajo toca el contrato API o el esquema de BD**, actualiza `openapi.yaml` / `schema.dbml` (y la migración correspondiente) como parte del mismo cambio, no después.

## 5. Cómo actualizar `docs/roadmap/TASKS.md`

`TASKS.md` es un documento vivo — se actualiza en el mismo cambio que resuelve la tarea, no en un paso aparte.

- Marca `- [x]` **solo** cuando la tarea esté funcionalmente completa (compila y funciona), no cuando el archivo simplemente exista o compile con un stub.
- Si una tarea queda a medias, déjala en `- [ ]` y añade una nota entre paréntesis explicando qué falta exactamente (ver ejemplos ya presentes en el documento, p. ej. "GameViewModel.kt — el archivo existe pero está vacío").
- Si durante el trabajo descubres una tarea nueva que no estaba listada (una dependencia, un caso borde, deuda técnica), añádela en la sección de la etapa correspondiente en vez de dejarla suelta en la conversación.
- **No borres tareas completadas** — son el historial de progreso del proyecto. Si una tarea deja de tener sentido (se descarta el enfoque), táchala explicando por qué en vez de eliminarla.
- Actualiza la línea `**Última revisión:**` al final de la sesión de trabajo con la fecha del día.
- Si una tarea completada cambia una de las 4 fuentes de verdad, confirma que ese archivo (`schema.dbml`, `openapi.yaml`, diagrama, ADR) quedó actualizado antes de marcarla como hecha.

## 6. Quality gates (GitHub Actions)

En `.github/workflows/` hay tres pipelines que corren en cada push/PR (filtrados por path, solo se disparan si tocas la carpeta relevante):

- **`backend-ci.yml`**: gofmt + `go vet`, `golangci-lint`, verifica que `sqlc generate` no deje diffs sin commitear, build + `go test -race` + aplica las migraciones goose contra un Postgres real del job, y `hadolint` sobre el `Dockerfile`.
- **`android-ci.yml`**: Android Lint, tests unitarios (`testDebugUnitTest`), `assembleDebug`.
- **`docs-ci.yml`**: valida que las fuentes de verdad sigan siendo válidas — Spectral lint sobre `openapi.yaml` y `schema.dbml` compilando a SQL.

Antes de dar una tarea por terminada, estos gates deben pasar en local (o al menos no introducir issues nuevos) para lo que toques: `make lint` / `make test` en backend, `./gradlew lintDebug testDebugUnitTest` en Android. Requieren que el repo esté conectado a GitHub para ejecutarse; localmente son solo los comandos del Makefile / Gradle.

## 7. Comandos útiles

**Backend** (`cd backend`):
```
make run                 # levantar la API localmente
make test                # go test -race ./...
make lint                # golangci-lint
make generate-sql        # regenerar repos con sqlc tras editar query.sql
make migrate-up          # aplicar migraciones goose
docker-compose up        # API + PostgreSQL en contenedores
```

**Android** (`cd android`):
```
./gradlew assembleDebug  # build
./gradlew test           # tests unitarios
./gradlew connectedAndroidTest  # tests instrumentados
```

## 8. Documentos relacionados

- [docs/roadmap/ROADMAP.md](docs/roadmap/ROADMAP.md) — visión, filosofía, etapas de alto nivel.
- [docs/roadmap/TASKS.md](docs/roadmap/TASKS.md) — checklist detallado y estado real, se actualiza continuamente.
- [docs/architecture/ARCHITECTURE.md](docs/architecture/ARCHITECTURE.md) — principios de diseño y patrones de capas.
