# Wireframes de las pantallas Android

Wireframes en texto/ASCII de las seis pantallas reales del cliente Android,
más la jerarquía de componentes y los elementos interactivos de cada una.
Basados en el Compose real de cada pantalla (`android/app/src/main/java/com/
commandercompanion/presentation/screens/`), no en un diseño aspiracional —
si algo no está en el código, no aparece acá.

No hay mockups visuales (colores exactos, tipografía, spacing en dp más allá
de lo que dice el código) porque esta sesión es puramente de documentación;
para eso hay que abrir el proyecto en Android Studio con el Compose
Preview. Este documento sirve para entender *qué* hay en cada pantalla y
*cómo* se relacionan sus elementos, no *cómo se ve* pixel a pixel.

---

## 1. `LoginScreen`

**Archivo:** `presentation/screens/login/LoginScreen.kt`
**Ruta:** `LoginRoute` (destino inicial del `NavHost`)

```
┌─────────────────────────────────────┐
│                                       │
│                                       │
│         Commander Companion          │  ← headlineMedium, centrado
│                                       │
│  ┌─────────────────────────────────┐ │
│  │ Email                            │ │  ← OutlinedTextField
│  └─────────────────────────────────┘ │
│  ┌─────────────────────────────────┐ │
│  │ Contraseña            ●●●●●●●●   │ │  ← OutlinedTextField, oculta
│  └─────────────────────────────────┘ │
│  (error de login, si lo hay)          │  ← Text, color error, bodySmall
│  ┌─────────────────────────────────┐ │
│  │  INICIAR SESIÓN / ⟳ (loading)    │ │  ← Button, fillMaxWidth, 56dp
│  └─────────────────────────────────┘ │
│                                       │
│  ──────────────  o  ──────────────  │  ← HorizontalDivider + texto
│                                       │
│  ┌─────────────────────────────────┐ │
│  │     Continuar con Google         │ │  ← OutlinedButton, fillMaxWidth
│  └─────────────────────────────────┘ │
│                                       │
└─────────────────────────────────────┘
```

**Jerarquía:** `Column` (centrado vertical y horizontal, padding 24dp) →
título → spacer → `OutlinedTextField` email → `OutlinedTextField` password
(`PasswordVisualTransformation`) → mensaje de error condicional
(`uiState.error`) → `Button` sólido (con `CircularProgressIndicator` en vez
de texto mientras `uiState.isLoading`) → separador (`Row` con 2
`HorizontalDivider` + texto "o") → `OutlinedButton` outline. Ambos campos y
ambos botones se deshabilitan (`enabled = !uiState.isLoading`) durante el
login.

**Interactivo:**
- Campo Email (texto libre, `singleLine`).
- Campo Contraseña (texto oculto, `singleLine`).
- Botón **"INICIAR SESIÓN"** → `LoginViewModel.loginWithPassword(email,
  password)` → `POST /auth/login` real.
- Botón **"Continuar con Google"** → `LoginViewModel.loginWithGoogle(context)`
  → Credential Manager → `POST /auth/google` real.
- `LaunchedEffect(uiState.loginSucceeded)` dispara `onLoginSuccess()` (un solo
  callback, no uno por método de login) apenas cualquiera de los dos
  autentica correctamente.

**Nota de fidelidad (actualizada 2026-07-27):** este ya NO es un shell de
navegación — ambos botones autentican de verdad contra el backend real vía
`LoginViewModel` (antes navegaban directo a `DashboardRoute` sin llamar a
nada; ver `docs/roadmap/TASKS.md`, Stage 4, para el historial del cambio).

---

## 2. `DashboardScreen`

**Archivo:** `presentation/screens/dashboard/DashboardScreen.kt`
**Ruta:** `DashboardRoute`

```
┌─────────────────────────────────────┐
│                                       │
│                                       │
│       Commander Companion            │  ← displayLarge, centrado
│                                       │
│  ┌─────────────────────────────────┐ │
│  │           NEW GAME                │ │  ← Button, fillMaxWidth, 64dp
│  └─────────────────────────────────┘ │
│  ┌─────────────────────────────────┐ │
│  │          HISTORIAL                │ │  ← OutlinedButton, fillMaxWidth, 48dp
│  └─────────────────────────────────┘ │
│                                       │
│           Cerrar sesión               │  ← TextButton
│                                       │
└─────────────────────────────────────┘
```

**Jerarquía:** `Column` (centrado, padding 16dp) → título → `Button` "NEW
GAME" → `OutlinedButton` "HISTORIAL" → `TextButton` "Cerrar sesión". Sigue
siendo la pantalla más simple del proyecto: sin barra superior, sin menú, sin
estado propio más allá del logout.

**Interactivo:**
- Botón **"NEW GAME"** → `onNewGame()` → navega a `PlayerSetupRoute`.
- Botón **"HISTORIAL"** → `onViewHistory()` → navega a `HistoryRoute`.
- `TextButton` **"Cerrar sesión"** → `DashboardViewModel.logout(onLogout)`:
  revoca el refresh token contra `POST /auth/logout` (best-effort), limpia
  `SessionManager`/`clearCredentialState` de Google y vuelve a `LoginRoute`.

**Nota de fidelidad (actualizada 2026-07-27):** sigue sin haber saludo
personalizado ni avatar — pero ya no es cierto que "no hay ningún dato que
dependa de sesión": el botón de logout sí, y su existencia es la prueba de
que ahora hay una sesión real que cerrar (login real, ver `LoginScreen`).

---

## 3. `PlayerSetupScreen`

**Archivo:** `presentation/screens/setup/PlayerSetupScreen.kt`
**ViewModel:** `PlayerSetupViewModel` (carga `playgroups` propios y, on-demand,
los decks de un miembro vía `PlaygroupRepository`, cacheados por `userId`)
**Ruta:** `PlayerSetupRoute`

**Actualizado 2026-07-28** — la pantalla gana un selector **Casual / Grupo**
(`SetupMode`, privado) antes del selector de cantidad de jugadores. Cambia
solo cómo se completa cada asiento, no el resto del flujo (`PreGameScreen`/
`GameTrackerScreen` no distinguen el modo — solo ven la lista final de
`PlayerConfig`, ver `PlayerConfigCodec`).

### Modo Casual (default)

Sin red ni cuentas — el life tracker de siempre, campo de nombre libre:

```
┌─────────────────────────────────────┐
│ Nueva partida                        │  ← headlineMedium
│                                       │
│ [Casual●] [ Grupo ]                  │  ← FilterChip x2, selector de modo
│ "Sin cuentas ni estadísticas: solo   │  ← bodySmall, explica el modo activo
│  trackear la partida en este         │
│  dispositivo."                       │
│                                       │
│ Jugadores                            │  ← titleSmall
│ [ 2 ] [ 3 ] [4●] [ 5 ] [ 6 ]         │  ← FilterChip x5 (2..6), uno seleccionado
│                                       │
│ ┌───────────────────────────────┐ ↕  │
│ │ Nombre: [Jugador 1        ]    │ │  │  ← OutlinedTextField
│ │ ●W ●U ●B ●R ●G ●C (colorless)  │ │  │  ← swatches circulares, uno con borde
│ ├───────────────────────────────┤ │  │
│ │ Nombre: [Jugador 2        ]    │ │  │  LazyColumn, weight(1f)
│ │ ●W ●U ●B ●R ●G ●C              │ │  │  (una fila por jugador, tantas
│ ├───────────────────────────────┤ │  │   como playerCount)
│ │ ...                            │ ↕  │
│ └───────────────────────────────┘    │
│  ┌─────────────────────────────────┐ │
│  │        EMPEZAR PARTIDA           │ │  ← Button, fillMaxWidth, 56dp
│  └─────────────────────────────────┘ │
└─────────────────────────────────────┘
```

### Modo Grupo

Reemplaza el campo de nombre libre por asignación a un miembro real del
playgroup elegido, con su deck. Agrega un `PlaygroupPicker` (`LazyRow` de
`FilterChip`, uno por playgroup propio) arriba de la lista de asientos:

```
┌─────────────────────────────────────┐
│ Nueva partida                        │
│ [ Casual ] [Grupo●]                  │
│ "Asigná asientos a miembros de tu    │
│  grupo: sus estadísticas quedan      │
│  reales al terminar."                │
│                                       │
│ Grupo                                │  ← titleSmall
│ [MiMesa●] [ Otro grupo ]             │  ← PlaygroupPicker, LazyRow
│                                       │
│ Jugadores                            │
│ [ 2 ] [ 3 ] [4●] [ 5 ] [ 6 ]         │
│                                       │
│ ┌───────────────────────────────┐ ↕  │
│ │ Asiento                        │ │  │  ← MemberPicker en vez de nombre libre
│ │ [Invitado●] [vos] [ana] [bea]  │ │  │     ("Invitado" = casilla local, sin
│ │ ●W ●U ●B ●R ●G ●C              │ │  │      GamePlayer remoto ni proxy-join)
│ ├───────────────────────────────┤ │  │
│ │ Asiento                        │ │  │  LazyColumn, weight(1f)
│ │ [Invitado] [vos●] [ana] [bea]  │ │  │
│ │ ●W ●U ●B ●R ●G ●C              │ │  │
│ │ ¿Con qué deck juega?           │ │  │  ← solo si hay miembro asignado
│ │ [Atraxa●] [Muldrotha]          │ │  │     ("todavía no tiene decks" si
│ ├───────────────────────────────┤ │  │      la lista viene vacía)
│ │ ...                            │ ↕  │
│ └───────────────────────────────┘    │
│  ┌─────────────────────────────────┐ │
│  │        EMPEZAR PARTIDA           │ │
│  └─────────────────────────────────┘ │
└─────────────────────────────────────┘
```

**Jerarquía:** `Column` (padding 16dp) → título → selector Casual/Grupo
(`Row` de 2 `FilterChip`) → texto explicativo → (solo Grupo) `PlaygroupPicker`
→ subtítulo "Jugadores" → `Row` de `FilterChip` (2..6, `MIN_PLAYERS`/
`MAX_PLAYERS`) → `LazyColumn` (weight 1f) con `PlayerConfigRow` por cada
jugador activo → `Button` final.

Cada `PlayerConfigRow` (privado) es un `Column` con, según el modo:
- **Casual:** `OutlinedTextField` de nombre (default `"Jugador N"`).
- **Grupo:** `MemberPicker` (`LazyRow` de `FilterChip`: "Invitado" + un chip
  por miembro disponible del grupo — un miembro ya asignado a otro asiento
  desaparece de la lista; el propio usuario se marca "(vos)"). Si hay un
  miembro asignado y tiene decks, agrega un segundo `LazyRow` de `FilterChip`
  para elegir con cuál juega.
- En ambos modos: `Row` de `ColorSwatch` (`Box` circular clicable, uno por
  color de `PlayerColorPalette` — paleta WUBRG + incoloro), con borde de 3dp
  en el color seleccionado.

**Interactivo:**
- 2 `FilterChip` (Casual/Grupo) — cambia el modo; no resetea `playerCount`
  ni colores, sí limpia las asignaciones de miembro/deck al cambiar de grupo.
- 5 `FilterChip` (2 a 6 jugadores) — cambia `playerCount`, lo que
  agranda/achica la `LazyColumn` de abajo.
- Por cada jugador visible: nombre libre (Casual) o `MemberPicker` +
  `FilterChip` de deck (Grupo) + 6 swatches de color clicables (selección
  única).
- Botón **"EMPEZAR PARTIDA"** → arma la lista de `PlayerConfig`
  (`name`/`colorKey`, más `assignedUserId`/`assignedUsername`/`deckId` si el
  asiento tiene un miembro de grupo asignado), genera un `gameId` local
  (`UUID.randomUUID()`) y navega pasando también el `playgroupId` elegido
  (`null` en modo Casual) — es lo que `GameRepository.bootstrapRemoteGame`
  usa para decidir self-join vs. proxy-join por asiento (ver
  [ADR-0013](../decisions/0013-proxy-join-y-autorizacion-de-acciones.md)).

---

## 4. `PreGameScreen`

**Archivo:** `presentation/screens/pregame/PreGameScreen.kt`
**Ruta:** `PreGameRoute(gameId, playersEncoded)`

```
┌─────────────────────────────────────┐
│ Antes de empezar                     │  ← headlineMedium
│                                       │
│ ¿Quién empieza?                      │  ← titleSmall
│ ┌───────────────────────────────────┐│
│ │                                   ││  ← Card, 80dp de alto
│ │      Empieza Jugador 3            ││     color = color del ganador del
│ │   (o "Sin sortear todavía")       ││     sorteo, o surfaceVariant si
│ │                                   ││     todavía no se sorteó
│ └───────────────────────────────────┘│
│  ┌─────────────────────────────────┐ │
│  │            SORTEAR                │ │  ← OutlinedButton
│  └─────────────────────────────────┘ │
│                                       │
│ Mulligans                             │  ← titleSmall
│ ┌───────────────────────────────────┐│
│ │ ●  Jugador 1        [-]  0  [+]   ││  ← LazyColumn, una fila por jugador
│ │ ●  Jugador 2        [-]  1  [+]   ││     (dot de color + nombre + stepper)
│ │ ●  Jugador 3        [-]  0  [+]   ││
│ │ ●  Jugador 4        [-]  2  [+]   ││
│ └───────────────────────────────────┘│
│                                       │
│  ┌─────────────────────────────────┐ │
│  │        EMPEZAR PARTIDA            │ │  ← Button, fillMaxWidth, 56dp
│  └─────────────────────────────────┘ │
└─────────────────────────────────────┘
```

**Jerarquía:** `Column` (padding 16dp) → título → sección "¿Quién empieza?"
(`Card` de resultado + `OutlinedButton` "SORTEAR") → sección "Mulligans"
(`LazyColumn` de `MulliganRow`) → `Button` final.

`MulliganRow` (privado): `Row` con dot circular de color, nombre
(`weight(1f)`), y un mini-stepper (`StepperButton "-"`, contador, `
StepperButton "+"`).

**Interactivo:**
- Botón **"SORTEAR"** → elige un índice al azar (`Random.nextInt`) entre los
  jugadores configurados; se puede volver a tocar para re-sortear.
- Por cada jugador: botones `-`/`+` de mulligans (mínimo 0, sin tope).
- Botón **"EMPEZAR PARTIDA"** → adjunta los mulligans finales a cada
  `PlayerConfig` y navega a `GameTrackerRoute` con el asiento ganador del
  sorteo (`startingPlayerSeat`).

**Nota de fidelidad:** no hay validación que obligue a sortear antes de
continuar — se puede tocar "EMPEZAR PARTIDA" con `startingSeat = -1` (nadie
"empieza" marcado) sin ningún bloqueo ni advertencia.

---

## 5. `GameTrackerScreen`

**Archivo:** `presentation/screens/game/GameTrackerScreen.kt` (+
`presentation/components/PlayerCard.kt`)
**Ruta:** `GameTrackerRoute`

### Estado normal (4 jugadores, grid 2×2)

```
┌─────────────────────────────────────┐
│ [<]      Turn: 3       [>] [Finalizar]│  ← header: Row SpaceBetween
│ Creando la partida en el servidor…    │  ← RemoteSyncBanner (condicional,
├───────────────────┬───────────────────┤     ver nota debajo del diagrama)
│                   │                   │
│ Jugador 1 · empieza│    Jugador 2      │  ← PlayerCard (color de fondo
│                   │                   │     = color del jugador)
│   [-]   38   [+]   │   [-]   40   [+]  │
│                   │                   │
│  Commander Damage │  Commander Damage │  ← hint (tocar para ver panel)
├───────────────────┼───────────────────┤
│                   │                   │
│    Jugador 3      │    Jugador 4      │
│  Mulligans: 1     │                   │  ← badge solo si mulligans > 0
│   [-]   35   [+]   │   [-]   22   [+]  │
│                   │                   │
│  Commander Damage │  Commander Damage │
└───────────────────┴───────────────────┘
```
`state.players.chunked(2)` arma filas de a 2: 2 jugadores → 1 fila, 4 → 2
filas, 5-6 → 3 filas (la última con 1 o 2 cartas). Cada `PlayerCard` tiene
`weight(1f)` tanto horizontal como verticalmente, así que el grid se reparte
la pantalla completa sin importar la cantidad de jugadores.

### Panel de daño de comandante (al tocar una `PlayerCard`)

```
┌───────────────────┐
│ ████████████████  │  ← overlay negro 80% opacidad sobre toda la card
│  Commander Damage  │
│                    │
│  ●3     ●5    ●0   │  ← CommanderDamageItem x (N-1 oponentes), grid 3 col
│ [-][+] [-][+] [-][+]│     dot = color del atacante, número = daño acumulado
│                    │
└───────────────────┘
```

**Jerarquía completa:**
`Column` raíz →
1. Header `Row` (SpaceBetween): `Button "<"` (turno -1) — `Text "Turn: N"` —
   `Row` (`Button ">"` turno +1, `OutlinedButton "Finalizar"`).
2. **`RemoteSyncBanner` (nuevo, 2026-07-27)**: franja de texto de ancho
   completo, condicional — no ocupa espacio si `remoteSync.status ==
   Synced` (caso silencioso, todo salió bien) o no hay mensaje. Con
   `Connecting`/`Disabled`/`WaitingForPlayers` muestra el mensaje en
   `surfaceVariant`; con `Failed`, en `errorContainer` (rojo). Ver caso de
   uso 1 en `docs/ux/casos-de-uso.md` para qué dispara cada estado.
3. `Column` (weight 1f) con filas (`Row`, weight 1f) de `PlayerCard`
   (weight 1f cada una).
4. Cada `PlayerCard` (`Surface` clicable, color = color del jugador) →
   `Column` centrado: nombre (+ "· empieza" si aplica) → badge de mulligans
   condicional → `Row` de vida (`IconButton "-"`, número grande, `IconButton
   "+"`) → hint "Commander Damage" (solo si el panel no está abierto).
   Si `showCommanderDamage` es true, se reemplaza por un `Surface` overlay
   con grid de `CommanderDamageItem` (dot de color del atacante, número,
   `IconButton -`/`+`) por cada oponente.
5. Condicional: `AlertDialog` de confirmación de "Finalizar" (si
   `showFinishConfirm`).
6. Condicional: `AlertDialog` final de resultado (si `state.isFinished`) —
   título con el ganador o "Partida finalizada", lista de vida final de cada
   jugador, botón "Volver al inicio".

**Interactivo:**
- `<` / `>` del header: turno -1 / +1 (mínimo 1).
- **"Finalizar"** (header): abre diálogo de confirmación.
- Por `PlayerCard`: tocar la tarjeta entera alterna el panel de daño de
  comandante; `-`/`+` de vida; si el panel está abierto, `-`/`+` de daño por
  cada oponente.
- Diálogo de confirmación: "Finalizar" (confirma) / "Cancelar".
- Diálogo final: "Volver al inicio" → vuelve a `DashboardRoute`.

---

## 6. `HistoryScreen`

**Archivo:** `presentation/screens/history/HistoryScreen.kt`
**Ruta:** `HistoryRoute`

```
┌─────────────────────────────────────┐
│ [<]   Historial de partidas          │  ← TopAppBar
├─────────────────────────────────────┤
│ ┌─────────────────────────────────┐ │
│ │ 26/07/2026 18:40      Finalizada │ │  ← Row SpaceBetween (fecha / estado)
│ │ Ganó: Jugador 1                   │ │  ← titleMedium
│ │ ● Jugador 1: 12   ● Jugador 2: 0  │ │  ← dots de color + nombre + vida
│ │ ● Jugador 3: 0 (1m) ● Jugador 4: 0│ │     final (+ sufijo mulligans)
│ └─────────────────────────────────┘ │
│ ┌─────────────────────────────────┐ │
│ │ 25/07/2026 21:05      En curso   │ │
│ │ 4 jugadores                       │ │  ← si no hay ganador (won=false
│ │ ● J1: 40  ● J2: 40  ● J3: 40 ...  │ │     para todos), muestra cantidad
│ └─────────────────────────────────┘ │     de jugadores en vez de "Ganó: X"
│              ⋮                       │
└─────────────────────────────────────┘

(si no hay partidas)
┌─────────────────────────────────────┐
│ [<]   Historial de partidas          │
├─────────────────────────────────────┤
│                                       │
│   Todavía no hay partidas registradas│  ← centrado, bodyLarge
│                                       │
└─────────────────────────────────────┘
```

**Jerarquía:** `Column` → `TopAppBar` (título + `TextButton "<"` como
navigationIcon) → si `games.isEmpty()`: `Box` centrado con mensaje; si no,
`LazyColumn` de `GameHistoryCard` (`key = game.id`).

Cada `GameHistoryCard` (`Card`) → `Column`: `Row` SpaceBetween (fecha
formateada `dd/MM/yyyy HH:mm` — `Text` de estado "Finalizada"/"En curso") →
`Text` de resultado ("Ganó: {nombre}" si hay `won == true`, o "{N}
jugadores" si no) → `Row` de jugadores ordenados por `seatIndex` (dot de
color + "{nombre}: {vida final}" + sufijo `(Nm)` si tuvo mulligans).

**Interactivo:**
- `TextButton "<"` en la `TopAppBar` → `onBack()` → `popBackStack()`.
- La lista es de solo lectura: no hay swipe-to-delete, no hay tap-to-expand,
  no hay filtros ni búsqueda.

**Nota de fidelidad:** los datos vienen 100% de Room
(`gameDao.getGamesWithPlayers()`), local al dispositivo — no hay ningún
indicador de sincronización porque no hay sincronización.

---

## Resumen de navegación entre wireframes

Ver también `docs/diagrams/android-navigation-flow.md` para el grafo
completo con las rutas y sus argumentos.

```
LoginScreen → DashboardScreen ─┬─→ PlayerSetupScreen → PreGameScreen → GameTrackerScreen ─┐
                                │                                                          │
                                └─→ HistoryScreen                    (vuelve a) ───────────┘
                                              ↑______________________________________________|
```
