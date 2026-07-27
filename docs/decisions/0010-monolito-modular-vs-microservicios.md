# ADR-0010: Monolito modular en vez de microservicios

**Estado:** Aceptada e implementada — **decisión heredada, contexto
reconstruido** (ver nota de método en ADR-0006; redactado retroactivamente
el 2026-07-27 a partir de `backend/cmd/api/main.go` y la estructura de
`backend/internal/`).

## Contexto

`docs/roadmap/ROADMAP.md` es explícito y temprano sobre esto: "Inicialmente
será un **Monolito Modular**. No habrá microservicios." — es una de las
pocas decisiones arquitectónicas declaradas en el propio ROADMAP en vez de
quedar implícita en el código. El proyecto necesitaba organizar
funcionalidad claramente separable por dominio (auth, usuarios, decks,
partidas, acciones de partida, grupos de juego, estadísticas, sincronización,
websocket) sin necesariamente pagar desde el día uno el costo operativo de
desplegar y coordinar servicios independientes.

## Decisión

**Un único binario Go** (`cmd/api/main.go`) que registra todos los módulos
de dominio como paquetes independientes bajo `internal/` (`auth`, `users`,
`decks`, `games`, `game-actions`, `playgroups`, `statistics`, `sync`,
`websocket`, `common`), cada uno con su propio `service.go` (lógica de
negocio), `handler.go` (rutas Fiber), y `query.sql`/código generado por sqlc
(acceso a datos) donde aplica — pero todos compartiendo el mismo proceso,
el mismo `pgxpool.Pool` de conexión a Postgres, y el mismo despliegue.
`ROADMAP.md` documenta explícitamente la evolución prevista: un segundo
diagrama de arquitectura "en fases posteriores" separa un `API Gateway`, un
`Match Engine` y un `Statistics Engine`, pero como aspiración a futuro, no
como diseño inicial.

## Alternativas consideradas

- **Microservicios desde el inicio** (un servicio por dominio: auth-service,
  games-service, statistics-service, etc.): se descartó explícitamente por
  el ROADMAP. El costo de coordinación (service discovery, comunicación
  inter-servicio, despliegues independientes, consistencia transaccional
  entre servicios — p. ej. `FinishGame` necesita disparar
  `RecalculateForGame` de forma fiable, algo trivial como una llamada de
  interfaz en-proceso hoy y mucho más costoso como llamada de red con sus
  propios fallos parciales) no se justifica para un equipo de un solo
  mantenedor y un producto en la fase de definir su MVP.
- **Monolito no-modular** (todo en un solo paquete `main`, sin separación
  por dominio): hubiera sido más rápido de arrancar, pero mucho más difícil
  de mantener a medida que crecen los módulos — la separación en `internal/
  <dominio>/` con interfaces explícitas entre módulos (p. ej.
  `games.StatisticsRecalculator`, la interfaz que `games` usa para
  desacoplarse de la implementación concreta de `statistics` y poder
  mockearla en tests) ya funciona como frontera modular clara incluso
  dentro de un único proceso, y es lo que permitiría extraer un módulo a su
  propio servicio más adelante sin rediseñar su lógica interna.
- **Extraer el Websocket/Match Engine como servicio aparte desde ya**
  (Stage 6, todavía no implementado): se decidió no adelantarlo — el
  ROADMAP lo deja para "fases posteriores" explícitamente, y hoy
  `internal/websocket/` está vacío. Adelantar esa separación sin tener
  siquiera el protocolo de mensajes diseñado (pendiente en `TASKS.md`,
  Stage 6) sería resolver un problema de escala que todavía no existe.

## Consecuencias

- Todos los módulos comparten el mismo `pgxpool.Pool` y el mismo ciclo de
  vida de proceso — un fallo o un despliegue del backend afecta a *todos*
  los dominios (auth, games, decks, etc.) a la vez; no hay aislamiento de
  fallos entre módulos como sí lo daría un microservicio caído de forma
  independiente.
- Escalar horizontalmente hoy significa escalar el binario completo (más
  réplicas del mismo proceso detrás de un balanceador), no escalar
  selectivamente el módulo con más carga (p. ej. `game-actions` durante
  partidas activas, que previsiblemente recibe más tráfico que `playgroups`).
  Aceptable mientras el volumen de uso real no lo exija.
- Los límites de módulo (`internal/<dominio>/`) son convención de código, no
  un límite de despliegue reforzado por infraestructura — nada impide hoy
  que un módulo importe directamente el paquete interno de otro en vez de
  pasar por una interfaz explícita, salvo disciplina de code review (un solo
  mantenedor, sin proceso de aprobación de PR obligatorio, ver `TASKS.md`
  branch protection: "No se exige aprobación de PR").
- Si el proyecto necesita microservicios reales en el futuro (el segundo
  diagrama de `ROADMAP.md` ya lo prevé), la migración más natural es
  extraer primero `websocket`/Match Engine (Stage 6, todavía sin código) y
  `statistics` (Stage 7, ya tiene una interfaz de desacople,
  `StatisticsRecalculator`, lista para convertirse en un cliente HTTP/gRPC
  en vez de una llamada en-proceso) — son los dos módulos que el propio
  ROADMAP ya dibuja como servicios separados a futuro.

## Referencias

- `docs/roadmap/ROADMAP.md`, sección "Arquitectura general" (ambos
  diagramas Mermaid) y "Inicialmente será un Monolito Modular"
- `backend/cmd/api/main.go`
- `backend/internal/` (estructura de módulos)
- `backend/internal/games/service.go` (interfaz `StatisticsRecalculator`
  como ejemplo de frontera modular explícita)
