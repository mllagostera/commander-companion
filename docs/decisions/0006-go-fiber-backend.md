# ADR-0006: Backend en Go con Fiber como framework HTTP

**Estado:** Aceptada e implementada — **decisión heredada, contexto
reconstruido**. Esta decisión es anterior al inicio del historial de ADRs
del proyecto (0001-0004, tomadas 2026-07-26); no hay registro contemporáneo
de por qué se eligió. Este documento se redacta retroactivamente
(2026-07-27) a partir de lo que ya está construido y confirmado en el
código (`backend/go.mod`, `backend/cmd/api/main.go`), no de una discusión
real que se haya presenciado.

## Contexto

El ROADMAP (`docs/roadmap/ROADMAP.md`) fija desde el principio "Go,
Gin/Fiber, PostgreSQL, sqlc, goose, Docker" como stack de Stage 1, y
"Arquitectura escalable" y "rapidez durante la partida" como objetivos de
producto. Había que elegir lenguaje y framework HTTP para una API REST
stateless, versionada (`/api/v1`), que sirve tanto al cliente Android nativo
como, después, a un cliente web (ver ADR-0004).

## Decisión

- **Lenguaje: Go** (`go 1.25.0` en `go.mod`).
- **Framework HTTP: Fiber v2** (`github.com/gofiber/fiber/v2 v2.52.14`),
  sobre `fasthttp` en vez del router `net/http` estándar o Gin (la otra
  opción que el propio ROADMAP dejaba abierta con "Gin/Fiber").

## Alternativas consideradas

- **Gin**: el ROADMAP lo menciona como alternativa explícita
  ("Gin/Fiber"). Ambos son frameworks maduros de rendimiento comparable en
  el ecosistema Go; se optó por Fiber probablemente por su API inspirada en
  Express (`c.Params`, `c.BodyParser`, middlewares con la misma firma que
  Node/Express), que reduce fricción de onboarding si el equipo viene de un
  stack JS/TS — el propio proyecto termina sumando un cliente Nuxt (ADR-0004)
  y una herramienta de test HTML (`tools/auth-test/`), consistente con
  familiaridad JS en el equipo.
- **`net/http` + router estándar (`chi`, `gorilla/mux`)**: más "sin
  dependencias" y más cercano a la librería estándar, pero requiere ensamblar
  a mano cosas que Fiber trae integradas (grouping de rutas, middleware de
  CORS ya usado vía `github.com/gofiber/fiber/v2/middleware/cors`, parsing de
  body). Para un equipo chico shippeando rápido, la superficie de Fiber ya
  lista se paga sola.
- **Otro lenguaje (Node/TypeScript, Python)**: dado que el cliente principal
  es Android nativo (no un framework JS compartido tipo React Native), no
  había presión de compartir código de dominio con el cliente; Go ofrece
  binarios de despliegue simples (un solo ejecutable + Docker liviano),
  tipado estático y buen soporte de concurrencia — razonable para una API
  que en fases posteriores (ver `ROADMAP.md`, "en fases posteriores") crece
  a Match Engine + Statistics Engine con Websocket.

## Consecuencias

- Fiber está construido sobre `fasthttp`, no sobre `net/http` — algunas
  librerías del ecosistema Go estándar que asumen `http.Handler` no son
  directamente compatibles y requieren un adaptador o el equivalente nativo
  de Fiber (en la práctica no ha sido un problema: auth, CORS, y las rutas
  de todos los módulos usan la API nativa de Fiber).
- La elección ya está profundamente enraizada en el código (`main.go`
  registra todos los módulos como grupos de rutas Fiber,
  `fiber.NewError` se usa como mecanismo estándar de mapear errores de
  dominio a códigos HTTP en todos los `service.go`) — revertir a otro
  framework HTTP hoy implicaría tocar el código de error-handling de los
  ocho módulos de `internal/`.
- Go como lenguaje fija el resto de la cadena de herramientas del backend:
  `sqlc` para generar código de acceso a datos tipado (ver ADR-0008),
  `golangci-lint` para linting, y el propio modelo de concurrencia de Go para
  cuando se implemente el Websocket de Stage 6.

## Referencias

- `backend/go.mod`
- `backend/cmd/api/main.go`
- `docs/roadmap/ROADMAP.md`, sección "Stage 1: Backend"
