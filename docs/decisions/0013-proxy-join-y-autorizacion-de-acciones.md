# ADR-0013: Proxy-join y autorización de acciones por `game_players.added_by`

**Estado:** Aceptada (2026-07-28)

## Contexto

El modelo de partidas del backend asume que cada `GamePlayer` se crea a
partir de la sesión JWT de quien se une (`POST /games/{id}/join` toma el
`user_id` siempre del token, nunca del body). Eso funciona para un cliente
web multi-dispositivo, pero el cliente Android es pass-and-play en un solo
dispositivo (ver el comentario de diseño en
`android/.../data/repository/GameRepository.kt`): un único usuario
autenticado por partida, y el resto de los asientos locales nunca tuvieron
`GamePlayer` propio ni estadísticas.

Se pidió una forma de que ese único dispositivo pueda anotar una partida
completa para un grupo de juego real (`playgroups`) — que **todos** los
asientos asignados a miembros del grupo generen estadísticas de verdad, no
solo el dueño de la sesión. Eso exige que un usuario autenticado pueda, en
nombre de otro, (a) unirse a una partida y (b) registrar sus acciones
(vida, daño de comandante, etc.).

Investigando el punto (b) se encontró un hueco de autorización preexistente
e independiente de este pedido: **`POST /games/{id}/actions`
(`internal/game-actions/handler.go: CreateAction`) nunca lee el `user_id`
del JWT** — el `actor_id` del body se acepta tal cual, sin verificar que
pertenezca al caller. Hoy, cualquier usuario autenticado que conozca un
`game_id` y un `GamePlayer.id` (ambos visibles vía `GET /games/{id}`) puede
registrar acciones en su nombre. Este cambio cierra ese hueco como parte
necesaria de la misma función que hay que tocar para agregar el permiso de
proxy.

## Decisión

### Columna nueva: `game_players.added_by`

`uuid null references users(id)` (migración
`00012_game_player_proxy_join.sql`). `null` si el jugador se unió con su
propia sesión (comportamiento de siempre). Si no es null, es el `user_id`
de quien lo unió como proxy — y ese usuario queda autorizado a actuar en su
nombre.

### Proxy-join: `POST /games/{id}/join` con `user_id` opcional

`JoinGameRequest` gana `user_id` opcional. Si viene y coincide con el
caller, comportamiento idéntico a hoy (`added_by` queda null). Si viene y
es **distinto** del caller (`internal/games/service.go: JoinGame`), se
exige:

1. La partida tiene `playgroup_id` (no es una partida Casual).
2. El caller es miembro de ese `playgroup_id`.
3. El `user_id` destino también es miembro de ese mismo `playgroup_id`.
4. El `deck_id` del body pertenece al **destino**, no al caller
   (`resolveOwnedDeckID` ya validaba esto contra un `userID` — solo cambia
   cuál se le pasa).

Si algo de esto falla, mismo criterio que el resto del módulo
(`ErrPlaygroupNotFound`/`ErrDeckNotFound` genéricos, sin distinguir "no
existe" de "no tenés permiso" — evita revelar membresías o decks ajenos).

Se ancla a `playgroup_id` (no a "cualquier par de usuarios que se conozcan")
a propósito: usar el mismo campo que ya existe en `games` mantiene la
superficie de autorización acotada a grupos reales, sin inventar una
relación de confianza nueva.

### Descubrir los decks del destino: `GET /playgroups/{id}/members/{userId}/decks`

Sin esto no hay de dónde elegir el `deck_id` del proxy-join. Mismo criterio
de autorización que el proxy-join (caller y destino miembros del mismo
grupo). Vive bajo `/playgroups` (no bajo `/decks`) porque la autorización
depende enteramente de la membresía compartida, no de una relación directa
entre los dos usuarios.

### Proxy-record: autorización en `POST /games/{id}/actions`

`game-actions/handler.go: CreateAction` empieza a leer `userID` de
`c.Locals` y se lo pasa a `RecordAction`. `resolveActionSubject` valida,
tras resolver el `actor` (`GamePlayer`):

```
autorizado := actor.UserID == callerID || actor.AddedBy == callerID
```

Si no, `403` (`ErrNotAuthorizedForActor`, nuevo). Esto es estrictamente más
estricto que el comportamiento actual (que no valida nada), así que no rompe
ningún flujo legítimo existente — todo `actor_id` que hoy se manda
corresponde siempre al propio caller en la práctica (el cliente Android
nunca mandó uno ajeno).

## Alternativas consideradas

- **Delegar todo a nivel de partida** (si el caller tiene *algún*
  `GamePlayer` en esa partida, puede actuar por cualquier otro asiento de
  la misma partida): más simple de implementar, pero cualquier jugador de
  la mesa podría alterar las estadísticas de cualquier otro con solo estar
  sentado — demasiado permisivo para algo que persiste estadísticas reales.
  `added_by` acota la autoridad a "quien efectivamente lo unió", que es
  quien sostiene el dispositivo.
- **Relación de confianza persistente entre usuarios** ("delegados"), en vez
  de derivarla de `added_by` por partida: más flexible (sobreviviría a
  partidas puntuales), pero es una tabla y un flujo de invitación/aceptación
  nuevos para un caso de uso — anotar la mesa de tu propio grupo — que ya
  tiene una señal de confianza natural y suficiente (`playgroup_members`).
  Se descarta hasta que haga falta algo más granular.
- **No cerrar el hueco de `POST /games/{id}/actions` en esta pasada**
  (dejarlo para un ticket aparte): se descarta porque la función que hay que
  tocar para agregar el permiso de proxy es exactamente la misma que hoy no
  valida nada — arreglarlo ahora es estrictamente más barato que hacerlo en
  dos pasadas, y dejarlo abierto a sabiendas ya no sería un descuido sino
  una decisión consciente de shippear con una vulnerabilidad conocida.

## Consecuencias

- `game_players.added_by` es la única fuente de verdad de "quién puede
  actuar por quién" — no hay revocación explícita (si la partida termina o
  el proxy-joiner deja el grupo, la autorización sigue existiendo para esa
  partida puntual, ya finalizada, donde no importa).
- El cliente Android (modo Grupo) es el primer y único llamador real de
  proxy-join hasta que exista un cliente multi-dispositivo (Stage 6); el
  cliente web no lo usa todavía.
- Cualquier extensión futura de "quién puede ver/actuar por quién" (p. ej.
  roles dentro de un grupo) debería revisar si `playgroup_members` sigue
  alcanzando o hace falta un modelo de permisos más rico — no está resuelto
  acá, solo lo mínimo para este caso de uso.

## Referencias

- `backend/migrations/00012_game_player_proxy_join.sql`
- `backend/internal/games/service.go` (`JoinGame`)
- `backend/internal/game-actions/service.go` (`RecordAction`,
  `resolveActionSubject`)
- `backend/internal/playgroups/service.go` (membresía compartida)
- `android/app/src/main/java/com/commandercompanion/data/repository/GameRepository.kt`
  (comentario de diseño sobre el modelo pass-and-play de un dispositivo)
- [ADR-0001](0001-auth-jwt-refresh-token-strategy.md) (JWT, base de
  `common.UserIDKey`)
