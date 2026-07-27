# ADR-0007: PostgreSQL como base de datos principal

**Estado:** Aceptada e implementada — **decisión heredada, contexto
reconstruido**. Al igual que ADR-0006, esta decisión precede el historial de
ADRs del proyecto y se documenta retroactivamente (2026-07-27) a partir del
estado real del código (`backend/migrations/`, `docs/database/schema.dbml`,
`backend/go.mod`), no de una discusión presenciada en su momento.

## Contexto

El modelo de datos (Stage 2 del ROADMAP) necesita relaciones bien definidas
desde el día uno: usuarios, decks, partidas (`games`), jugadores de partida
(`game_players`), acciones de partida (`game_actions`), grupos de juego
(`playgroups`), y tablas de resumen de estadísticas
(`user_statistics_summary`, `deck_statistics_summary`). El ROADMAP fija
PostgreSQL explícitamente en el stack de Stage 1 y en las dos versiones del
diagrama de arquitectura (`ROADMAP.md`, ambos diagramas Mermaid terminan en
`PostgreSQL`).

## Decisión

**PostgreSQL** como único motor de base de datos, accedido vía
`github.com/jackc/pgx/v5` (driver + pool `pgxpool.Pool`) desde Go, con
`sqlc` generando el código de acceso tipado sobre ese driver (ver ADR-0008)
y `goose` gestionando las migraciones versionadas (`migrations/00001_initial
_schema.sql`, `00002_auth.sql`).

## Alternativas consideradas

- **MySQL/MariaDB**: igualmente viable para un modelo relacional
  convencional; se descarta a favor de Postgres probablemente por
  características que el esquema ya aprovecha o previsiblemente aprovechará:
  `CHECK` constraints expresivos (`CHECK (password_hash IS NOT NULL OR
  google_id IS NOT NULL)` en `users`, ver `migrations/00002_auth.sql`), tipos
  nativos más ricos (UUID, JSON/JSONB — usado en `game_actions.payload`
  según `game-actions/service.go`, que serializa/deserializa el payload de
  cada acción como JSON), y el ecosistema de herramientas Go (`pgx` es el
  driver de facto más maduro y performante del ecosistema Go moderno, más
  que sus equivalentes MySQL).
- **Base de datos NoSQL (MongoDB, DynamoDB)**: descartada porque el dominio
  es intrínsecamente relacional (usuarios ↔ decks ↔ partidas ↔ jugadores de
  partida ↔ acciones, con integridad referencial real: un `game_player` no
  puede existir sin su `game` y su `deck`) y porque las estadísticas
  agregadas (Stage 7) se benefician de `GROUP BY`/agregaciones SQL en vez de
  map-reduce o agregación de documentos.
- **SQLite**: descartado por no encajar con "Sincronización entre
  jugadores" y "Arquitectura escalable" (objetivos explícitos del
  ROADMAP) — el backend es un servidor centralizado con múltiples clientes
  concurrentes, no una app de un solo usuario con almacenamiento embebido
  (ese rol lo cumple Room, pero *en Android*, no en el backend — ver
  ADR-0009).

## Consecuencias

- El esquema vive en `docs/database/schema.dbml` como fuente de verdad
  (una de las "cuatro fuentes de verdad" declaradas en `ROADMAP.md`), y se
  valida en CI compilando a SQL con `@dbml/cli` (`docs-ci.yml`) — atarse a
  Postgres específicamente (no "SQL genérico") ya se refleja en el uso de
  tipos y sintaxis Postgres-específicos en las migraciones.
  y en las queries. Migrar de motor a esta altura implicaría reescribir
  DBML, migraciones y queries `sqlc`.
- Todo el testing de integración del backend (`internal/testutil`,
  usado por `auth`, `decks`, `games`, `game-actions`, `playgroups`,
  `statistics`) corre contra **Postgres real**, no un mock ni SQLite en
  memoria — más fiel a producción, pero requiere una instancia de Postgres
  disponible (localmente o en CI, ver `backend-ci.yml`) para correr los
  tests, y obliga a `go test -p 1` porque los tests comparten la misma base
  y hacen `TRUNCATE` entre sí.
- El motor de estadísticas (Stage 7) y el futuro Match Engine (ver segundo
  diagrama de `ROADMAP.md`) asumen que pueden leer/escribir contra la misma
  instancia de Postgres sin una capa de replicación o sharding — aceptable
  mientras el proyecto siga siendo un monolito modular de un solo
  mantenedor (ver ADR-0010).

## Referencias

- `docs/database/schema.dbml`
- `backend/migrations/00001_initial_schema.sql`, `00002_auth.sql`
- `backend/go.mod` (`github.com/jackc/pgx/v5`)
- `docs/roadmap/ROADMAP.md`, sección "Fuentes de verdad" y ambos diagramas
  de arquitectura
