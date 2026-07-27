# ADR-0008: sqlc para acceso a datos tipado + goose para migraciones

**Estado:** Aceptada e implementada — **decisión heredada, contexto
reconstruido** (ver nota de método en ADR-0006 y ADR-0007; redactado
retroactivamente el 2026-07-27 a partir de `backend/sqlc.yaml`,
`backend/migrations/`, y los ocho módulos de `internal/` que usan código
generado por sqlc).

## Contexto

Con Go + PostgreSQL ya elegidos (ADR-0006, ADR-0007), quedaban dos
decisiones de tooling con impacto directo en cómo se escribe cada módulo de
`internal/`: cómo llegar de SQL a código Go tipado (ORM completo vs. query
builder vs. generación de código a partir de SQL crudo), y cómo versionar y
aplicar cambios de esquema de forma reproducible en local/CI/producción.

## Decisión

- **sqlc** (`sqlc.yaml`, `sql_package: "pgx/v5"`) genera el código de
  acceso a datos de los seis módulos con queries propias (`users`, `auth`,
  `decks`, `playgroups`, `games`, `game-actions`, `statistics`) a partir de
  SQL escrito a mano en cada `internal/<módulo>/query.sql`, produciendo
  structs y una interfaz `Querier` (`emit_interface: true`) por módulo,
  directamente compatibles con `pgxpool.Pool` (`emit_json_tags: true` para
  serializar directo a DTOs de respuesta).
- **goose** gestiona las migraciones versionadas
  (`migrations/00001_initial_schema.sql`, `00002_auth.sql`), aplicadas en CI
  contra un Postgres real (`backend-ci.yml`) como parte del gate de build.

## Alternativas consideradas

- **ORM completo (GORM, ent)**: ofrece más "magia" (relaciones navegables,
  migraciones autogeneradas desde structs), pero a costa de SQL implícito
  difícil de predecir/optimizar y de una capa de abstracción adicional sobre
  el driver. Se descartó a favor de escribir el SQL a mano y generar solo el
  *binding* tipado — coherente con la filosofía del ROADMAP de "DBML como
  fuente de verdad del esquema" en vez de que el esquema se derive del
  código Go.
- **Query builder dinámico (squirrel, goqu)**: intermedio entre ORM y SQL
  crudo, pero no da tipado estático de las columnas de retorno sin escribir
  igual el `Scan` a mano — sqlc da esa seguridad de tipos generando el
  binding directamente desde el SQL real y el esquema real, detectando en
  tiempo de generación (`sqlc generate`) columnas que no existen o tipos que
  no calzan, antes de llegar a runtime.
- **Migraciones manuales sin herramienta (scripts sueltos)**: se descartó
  por falta de trazabilidad de qué migró y cuándo en cada entorno; goose
  resuelve esto con su tabla de versión y comandos `up`/`down` simétricos
  (usados explícitamente en CI: "migraciones goose contra Postgres real").
- **Otra herramienta de migraciones (golang-migrate, Atlas)**: golang-migrate
  es la alternativa más cercana en popularidad; se optó por goose
  probablemente por su sintaxis de migración como SQL plano con anotaciones
  `-- +goose Up`/`-- +goose Down` en el mismo archivo (más legible en review
  que archivos `.up.sql`/`.down.sql` separados de golang-migrate).

## Consecuencias

- Cambiar el esquema de una tabla requiere tocar en orden: `docs/database/
  schema.dbml` (fuente de verdad documental) → una migración goose nueva →
  el `query.sql` del módulo afectado si cambian columnas usadas → `sqlc
  generate` para regenerar el binding. Saltarse `sqlc generate` deja código
  Go desincronizado del esquema real; por eso `backend-ci.yml` corre `sqlc
  generate` y falla el build si detecta diffs pendientes.
- La versión de la imagen/binario de `sqlc` importa: el propio historial de
  `TASKS.md` registra que hubo que fijar `sql_package: "pgx/v5"` en
  `sqlc.yaml` porque el generador por defecto producía un `DBTX`
  incompatible con `pgxpool.Pool` — un recordatorio de que sqlc y el driver
  elegido (ADR-0007) deben mantenerse alineados en cada actualización.
- Cada módulo nuevo con acceso a datos propio repite el mismo patrón:
  entrada en `sqlc.yaml` + `query.sql` + `sqlc generate` — es mecánico, pero
  es trabajo manual por módulo (no hay generación automática de CRUD
  completo como en un ORM con scaffolding).
- Índices adicionales más allá de las PK (mencionados como pendientes en
  `TASKS.md`, Stage 2) y constraints como `games.status`/`action_type` como
  enum en vez de `varchar` libre quedan fuera de lo que sqlc/goose resuelven
  automáticamente — siguen siendo trabajo de diseño de esquema manual.

## Referencias

- `backend/sqlc.yaml`
- `backend/migrations/00001_initial_schema.sql`, `00002_auth.sql`
- `docs/roadmap/TASKS.md`, Stage 1 ("se corrigió `sqlc.yaml`...") y
  Transversal (CI de `backend-ci.yml`)
