# ADR-0005: Protocolo de sincronización en vivo por WebSocket

**Estado:** Aceptada e implementada parcialmente (2026-07-27) — servidor
implementado (`internal/websocket/`); cliente Android queda pendiente (ver
Stage 6 en `docs/roadmap/TASKS.md`).

## Contexto

El motor de partida (`internal/games`, `internal/game-actions`) ya es real:
`POST /games/:id/actions` registra acciones (`LifeChange`, `CombatDamage`,
`CommanderDamage`, `PoisonCounter`, `TurnStart`, `TurnEnd`, `Elimination`) y
muta el estado real del jugador afectado (`life_total`, `poison_counters`,
`is_eliminated`). El problema: si dos jugadores están sentados en la misma
partida, el cliente del jugador A no tiene forma de enterarse de una acción
que hizo el jugador B salvo haciendo polling manual de
`GET /games/:id/timeline` o `GET /games/:id`. Para una app de trackeo de vida
en vivo durante una partida de Commander, el polling es inaceptable en
latencia y en costo (N clientes preguntando cada X segundos por cada partida
activa).

Esta ADR define el protocolo mínimo de WebSocket para cerrar esa brecha:
qué eventos se retransmiten, a quién, con qué formato de mensaje, cómo se
autentica la conexión (el modelo de auth existente es 100% Bearer JWT sobre
headers HTTP, que no aplica directamente a un handshake de WebSocket desde
navegador), y qué pasa con la conexión durante el ciclo de vida de la
partida.

Alcance de esta pasada: **solo el servidor** (`internal/websocket/`, wireado
a `game-actions`/`games`). El cliente Android que consuma este protocolo
(conexión, reconexión con backoff, aplicar eventos entrantes al
`GameState`) es la última tarea de Stage 6 y no se aborda acá.

## Decisión

### 1. Qué se retransmite, y a quién

Se retransmiten **las siete acciones de `game_actions` sin excepción**
(`LifeChange`, `CombatDamage`, `CommanderDamage`, `PoisonCounter`,
`TurnStart`, `TurnEnd`, `Elimination`), de forma uniforme, más un evento de
ciclo de vida (`game_finished`) cuando la partida termina.

No se filtra ni se elige un subconjunto "más importante" de acciones,
porque:

- Las siete ya comparten una única forma en la API REST
  (`GameActionResponse`: `action_type` + `payload` libre); el cliente ya
  sabe interpretarlas todas para pintar el timeline. Reusar exactamente el
  mismo DTO en vivo evita mantener dos vocabularios de eventos.
- Si mañana se agrega un octavo `action_type` al vocabulario de
  `game-actions` (`isValidActionType`), se retransmite automáticamente sin
  tocar `internal/websocket`.

El destinatario de cada evento es **toda conexión suscripta a ese
`game_id`** — no se filtra por si el usuario conectado es jugador de esa
partida (ver "Fuera de alcance" más abajo, punto de autorización).

`games.JoinGame` / `LeaveGame` / `StartGame` (transiciones de la partida en
estado `pending`) **no se retransmiten** en esta pasada: ocurren antes de
que haya nada que trackear en vivo (la partida ni empezó), y el flujo hoy es
"todos se sientan, alguien aprieta empezar" en la misma pantalla — no hay
necesidad demostrada de verlo en vivo todavía. Queda como extensión natural
si aparece esa necesidad.

### 2. Formato del sobre del mensaje (envelope)

Todo mensaje que el servidor envía por el socket usa el mismo sobre JSON:

```json
{
  "type": "game_action",
  "game_id": "6e59b99a-...-uuid",
  "actor_id": "b4c9d1d0-...-uuid",
  "payload": { "...": "..." },
  "timestamp": "2026-07-27T14:32:01Z"
}
```

- `type`: uno de `connected`, `game_action`, `game_finished`, `error` (ver
  abajo).
- `game_id`: siempre presente, redundante con la sala a la que está
  suscripta la conexión (simplifica el cliente: no necesita recordar a qué
  partida pertenece cada socket si ya lo lee del mensaje).
- `actor_id`: quién originó el evento; vacío/omitido en eventos que no
  tienen un actor natural (`connected`, `game_finished`, `error`).
- `payload`: específico de `type`; ver detalle por evento debajo.
- `timestamp`: hora del servidor en el momento de emitir el mensaje
  (RFC3339, UTC), **no necesariamente igual** a
  `GameActionResponse.created_at` (que es la hora de persistencia en
  Postgres) — son eventos ligeramente distintos (una acción se persiste, y
  luego, por separado, se retransmite), aunque en la práctica ocurren en el
  mismo request y difieren en microsegundos.

Por tipo:

- **`connected`**: el servidor lo envía una única vez, justo después de que
  la conexión autentica correctamente. `payload` vacío. Sirve de ack: el
  cliente sabe que ya está suscripto y puede dejar de mostrar un spinner de
  "conectando".
- **`game_action`**: `payload` es exactamente un `GameActionResponse` (el
  mismo DTO que ya devuelve `POST /games/:id/actions` y
  `GET /games/:id/timeline` — `id`, `game_id`, `actor_id`, `target_id`,
  `action_type`, `payload`, `created_at`). `actor_id` del sobre es el mismo
  que `payload.actor_id`, duplicado a nivel de sobre para que el cliente
  pueda filtrar/rutear sin deserializar el payload completo.
- **`game_finished`**: `payload` vacío. Es un aviso, no un snapshot — el
  cliente debe pedir el estado final real por REST
  (`GET /games/:id`, `GET /statistics/*`) si lo necesita, en vez de que el
  servidor duplique esa información por dos canales. Ver "REST sigue siendo
  la fuente de verdad" más abajo.
- **`error`**: solo se usa durante el handshake de autenticación (ver
  sección 3), nunca después de que la conexión quedó autenticada.
  `payload: { "message": "..." }`.

**REST sigue siendo la fuente de verdad.** El WebSocket es un canal de
*notificación* de que algo cambió (y, para `game_action`, qué cambió
exactamente), no una fuente de verdad alternativa ni un mecanismo con
garantías de entrega. Frente a cualquier duda de sincronización (reconexión,
mensaje perdido, condición de carrera al conectar), el cliente reconcilia
contra `GET /games/:id` / `GET /games/:id/timeline`. Esta decisión es la que
permite dejar explícitamente fuera de alcance el replay de mensajes (sección
5): el costo de no tenerlo es "un round-trip extra a REST en el peor caso",
no pérdida de estado.

### 3. Autenticación de la conexión

**Decisión: mensaje de auth inicial después de conectar**, no JWT por query
param ni por subprotocolo.

El cliente abre el WebSocket sin credenciales en el handshake HTTP, y como
**primer mensaje de texto** (con un timeout de 10s) debe enviar:

```json
{ "type": "auth", "token": "<access token JWT, el mismo de Authorization: Bearer>" }
```

El servidor valida el JWT con la misma lógica que ya usa
`auth.RequireAuth` (`auth.VerifyAccessToken`, nueva función exportada que
envuelve la verificación de firma/expiración ya existente en
`internal/auth/token.go` — no se duplica la lógica de verificación). Si es
válido, responde `connected` y la conexión queda suscripta a la sala del
`game_id` de la URL. Si no llega el mensaje a tiempo, no es JSON válido, no
es `type: "auth"`, o el token es inválido/expiró, el servidor envía un
`error` con el motivo y cierra el socket con código `1008` (Policy
Violation).

Se descartaron las otras dos formas estándar de resolver esto:

- **JWT como query param en la URL del handshake**
  (`GET /ws/games/:id?token=...`): es la opción más simple de implementar y
  la más común en tutoriales, pero se descartó por dos razones concretas de
  *este* backend, no genéricas:
  1. `main.go` ya tiene `logger.New()` de Fiber como middleware global,
     que loguea el path completo de cada request — incluida la query string
     — de **toda** request HTTP, y el handshake de WebSocket es una request
     HTTP normal antes del upgrade. Un access token en la URL terminaría en
     los logs del servidor en texto plano en cada conexión, algo que ya se
     evita deliberadamente para el resto de la API (Bearer token va en un
     header, no en la URL, precisamente para no aparecer en logs de acceso
     ni en historiales de proxies intermedios).
  2. Habría que decidir qué hacer con ese query param en cualquier proxy
     reverso o CDN delante del backend a futuro (varios cachean o loguean
     querystrings por defecto) — un problema que el mensaje de auth inicial
     no tiene, porque el token nunca viaja en la URL.
- **JWT como `Sec-WebSocket-Protocol` (subprotocolo)**: evita el problema de
  logging de la URL, pero es un uso semánticamente incorrecto del campo
  (está pensado para negociar qué protocolo de aplicación se habla sobre el
  socket, no para transportar credenciales) y tiene restricciones de
  caracteres/longitud que obligan a codificar el JWT de formas no
  estándar en algunos clientes. El mensaje de auth inicial logra el mismo
  resultado (cero tokens en la URL/headers del handshake) sin pelearse con
  esas restricciones, a costa de un pequeño estado intermedio ("conectado
  pero no autenticado todavía") que hay que manejar con un timeout — costo
  que se considera aceptable y ya está resuelto en la implementación.

Esto es coherente con el resto del modelo de auth del proyecto
([ADR-0001](0001-auth-jwt-refresh-token-strategy.md)): sigue siendo
Bearer-JWT-only, sin cookies ni sesiones server-side nuevas; el WebSocket
solo cambia *cómo* viaja el mismo token, no qué token es ni cómo se emite.

### 4. Ciclo de vida de la conexión

- **Conectar**: `GET /api/v1/ws/games/{game_id}` (ruta pública, sin
  `auth.RequireAuth` — la autenticación ocurre por el mensaje inicial, no
  por el header de la request de upgrade). Se valida que `{game_id}` tenga
  formato UUID antes de aceptar el upgrade (400 si no); **no** se valida
  que la partida exista ni que el usuario autenticado sea un jugador de
  ella (ver "Fuera de alcance").
- **Autenticar**: ver sección 3. Éxito → `connected` + queda suscripto.
  Fallo → `error` + cierre `1008`.
- **Mientras dura la partida (`active`)**: cada `game_action` registrada
  exitosamente por `POST /games/:id/actions` se retransmite a la sala.
  Best-effort: si una conexión tiene su buffer de salida lleno (cliente
  lento o colgado), ese mensaje puntual se descarta *solo para esa
  conexión* — nunca bloquea el request HTTP que originó la acción ni afecta
  a las demás conexiones de la sala.
- **Un cliente se desconecta** (cierra la app, pierde la red, refresca la
  página): el servidor lo detecta cuando la próxima lectura/escritura sobre
  ese socket falla, y lo remueve de la sala. No hay que avisarle a nadie
  más — no hay eventos de presencia en esta pasada (ver "Fuera de
  alcance").
- **La partida termina** (`FinishGame`, vía `games.Broadcaster`): se
  retransmite `game_finished` a toda la sala y el servidor **cierra
  activamente todas las conexiones** de esa sala (código `1000`, cierre
  normal). Justificación: una vez `finished`, `game-actions.RecordAction`
  rechaza cualquier acción nueva (`game is not active`, 409) — no puede
  haber más `game_action` para esa sala nunca más, así que mantener el
  socket abierto solo consumiría un file descriptor sin propósito. El
  cliente que quiera el resultado final ya sabe pedirlo por REST
  (`GET /games/:id`, endpoints de `/statistics/*`).
- **Reconexión**: no hay continuidad de sesión entre conexiones — una
  reconexión es indistinguible de una conexión nueva (nuevo mensaje `auth`,
  nueva entrada en la sala). El cliente es responsable de, al (re)conectar,
  refrescar su estado desde REST antes o en paralelo a suscribirse (evita
  perder acciones que hayan ocurrido durante el corte) y de deduplicar por
  `GameActionResponse.id` si un evento que ya aplicó por REST también le
  llega luego por WebSocket (esto pasa siempre, de hecho, para el propio
  autor de la acción: recibe su `GameActionResponse` como respuesta del
  `POST`, y una copia idéntica más tarde por WebSocket — no es un bug, es
  la consecuencia de retransmitir a "toda la sala" sin excluir al emisor,
  ver "Fuera de alcance").

### 5. Fuera de alcance de esta pasada

Documentado explícitamente para no confundir "no implementado" con
"olvidado":

- **Garantías de entrega / replay al reconectar**: no hay cola de mensajes
  pendientes ni buffer de "lo que te perdiste". Si una conexión no está
  suscripta en el momento de un `Broadcast` (todavía no conectó, se cayó,
  o su buffer estaba lleno), ese mensaje se pierde para ella
  definitivamente. Mitigado por el punto de la sección 2 (REST es la
  fuente de verdad; el cliente reconcilia). Justificación de por qué se
  difiere: implementarlo bien requiere decidir dónde vive ese buffer
  (¿en memoria del proceso? ¿se pierde igual si el proceso reinicia?
  ¿por cuánto tiempo? ¿un log persistente tipo Kafka/Redis Streams?) — es
  una decisión de infraestructura no trivial que no se justifica sin datos
  reales de cuán seguido pasa un corte de red durante una partida.
- **Autorización a nivel de jugador**: cualquier usuario autenticado (JWT
  válido) puede suscribirse a **cualquier** `game_id`, exista o no, sea o
  no jugador de esa partida — el servidor no valida membership. El único
  control es "tener un JWT válido" (igual que el resto de la API exige
  estar logueado, pero no exige ser dueño del recurso en varios paths de
  lectura). El riesgo real es bajo (los `game_id` son UUIDs v4 no
  adivinables, y ver la sala de otro no expone más que lo que ya expone
  `GET /games/:id/timeline`, endpoint que hoy tampoco valida membership),
  pero es una laguna real que se documenta en vez de asumir que no existe.
- **Escalado multi-proceso / pub-sub externo**: el `Hub` vive en memoria de
  un único proceso (`map[game_id][]conn` protegido por un `sync.RWMutex`).
  Si el backend corre en más de una réplica, dos jugadores de la misma
  partida conectados a réplicas distintas **no se ven entre sí** — cada
  proceso solo sabe de sus propias conexiones. Resolver esto requiere un
  bus de mensajes compartido (Redis Pub/Sub, NATS, `LISTEN`/`NOTIFY` de
  Postgres) del que hoy no hay necesidad: el backend corre como un único
  proceso (ver `docker-compose.yml`, sin ningún componente de
  orquestación/escalado horizontal todavía).
- **Presencia** ("qué jugadores están conectados ahora mismo") y
  **indicadores de actividad** (typing/"fulano está pensando su turno"): no
  hay ningún evento de este tipo. Es una mejora de UX real pero
  independiente del problema que esta ADR resuelve (sincronizar el estado
  del juego), y agregarla implica decisiones propias (¿qué es "presente":
  el socket abierto, o alguna interacción reciente? ¿se retransmite
  join/leave del socket, aunque no correspondan 1:1 con estar sentado en la
  partida?).
- **Canal cliente→servidor sobre el socket**: el WebSocket es unidireccional
  server→client después del mensaje `auth` inicial — el servidor ignora
  cualquier mensaje posterior que un cliente le envíe. Registrar acciones
  sigue siendo exclusivamente vía `POST /games/:id/actions` (REST). No hay
  ninguna ventaja de latencia relevante en mover ese POST al socket para
  esta app, y hacerlo obligaría a duplicar toda la validación/autorización
  de `game-actions.RecordAction` en el handler del socket.
- **Heartbeat / ping-pong applicativo**: no se implementa un ticker de
  ping/pong explícito. La detección de conexiones muertas depende de que la
  próxima lectura o escritura sobre el socket TCP falle (lo cual puede
  tardar, según el SO, bastante más que un ping/pong explícito ante un
  corte "silencioso" de red, ej. el cliente pierde conectividad sin cerrar
  limpio). Se documenta como limitación conocida, no como decisión
  definitiva — es la primera candidata a agregar si en la práctica se ven
  salas con conexiones fantasma acumulándose.

## Alternativas consideradas (arquitectura general)

- **Server-Sent Events (SSE) en vez de WebSocket**: sería suficiente para
  el caso de uso actual (server→client únicamente, ver punto de "canal
  cliente→servidor" arriba) y más simple de implementar sobre HTTP/1.1
  puro. Se descartó igual porque (a) el roadmap ya nombra a este stage
  "Sincronización (Websocket)" explícitamente, y (b) SSE tiene peor soporte
  nativo en Android/OkHttp que WebSocket, que es un ciudadano de primera
  clase en el stack Android ya elegido — no vale la pena pelearse con un
  polyfill de SSE en el cliente principal para ahorrarse la (pequeña)
  complejidad extra de manejar el handshake de WebSocket en el servidor.
- **Polling corto (short polling) como solución "suficiente"**: es lo que
  existe hoy de facto (nada, en realidad — ni siquiera hay polling
  implementado en el cliente). Se descartó como alternativa *permanente*
  precisamente porque es el problema que esta ADR resuelve, no una opción
  competitiva.

## Consecuencias

- `internal/auth` gana una función exportada nueva
  (`auth.VerifyAccessToken`) que expone la verificación de JWT que antes
  solo usaba el middleware `RequireAuth` internamente. Superficie de
  ataque adicional: ninguna nueva (misma verificación, un caller más).
- `games.Service` y `gameactions.Service` ganan una dependencia nueva
  (`Broadcaster`, un método cada uno) inyectada por constructor, con el
  mismo patrón que `games.StatisticsRecalculator` — ninguno de los dos
  importa `internal/websocket` directamente, evitando un acoplamiento
  fuerte y permitiendo mockear el broadcast en tests.
- Nueva ruta pública (sin `auth.RequireAuth`) en la superficie HTTP:
  `GET /api/v1/ws/games/:id`. Documentar en `openapi.yaml` queda pendiente
  como tarea separada (OpenAPI 3.1 no modela bien WebSockets; probablemente
  amerite solo una nota en la descripción del path REST equivalente en vez
  de un intento de spec formal).
- El `Hub` en memoria es un límite de escalado conocido y documentado
  (ver "Fuera de alcance"): mudar el backend a múltiples réplicas requiere
  revisar esta ADR primero.

## Referencias

- Implementación: `backend/internal/websocket/` (`hub.go`, `client.go`,
  `handler.go`, `envelope.go`, `broadcaster.go`)
- Wiring: `backend/cmd/api/main.go` (`registerModules`)
- Interfaces de desacoplamiento: `backend/internal/games/service.go`
  (`Broadcaster`), `backend/internal/game-actions/service.go`
  (`Broadcaster`) — mismo patrón que `games.StatisticsRecalculator`
- Verificación de JWT reutilizada: `backend/internal/auth/token.go`
  (`VerifyAccessToken`)
- Ver también [ADR-0001](0001-auth-jwt-refresh-token-strategy.md) (modelo
  de auth Bearer-JWT que esta ADR reutiliza sin modificar)
