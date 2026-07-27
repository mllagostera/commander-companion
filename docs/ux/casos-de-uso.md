# Casos de uso detallados

Flujos de usuario paso a paso para las cinco operaciones centrales del
producto: crear partida, unirse a una partida, trackear vida durante la
partida, finalizar partida y ver estadísticas.

**Cómo leer este documento:** para cada caso de uso se describen dos
columnas — **"Hoy"** (lo que el código hace ahora mismo, verificado leyendo
`backend/internal/games/service.go`, `backend/internal/game-actions/service.go`
y las pantallas Android en `presentation/screens/`) y **"Objetivo"** (el flujo
end-to-end que el ROADMAP prevé una vez Android hable con el backend, Stage
5). Hoy ambos existen en paralelo pero **desconectados**: el backend ya
implementa la máquina de estados completa de una partida multi-usuario
autenticada; el cliente Android es un life tracker 100% local (Room), de un
solo dispositivo, sin sesión de usuario ni llamada de red alguna al backend
de `games`/`game-actions`. Ver `docs/roadmap/TASKS.md`, Stage 4 y 5.

---

## 1. Crear partida

**Actor:** cualquier usuario (hoy: cualquiera con el dispositivo Android en
la mano; objetivo: usuario autenticado).

### Hoy (Android, 100% local)

1. Usuario abre la app → `LoginRoute` (pantalla de login, ver más abajo) →
   toca "INICIAR SESIÓN" o "Continuar con Google" → navega a
   `DashboardRoute` sin autenticar realmente contra nada (`LoginScreen.kt`
   no llama a ningún backend todavía; ambos botones solo disparan la
   navegación).
2. En `DashboardScreen`, toca **"NEW GAME"** → navega a `PlayerSetupRoute`.
3. En `PlayerSetupScreen` (`PlayerSetupScreen.kt`):
   - Elige la cantidad de jugadores con `FilterChip` (2 a 6, constantes
     `MIN_PLAYERS`/`MAX_PLAYERS`).
   - Para cada jugador: nombre libre (`OutlinedTextField`, default
     `"Jugador N"`) y color de entre la paleta WUBRG + incoloro
     (`PlayerColorPalette`, swatches circulares).
   - Toca **"EMPEZAR PARTIDA"**: se genera un `gameId` local con
     `UUID.randomUUID()` (no es un ID del backend, no existe ninguna fila de
     `games` en Postgres) y se codifican los `PlayerConfig` (nombre + color)
     en un string (`encodePlayerConfigs`) que viaja como argumento de ruta.
4. Navega a `PreGameRoute(gameId, playersEncoded)`.
5. En `PreGameScreen` (`PreGameScreen.kt`):
   - **Sorteo de turno**: botón "SORTEAR" elige un jugador al azar
     (`Random.nextInt(configs.size)`) y lo resalta con su color; el resultado
     (`startingSeat`) se propaga a la partida como badge "· empieza".
   - **Mulligans**: contador ± por jugador (mínimo 0, sin tope superior) que
     se guarda en cada `PlayerConfig.mulligans` antes de arrancar.
   - Toca **"EMPEZAR PARTIDA"** de nuevo → navega a `GameTrackerRoute`,
     haciendo `popUpTo(DashboardRoute)` (setup y pre-partida salen del
     back stack).
6. Al entrar a `GameTrackerScreen`, `GameViewModel.persistNewGame()` inserta
   en Room (Hilt `DatabaseModule`, `GameDao`) un `GameEntity` con
   `status = "IN_PROGRESS"` y un `PlayerResultEntity` por jugador (vida
   inicial 40, color, mulligans). **Este es el único punto de persistencia
   de "crear partida" hoy**: no hay ninguna request HTTP.

### Objetivo (backend real, Stage 5)

1. Un usuario autenticado (JWT Bearer) llama `POST /games` con
   `playgroup_id` opcional (`games/service.go: CreateGame`). Se crea una fila
   en `games` con `status = "pending"`, sin jugadores todavía.
2. Cada jugador (incluido el creador) se une explícitamente vía
   `POST /games/{id}/join` (ver caso de uso 2) — crear la partida y "sentarse"
   en ella son dos pasos separados en el backend, a diferencia de hoy en
   Android donde configurar jugadores y crear la partida es un solo paso.
3. Cuando hay ≥ 2 jugadores unidos, alguien llama `POST /games/{id}/start`,
   que transiciona `pending → active` (ver
   `docs/diagrams/game-state-machine.md`).
4. En Android, esto reemplazaría el paso 3-4 de "Hoy": en vez de generar un
   `gameId` local y codificar jugadores en la ruta, `PlayerSetupScreen`
   llamaría a `POST /games` + N × `POST /games/{id}/join` (con el `deck_id`
   de cada jugador autenticado) antes de navegar a `PreGameScreen`.

**Divergencia clave:** el backend modela "crear" y "unirse" como pasos
distintos de una partida multi-usuario con autenticación y decks reales; hoy
Android los colapsa en un único flujo de configuración local sin usuarios ni
decks, porque todos los jugadores comparten el mismo dispositivo físico.

---

## 2. Unirse a una partida

**Actor:** usuario autenticado con al menos un deck propio.

### Hoy (Android)

No existe como caso de uso independiente. "Unirse" en la práctica es
agregar una fila más en la lista de jugadores de `PlayerSetupScreen` (subir
el `FilterChip` de cantidad de 2 a 6) — no hay concepto de invitar, aceptar
ni identidad de usuario por jugador. Todos los "jugadores" son simplemente
nombres + colores tecleados por la persona que tiene el teléfono.

### Objetivo (backend real, `games/service.go: JoinGame`)

1. Usuario autenticado llama `POST /games/{id}/join` con body `{ deck_id }`
   (el `user_id` **no** va en el body — se toma siempre del JWT, para que
   nadie pueda anotar jugadores a nombre de otro usuario).
2. Validaciones server-side, en orden:
   - La partida debe existir y estar en `status = "pending"` — si no, `409
     "game is not accepting new players"`.
   - El `deck_id` debe existir y pertenecer al usuario autenticado — si no
     existe **o** es de otro usuario, `404 "deck not found"` en ambos casos
     (no se distingue cuál, para no revelar decks ajenos por ID).
   - El usuario no debe estar ya sentado en esa partida — si ya está, `409
     "already joined this game"`.
3. Si todo es válido, se crea una fila en `game_players` con vida inicial
   (`life_total` default de esquema), y se puede seguir jugando/uniendo
   mientras la partida siga en `pending`.
4. Un jugador puede arrepentirse y llamar `POST /games/{id}/leave` mientras
   la partida siga en `pending` (`LeaveGame`); una vez `active`, `409
   "cannot leave a game that already started"`.

**Divergencia clave:** en el backend "unirse" implica autenticación +
ownership de un deck concreto; en Android hoy no hay ni sesión ni deck
asociado a cada jugador de la partida local.

---

## 3. Trackear vida durante la partida

**Actor:** cualquier jugador (hoy: quien toca la pantalla del dispositivo
compartido; objetivo: cada jugador desde su propio dispositivo, sincronizado
vía backend/Websocket).

### Hoy (Android, `GameTrackerScreen` + `GameViewModel` + `PlayerCard`)

1. `GameTrackerScreen` arma un grid dinámico: los jugadores se agrupan de a
   2 por fila (`state.players.chunked(2)`), funciona igual para 2 que para 6
   jugadores (ya no hay layout fijo a 4).
2. **Contador de turno** en el header: botones `<`/`>` incrementan o
   decrementan `state.currentTurn` (`GameViewModel.nextTurn/previousTurn`).
   Es puramente local a la sesión, no se persiste como evento y no indica de
   quién es el turno (solo un número global).
3. **Vida** (`PlayerCard`): cada tarjeta muestra el nombre del jugador (con
   sufijo "· empieza" si es `isStartingPlayer`), el badge de mulligans si
   `mulligans > 0`, y el número de vida grande en el centro con botones
   `-`/`+` que llaman `onLifeChange(-1)`/`onLifeChange(1)` →
   `GameViewModel.adjustLife(playerId, amount)`. Cada ajuste dispara
   `checkForGameOver()`.
4. **Daño de comandante**: tocar la tarjeta entera (`clickable` en el
   `Surface`) alterna un overlay (`showCommanderDamage`) que muestra, en una
   grilla de hasta 3 columnas, un ítem por cada **otro** jugador
   (`otherPlayers`) con su color, el daño de comandante acumulado recibido de
   ese atacante, y botones `-`/`+`. Subir el daño de un atacante también
   resta esa misma cantidad de la vida total del jugador (`life = life -
   amount` en `adjustCommanderDamage`) — es decir, "daño de comandante" no es
   un contador aparte de la vida, sino una forma con memoria por-oponente de
   restar vida.
   - **Límite conocido**: el modelo (`PlayerState.commanderDamage: Map<Int,
     Int>`) trackea daño acumulado por oponente pero no implementa la regla
     de "21 de daño de comandante de una **misma fuente** elimina" —
     simplemente resta vida; el jugador se elimina solo cuando su vida total
     llega a 0 (igual límite que en el backend, ver abajo).
5. Cada cambio de vida o de daño de comandante llama `checkForGameOver()`:
   si queda exactamente 1 jugador con `life > 0` (y hay más de 1 jugador en
   la partida), se finaliza automáticamente esa partida con ese jugador como
   ganador (ver caso de uso 4).
6. Nada de esto genera una request HTTP ni un registro en una tabla de
   acciones — todo vive en `_state: MutableState<GameState>` en memoria hasta
   que se finaliza la partida (ver caso de uso 4). Si el proceso muere antes
   de finalizar, se pierde el estado de vida en curso (documentado como fuera
   de alcance en `TASKS.md`).

### Objetivo (backend real, `game-actions/service.go`)

1. Cada cambio de vida/daño se registraría como
   `POST /games/{id}/actions` con `{ actor_id, target_id?, action_type,
   payload: { amount } }`, donde `actor_id`/`target_id` son IDs de
   `game_players` de **esa** partida (no `user_id`), y `action_type` es uno
   de: `LifeChange`, `CombatDamage`, `CommanderDamage`, `PoisonCounter`,
   `TurnStart`, `TurnEnd`, `Elimination`.
2. El backend valida que la partida esté `active` (`409` si no) y que
   actor/target pertenezcan a esa partida (`404` si no), y **muta el estado
   real** del jugador afectado, no solo registra el evento en un log:
   - `LifeChange` ajusta `life_total` en cualquier signo.
   - `CombatDamage`/`CommanderDamage` restan de `life_total` (mismo efecto
     hoy — el esquema no distingue el origen del daño de comandante por
     jugador, así que **tampoco el backend implementa todavía** la regla de
     21 de una sola fuente; ver `applyAction` en `game-actions/service.go`,
     comentado explícitamente como límite conocido).
   - `PoisonCounter` ajusta `poison_counters`.
   - `TurnStart`/`TurnEnd` quedan como marcadores de solo-log (el esquema de
     `games` no tiene columna de "de quién es el turno actual" todavía).
   - **Auto-eliminación server-side**: si tras un ajuste `life_total <= 0` o
     `poison_counters >= 10`, el jugador se marca `is_eliminated = true`
     automáticamente (reglas estándar de Commander) — equivalente a lo que
     hoy Android decide localmente al chequear `life > 0`.
3. `GET /games/{id}/actions` expone el timeline completo, ordenado
   cronológicamente, para reconstruir la partida (usado también por
   `statistics.RecalculateForGame`, ver caso de uso 5).
4. Retransmitir estos eventos en tiempo real a todos los dispositivos
   sentados en la partida es Stage 6 (Websocket), todavía sin implementar
   (`internal/websocket/` está vacío).

**Divergencia clave:** hoy la vida vive solo en memoria del dispositivo que
la está tocando, sin registro de acciones individuales ni de quién causó
cada cambio; el backend ya tiene un motor completo de acciones auditable
por jugador, pero ninguna pantalla de Android lo llama todavía.

---

## 4. Finalizar partida

**Actor:** cualquier jugador presente (hoy); el backend no distingue quién
puede finalizar (cualquier request autenticada a ese endpoint puede hacerlo,
no hay un rol de "host").

### Hoy (Android, `GameViewModel.finishGame`)

1. Dos disparadores posibles:
   - **Automático**: `checkForGameOver()` detecta que queda 1 solo jugador
     con `life > 0` tras cualquier cambio de vida/daño, y llama
     `finishGame(winnerId = alive.first().id)` directamente.
   - **Manual**: el jugador toca **"Finalizar"** en el header →
     `showFinishConfirm = true` → `AlertDialog` de confirmación ("Se
     registrará la vida actual de cada jugador en el historial") → confirmar
     llama `finishGame()` sin `winnerId` explícito.
2. Si se finaliza manualmente sin ganador claro, `finishGame` resuelve el
   ganador como el jugador con más vida entre los que tienen `life > 0`,
   **solo si es único** (`count { it.life == player.life } == 1`); si hay
   empate en el máximo (2+ jugadores con la misma vida más alta) o todos
   están a 0 o menos, `winnerId = null` → la partida queda "sin ganador".
3. Se marca `isFinished = true` y se dispara `persistGameResult`: en Room,
   `gameDao.finishGame(gameId, status = "FINISHED", endTime = now)` y, por
   cada jugador, `updatePlayerResult(gameId, seatIndex, finalLife, won =
   (id == winnerId))`.
4. Se muestra un `AlertDialog` final: título "¡{ganador} gana!" o "Partida
   finalizada" si no hay ganador, y el detalle de vida final de cada
   jugador. "Volver al inicio" hace `onFinish()` →
   `navController.popBackStack(DashboardRoute, inclusive = false)`.
5. Una vez `isFinished = true`, tanto `adjustLife` como
   `adjustCommanderDamage` y `finishGame` vuelven a ejecutarse como no-op (se
   chequea `if (_state.value.isFinished) return` al principio) — la partida
   queda congelada.

### Objetivo (backend real, `games/service.go: FinishGame`)

1. `POST /games/{id}/finish` solo es válido si la partida está `active` —
   `409 "only an active game can be finished"` en cualquier otro estado
   (incluido intentar finalizar una ya finalizada).
2. A diferencia de Android, el endpoint **no recibe un ganador explícito**:
   la partida pasa a `status = "finished"` con `finished_at` seteado, y
   quién ganó se **deriva después**, en `statistics.RecalculateForGame`
   (disparado automáticamente dentro de la misma transacción lógica de
   `FinishGame`, vía la interfaz `StatisticsRecalculator`), usando el mismo
   criterio de "único sobreviviente no eliminado" (`is_eliminated = false`)
   — si hay 2+ sobrevivientes porque la partida se cortó a mano antes de
   llegar a 1, no se acredita victoria a nadie, aunque sí se cuenta
   `games_played` para todos los participantes.
3. Este recálculo también actualiza `user_statistics_summary` y
   `deck_statistics_summary` (daño infligido, vida máxima alcanzada, etc.),
   que es la base del caso de uso 5.

**Divergencia clave:** Android decide el ganador **en el cliente**, en el
momento de finalizar, con una regla algo distinta (máxima vida con empate
como "sin ganador", en vez de único sobreviviente no eliminado); el backend
lo decide **en el servidor**, después de finalizar, con la regla de "único
sobreviviente". Ambas reglas coinciden en el caso común (queda 1 jugador con
vida > 0) pero pueden divergir en partidas cortadas a mano con más de un
jugador vivo.

---

## 5. Ver estadísticas

**Actor:** cualquier usuario (objetivo); en Android hoy no hay concepto de
usuario, solo de dispositivo.

### Hoy (Android, `HistoryScreen` + `HistoryViewModel`)

1. Desde `DashboardScreen`, botón **"HISTORIAL"** → `HistoryRoute`.
2. `HistoryViewModel` expone `gameDao.getGamesWithPlayers()` como
   `StateFlow<List<GameWithPlayers>>` — **es historial de partidas locales
   de este dispositivo, no estadísticas agregadas** (no hay winrate, no hay
   totales de daño, no hay gráficos).
3. Por cada partida (`GameHistoryCard`): fecha/hora de inicio formateada,
   estado ("Finalizada" / "En curso"), nombre del ganador si lo hay (o la
   cantidad de jugadores si no), y una fila con el color + nombre + vida
   final de cada jugador ordenados por asiento, con sufijo `(Nm)` si tuvo
   mulligans.
4. Si no hay partidas guardadas, se muestra el mensaje "Todavía no hay
   partidas registradas".
5. **No existe ninguna pantalla de estadísticas agregadas en Android hoy**
   (por usuario, por deck o por grupo) — es historial crudo partida por
   partida, y además está atado a Room local: si se desinstala la app o se
   cambia de dispositivo, se pierde.

### Objetivo (backend real, `internal/statistics`)

Tres endpoints ya implementados y con tests de integración, pero **sin
ninguna pantalla de Android que los consuma todavía** (bloqueado por Stage
4/5 — Android no habla con el backend):

- `GET /statistics/user/{id}`: `games_played`, `games_won`,
  `total_damage_dealt`, `total_commander_damage_dealt`, `total_eliminations`
  acumulados de todas las partidas finalizadas del usuario. Si el usuario
  nunca terminó una partida, devuelve ceros (no `404`).
- `GET /statistics/deck/{id}`: lo mismo por deck, más
  `highest_life_total_achieved` (recalculado repitiendo el log de acciones
  desde el baseline de 40, no solo el `life_total` final). Requiere
  ownership del deck (404 si no es del usuario autenticado).
- `GET /statistics/playgroup/{id}`: agregación **en vivo** (no hay tabla de
  resumen por grupo) sobre las partidas finalizadas de ese playgroup:
  partidas jugadas/ganadas por miembro, mismo criterio de "único
  sobreviviente" para el ganador.
- **Límite conocido documentado en el backend**: `total_eliminations` solo
  cuenta acciones `Elimination` explícitas con un target distinto del
  actor — las auto-eliminaciones por vida/veneno (la forma más común de
  terminar en Commander) no quedan atribuidas a un actor específico en el
  log, así que no suman a las estadísticas de eliminaciones de nadie.

**Divergencia clave:** "ver estadísticas" hoy en Android es en realidad "ver
historial local de partidas" (sin agregación, sin usuario, sin red); el
backend ya tiene motor de estadísticas agregadas reales por usuario/deck/
grupo, pero es funcionalidad completamente invisible para quien usa la app
móvil hasta que se conecte Android al backend (Stage 5) y se construya la UI
correspondiente (pendiente también en Stage 7).

---

## Resumen de la brecha Android ↔ Backend

| Caso de uso | Backend (`internal/games`, `internal/game-actions`, `internal/statistics`) | Android (hoy) |
|---|---|---|
| Crear partida | `POST /games` → `pending`, multi-usuario, requiere auth | Local, un solo dispositivo, sin auth, `gameId` = UUID aleatorio |
| Unirse | `POST /games/{id}/join`, ownership de deck, scoping por `pending` | No existe: "unirse" = agregar un jugador más en el setup local |
| Trackear vida | `POST /games/{id}/actions`, timeline auditable, auto-eliminación server-side | En memoria (`GameViewModel`), sin log de acciones, sin red |
| Finalizar | `POST /games/{id}/finish`, solo desde `active`, ganador derivado post-hoc | Botón manual o automático, ganador decidido en el cliente |
| Ver estadísticas | `GET /statistics/{user,deck,playgroup}/{id}`, agregados reales | `HistoryScreen`, historial crudo de Room, sin agregación |

La unificación de ambas columnas es, según `docs/roadmap/TASKS.md` ("Orden
de trabajo sugerido", punto 6), la mayor brecha pendiente del proyecto:
capas de dominio/datos en Android, autenticación real, y `CommanderApi.kt`
con los endpoints reales de `games`/`game-actions`/`statistics`.
