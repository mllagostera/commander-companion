# ADR-0011: Estrategia de migraciones (naming) y recompute de estadísticas pre-calculadas

**Estado:** Aceptada (2026-07-27) — formaliza una convención que ya se
seguía de facto desde `migrations/00001_initial_schema.sql`, y propone (sin
implementar todavía) un mecanismo de backfill que hoy no existe.

## Contexto

El proyecto ya tiene 6 migraciones (`00001` a `00006`, ver ADR-0008 para la
elección de goose+sqlc) y dos tablas de estadísticas pre-calculadas
(`user_statistics_summary`, `deck_statistics_summary`) que se actualizan de
forma incremental cada vez que termina una partida
(`internal/statistics/service.go: RecalculateForGame`, ver Stage 1 y 7 de
`docs/roadmap/TASKS.md`). Dos huecos quedaban sin documentar:

1. **Naming de migraciones**: el patrón usado (`%05d_slug_en_snake_case.sql`)
   nunca se escribió en ningún lado — cualquiera que agregue una migración
   nueva tiene que inferirlo leyendo el directorio.
2. **Recompute de estadísticas**: si la fórmula de agregación de
   `RecalculateForGame` cambia (por ejemplo, para dejar de comportarse igual
   `CombatDamage`/`CommanderDamage`, ver la limitación conocida en
   `TASKS.md:70`), no existe ningún mecanismo para volver a calcular las filas
   de resumen de partidas ya finalizadas. `docs/database/schema.dbml` (línea
   ~102) ya deja una nota reconociendo esta necesidad, sin proponer cómo.

## Decisión

### 1. Naming de migraciones

Se formaliza por escrito la convención ya usada de facto en las 6
migraciones existentes:

- **Nombre de archivo**: `%05d_slug_en_snake_case.sql` (5 dígitos con ceros a
  la izquierda, guion bajo, descripción corta en snake_case). Ejemplos reales:
  `00002_auth.sql`, `00004_status_constraints.sql`,
  `00006_deck_image_url.sql`.
- **Secuencia única y plana**: todas las migraciones comparten una sola
  numeración correlativa en `backend/migrations/`, sin prefijo ni
  sub-carpeta por módulo, aunque una migración típicamente afecte a un solo
  módulo de `internal/`. Facilita saber en qué orden se aplicaron sin tener
  que mirar varias carpetas.
- **Estructura obligatoria**: cada migración envuelve su `-- +goose Up` y
  `-- +goose Down` en su propio bloque `-- +goose StatementBegin` /
  `-- +goose StatementEnd` (aunque sea una sola sentencia), y `Down` revierte
  `Up` en el orden exactamente inverso (LIFO para migraciones con más de una
  sentencia — ver `00001_initial_schema.sql`, que dropea las 9 tablas en
  orden inverso al de creación por las FKs).
- **Comentarios que citan el código real**: cuando una migración impone algo
  que el código Go ya valida en runtime (un `CHECK`, un índice pensado para
  una query concreta), el comentario en el `.sql` referencia el archivo/
  función Go correspondiente (ver `00004_status_constraints.sql`, que cita
  `internal/games/service.go` e `internal/game-actions/service.go`).
- **Verificación antes de mergear**: se corre `up` → `down` → `up` contra un
  Postgres real en local antes de abrir el PR (ya era la práctica seguida,
  documentada suelta en el historial de `TASKS.md`, nunca en un solo lugar).
  `backend-ci.yml` solo corre `up` contra el servicio de Postgres de CI — el
  ciclo `down`→`up` sigue siendo una verificación manual, no automatizada.
- **Cambiar el esquema sigue el orden ya establecido en ADR-0008**:
  `docs/database/schema.dbml` → migración goose → `query.sql` del módulo
  afectado si cambian columnas usadas → `sqlc generate`.

### 2. Recompute de estadísticas pre-calculadas

Hoy no existe ningún mecanismo de backfill (confirmado: no hay CLI, endpoint
ni script — la única forma de tocar `user_statistics_summary`/
`deck_statistics_summary` es `RecalculateForGame`, llamada una única vez por
partida desde `games.FinishGame`). Se propone, para cuando haga falta
re-derivar estadísticas históricas:

- Un comando one-off nuevo, `backend/cmd/recalculate-stats/main.go`, que:
  1. `TRUNCATE` las dos tablas de resumen.
  2. Recorra todas las partidas con `status = 'finished'` **en orden
     cronológico** (`created_at` o `id`, da igual mientras sea un orden
     total y determinístico).
  3. Llame a `statistics.RecalculateForGame(gameID)` una vez por partida.
- **Invariante que el script debe respetar**: los upserts de
  `RecalculateForGame` son incrementales (`ON CONFLICT DO UPDATE SET x = x +
  EXCLUDED.x`), no reemplazos — llamarlo dos veces para la misma partida
  duplica sus contribuciones. Hoy esto no es un bug activo porque la máquina
  de estados de `games` no permite finalizar la misma partida dos veces, pero
  cualquier futuro script de backfill **tiene que garantizar que cada
  partida se procese exactamente una vez** (de ahí el `TRUNCATE` inicial: es
  más simple recalcular todo desde cero que intentar un backfill parcial
  idempotente).
- No se implementa este comando en esta pasada — es una propuesta para
  cuando la fórmula de agregación cambie de verdad y haga falta re-derivar
  historial; hasta entonces, documentarlo alcanza.

## Alternativas consideradas

- **Migraciones con timestamp en vez de secuencia** (`20260727120000_x.sql`,
  patrón común en Rails/Django): evita colisiones de número si dos ramas
  crean una migración en paralelo, pero el proyecto ya tiene 6 migraciones
  con el esquema secuencial y es un solo mantenedor (ADR-0010) — el riesgo de
  colisión que resuelve el timestamp no aplica hoy. Cambiar de esquema a
  mitad de camino generaría más confusión que el problema que evita.
- **Snapshot incremental en vez de recompute completo** (guardar un
  checkpoint de qué partidas ya se procesaron y solo recalcular las nuevas):
  más eficiente para bases grandes, pero resuelve un problema de escala que
  el proyecto no tiene todavía (recorrer todas las partidas finalizadas de
  un desarrollo en curso es barato) y complica la lógica de idempotencia
  justo en el punto donde hoy es más simple garantizarla (recalcular todo
  desde cero). Se prefiere la opción simple hasta que el volumen real lo
  justifique.
- **Hacer que `RecalculateForGame` sea idempotente por sí mismo** (con una
  tabla de auditoría `recalculated_games` o similar) en vez de delegar la
  invariante al script de backfill: se descartó por ahora porque agrega una
  tabla y un chequeo extra al camino caliente (fin de cada partida) para un
  caso que solo importa en el camino frío (backfill manual, poco frecuente).

## Consecuencias

- Cualquier migración nueva debe seguir el naming y la estructura de esta
  ADR; un review que vea un nombre fuera de patrón (fecha, sub-carpeta,
  `Down` que no revierte `Up` en orden inverso) puede señalarlo citando este
  documento.
- El comando `recalculate-stats` queda como una tarea futura concreta (no
  abierta todavía en `TASKS.md` como ítem de código, solo documentada acá) —
  se abre cuando de verdad cambie una fórmula de agregación y haga falta
  re-derivar historial.
- **Nota de seguimiento, fuera de alcance de esta ADR**: `docs-ci.yml`'s
  `dbml-validate` solo verifica que `schema.dbml` compile a SQL
  (`dbml2sql`), no lo compara contra el esquema realmente migrado — DBML y
  migraciones pueden divergir sin que CI lo detecte. Cerrar ese gap
  (comparar `dbml2sql` contra un `pg_dump --schema-only` de una base recién
  migrada) queda como mejora futura, no como parte de este cambio.

## Referencias

- `backend/migrations/00001_initial_schema.sql` a `00006_deck_image_url.sql`
- `backend/internal/statistics/service.go` (`RecalculateForGame`)
- `docs/database/schema.dbml` (nota sobre estadísticas pre-calculadas)
- [ADR-0008](0008-sqlc-goose.md) (sqlc + goose, orden de cambio de esquema)
- `docs/roadmap/TASKS.md`, Stage 2 y Stage 7
