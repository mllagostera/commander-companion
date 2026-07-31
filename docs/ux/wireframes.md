# Wireframes of the Android screens

Text/ASCII wireframes of the six real screens of the Android client, plus
the component hierarchy and the interactive elements of each. Based on the
actual Compose code of each screen (`android/app/src/main/java/com/
commandercompanion/presentation/screens/`), not on an aspirational design —
if something isn't in the code, it doesn't appear here.

There are no visual mockups (exact colors, typography, spacing in dp beyond
what the code says) because this session is purely for documentation
purposes; for that you have to open the project in Android Studio with the
Compose Preview. This document serves to understand *what* is on each
screen and *how* its elements relate to each other, not *what it looks
like* pixel by pixel.

---

## 1. `LoginScreen`

**File:** `presentation/screens/login/LoginScreen.kt`
**Route:** `LoginRoute` (initial destination of the `NavHost`)

```
┌─────────────────────────────────────┐
│                                       │
│                                       │
│         Commander Companion          │  ← headlineMedium, centered
│                                       │
│  ┌─────────────────────────────────┐ │
│  │ Email                            │ │  ← OutlinedTextField
│  └─────────────────────────────────┘ │
│  ┌─────────────────────────────────┐ │
│  │ Password               ●●●●●●●●  │ │  ← OutlinedTextField, hidden
│  └─────────────────────────────────┘ │
│  (login error, if any)                │  ← Text, error color, bodySmall
│  ┌─────────────────────────────────┐ │
│  │  SIGN IN / ⟳ (loading)           │ │  ← Button, fillMaxWidth, 56dp
│  └─────────────────────────────────┘ │
│                                       │
│  ──────────────  or  ──────────────  │  ← HorizontalDivider + text
│                                       │
│  ┌─────────────────────────────────┐ │
│  │     Continue with Google         │ │  ← OutlinedButton, fillMaxWidth
│  └─────────────────────────────────┘ │
│                                       │
└─────────────────────────────────────┘
```

**Hierarchy:** `Column` (vertically and horizontally centered, 24dp
padding) → title → spacer → `OutlinedTextField` email → `OutlinedTextField`
password (`PasswordVisualTransformation`) → conditional error message
(`uiState.error`) → solid `Button` (with `CircularProgressIndicator` instead
of text while `uiState.isLoading`) → separator (`Row` with 2
`HorizontalDivider` + text "or") → outline `OutlinedButton`. Both fields and
both buttons are disabled (`enabled = !uiState.isLoading`) during login.

**Interactive:**
- Email field (free text, `singleLine`).
- Password field (hidden text, `singleLine`).
- **"SIGN IN"** button → `LoginViewModel.loginWithPassword(email,
  password)` → real `POST /auth/login`.
- **"Continue with Google"** button →
  `LoginViewModel.loginWithGoogle(context)` → Credential Manager → real
  `POST /auth/google`.
- `LaunchedEffect(uiState.loginSucceeded)` triggers `onLoginSuccess()` (a
  single callback, not one per login method) as soon as either method
  authenticates successfully.

**Fidelity note (updated 2026-07-27):** this is NO LONGER a navigation
shell — both buttons authenticate for real against the actual backend via
`LoginViewModel` (previously they navigated straight to `DashboardRoute`
without calling anything; see `docs/roadmap/TASKS.md`, Stage 4, for the
change history).

---

## 2. `DashboardScreen`

**File:** `presentation/screens/dashboard/DashboardScreen.kt`
**Route:** `DashboardRoute`

```
┌─────────────────────────────────────┐
│                                       │
│                                       │
│       Commander Companion            │  ← displayLarge, centered
│                                       │
│  ┌─────────────────────────────────┐ │
│  │           NEW GAME                │ │  ← Button, fillMaxWidth, 64dp
│  └─────────────────────────────────┘ │
│  ┌─────────────────────────────────┐ │
│  │          HISTORY                  │ │  ← OutlinedButton, fillMaxWidth, 48dp
│  └─────────────────────────────────┘ │
│                                       │
│           Sign out                    │  ← TextButton
│                                       │
└─────────────────────────────────────┘
```

**Hierarchy:** `Column` (centered, 16dp padding) → title → "NEW GAME"
`Button` → "HISTORY" `OutlinedButton` → "Sign out" `TextButton`. It remains
the simplest screen in the project: no top bar, no menu, no state of its
own beyond logout.

**Interactive:**
- **"NEW GAME"** button → `onNewGame()` → navigates to `PlayerSetupRoute`.
- **"HISTORY"** button → `onViewHistory()` → navigates to `HistoryRoute`.
- **"Sign out"** `TextButton` → `DashboardViewModel.logout(onLogout)`:
  revokes the refresh token against `POST /auth/logout` (best-effort),
  clears `SessionManager`/Google's `clearCredentialState` and returns to
  `LoginRoute`.

**Fidelity note (updated 2026-07-27):** there is still no personalized
greeting or avatar — but it is no longer true that "there is no data that
depends on the session": the logout button does, and its existence is proof
that there is now a real session to close (real login, see `LoginScreen`).

---

## 3. `PlayerSetupScreen`

**File:** `presentation/screens/setup/PlayerSetupScreen.kt`
**ViewModel:** `PlayerSetupViewModel` (loads the user's own `playgroups`
and, on-demand, a member's decks via `PlaygroupRepository`, cached by
`userId`)
**Route:** `PlayerSetupRoute`

**Updated 2026-07-28** — the screen gains a **Casual / Group** selector
(`SetupMode`, private) before the player-count selector. It only changes
how each seat is filled in, not the rest of the flow (`PreGameScreen`/
`GameTrackerScreen` do not distinguish the mode — they only see the final
list of `PlayerConfig`, see `PlayerConfigCodec`).

### Casual mode (default)

No network or accounts — the usual life tracker, free-text name field:

```
┌─────────────────────────────────────┐
│ New game                             │  ← headlineMedium
│                                       │
│ [Casual●] [ Group ]                  │  ← FilterChip x2, mode selector
│ "No accounts or statistics: just     │  ← bodySmall, explains the active mode
│  tracking the game on this           │
│  device."                            │
│                                       │
│ Players                              │  ← titleSmall
│ [ 2 ] [ 3 ] [4●] [ 5 ] [ 6 ]         │  ← FilterChip x5 (2..6), one selected
│                                       │
│ ┌───────────────────────────────┐ ↕  │
│ │ Name: [Player 1           ]    │ │  │  ← OutlinedTextField
│ │ ●W ●U ●B ●R ●G ●C (colorless)  │ │  │  ← circular swatches, one with a border
│ ├───────────────────────────────┤ │  │
│ │ Name: [Player 2           ]    │ │  │  LazyColumn, weight(1f)
│ │ ●W ●U ●B ●R ●G ●C              │ │  │  (one row per player, as many
│ ├───────────────────────────────┤ │  │   as playerCount)
│ │ ...                            │ ↕  │
│ └───────────────────────────────┘    │
│  ┌─────────────────────────────────┐ │
│  │        START GAME                │ │  ← Button, fillMaxWidth, 56dp
│  └─────────────────────────────────┘ │
└─────────────────────────────────────┘
```

### Group mode

Replaces the free-text name field with assignment to a real member of the
chosen playgroup, with their deck. Adds a `PlaygroupPicker` (`LazyRow` of
`FilterChip`, one per own playgroup) above the seat list:

```
┌─────────────────────────────────────┐
│ New game                             │
│ [ Casual ] [Group●]                  │
│ "Assign seats to members of your     │
│  group: their statistics become      │
│  real once the game ends."           │
│                                       │
│ Group                                │  ← titleSmall
│ [MyTable●] [ Other group ]           │  ← PlaygroupPicker, LazyRow
│                                       │
│ Players                              │
│ [ 2 ] [ 3 ] [4●] [ 5 ] [ 6 ]         │
│                                       │
│ ┌───────────────────────────────┐ ↕  │
│ │ Seat                           │ │  │  ← MemberPicker instead of free-text name
│ │ [Guest●] [you] [ana] [bea]     │ │  │     ("Guest" = local-only slot, no
│ │ ●W ●U ●B ●R ●G ●C              │ │  │      remote GamePlayer or proxy-join)
│ ├───────────────────────────────┤ │  │
│ │ Seat                           │ │  │  LazyColumn, weight(1f)
│ │ [Guest] [you●] [ana] [bea]     │ │  │
│ │ ●W ●U ●B ●R ●G ●C              │ │  │
│ │ Which deck are they playing?   │ │  │  ← only if a member is assigned
│ │ [Atraxa●] [Muldrotha]          │ │  │     ("no decks yet" if
│ ├───────────────────────────────┤ │  │      the list comes back empty)
│ │ ...                            │ ↕  │
│ └───────────────────────────────┘    │
│  ┌─────────────────────────────────┐ │
│  │        START GAME                │ │
│  └─────────────────────────────────┘ │
└─────────────────────────────────────┘
```

**Hierarchy:** `Column` (16dp padding) → title → Casual/Group selector
(`Row` of 2 `FilterChip`) → explanatory text → (Group only)
`PlaygroupPicker` → "Players" subtitle → `Row` of `FilterChip` (2..6,
`MIN_PLAYERS`/`MAX_PLAYERS`) → `LazyColumn` (weight 1f) with a
`PlayerConfigRow` per active player → final `Button`.

Each `PlayerConfigRow` (private) is a `Column` with, depending on the mode:
- **Casual:** name `OutlinedTextField` (default `"Player N"`).
- **Group:** `MemberPicker` (`LazyRow` of `FilterChip`: "Guest" + one chip
  per available group member — a member already assigned to another seat
  disappears from the list; the user themselves is marked "(you)"). If a
  member is assigned and has decks, it adds a second `LazyRow` of
  `FilterChip` to choose which one they're playing.
- In both modes: `Row` of `ColorSwatch` (clickable circular `Box`, one per
  color of `PlayerColorPalette` — WUBRG + colorless palette), with a 3dp
  border on the selected color.

**Interactive:**
- 2 `FilterChip` (Casual/Group) — changes the mode; does not reset
  `playerCount` or colors, but does clear member/deck assignments when
  changing group.
- 5 `FilterChip` (2 to 6 players) — changes `playerCount`, which
  grows/shrinks the `LazyColumn` below.
- For each visible player: free-text name (Casual) or `MemberPicker` +
  deck `FilterChip` (Group) + 6 clickable color swatches (single
  selection).
- **"START GAME"** button → builds the list of `PlayerConfig`
  (`name`/`colorKey`, plus `assignedUserId`/`assignedUsername`/`deckId` if
  the seat has a group member assigned), generates a local `gameId`
  (`UUID.randomUUID()`) and navigates while also passing the chosen
  `playgroupId` (`null` in Casual mode) — this is what
  `GameRepository.bootstrapRemoteGame` uses to decide self-join vs.
  proxy-join per seat (see
  [ADR-0013](../decisions/0013-proxy-join-y-autorizacion-de-acciones.md)).

---

## 4. `PreGameScreen`

**File:** `presentation/screens/pregame/PreGameScreen.kt`
**Route:** `PreGameRoute(gameId, playersEncoded)`

```
┌─────────────────────────────────────┐
│ Before starting                      │  ← headlineMedium
│                                       │
│ Who starts?                          │  ← titleSmall
│ ┌───────────────────────────────────┐│
│ │                                   ││  ← Card, 80dp tall
│ │      Player 3 starts              ││     color = color of the draw
│ │   (or "Not drawn yet")            ││     winner, or surfaceVariant if
│ │                                   ││     not drawn yet
│ └───────────────────────────────────┘│
│  ┌─────────────────────────────────┐ │
│  │            DRAW                   │ │  ← OutlinedButton
│  └─────────────────────────────────┘ │
│                                       │
│ Mulligans                             │  ← titleSmall
│ ┌───────────────────────────────────┐│
│ │ ●  Player 1         [-]  0  [+]   ││  ← LazyColumn, one row per player
│ │ ●  Player 2         [-]  1  [+]   ││     (color dot + name + stepper)
│ │ ●  Player 3         [-]  0  [+]   ││
│ │ ●  Player 4         [-]  2  [+]   ││
│ └───────────────────────────────────┘│
│                                       │
│  ┌─────────────────────────────────┐ │
│  │        START GAME                 │ │  ← Button, fillMaxWidth, 56dp
│  └─────────────────────────────────┘ │
└─────────────────────────────────────┘
```

**Hierarchy:** `Column` (16dp padding) → title → "Who starts?" section
(result `Card` + "DRAW" `OutlinedButton`) → "Mulligans" section
(`LazyColumn` of `MulliganRow`) → final `Button`.

`MulliganRow` (private): `Row` with a circular color dot, name
(`weight(1f)`), and a mini-stepper (`StepperButton "-"`, counter,
`StepperButton "+"`).

**Interactive:**
- **"DRAW"** button → picks a random index (`Random.nextInt`) among the
  configured players; can be tapped again to redraw.
- For each player: mulligan `-`/`+` buttons (minimum 0, no cap).
- **"START GAME"** button → attaches the final mulligans to each
  `PlayerConfig` and navigates to `GameTrackerRoute` with the draw's
  winning seat (`startingPlayerSeat`).

**Fidelity note:** there is no validation forcing a draw before continuing
— you can tap "START GAME" with `startingSeat = -1` (no one marked as
"starting") with no block or warning.

---

## 5. `GameTrackerScreen`

**File:** `presentation/screens/game/GameTrackerScreen.kt` (+
`presentation/components/PlayerCard.kt`)
**Route:** `GameTrackerRoute`

### Normal state (4 players, 2×2 grid)

```
┌─────────────────────────────────────┐
│ [<]      Turn: 3       [>] [Finish]  │  ← header: Row SpaceBetween
│ Creating the game on the server…      │  ← RemoteSyncBanner (conditional,
├───────────────────┬───────────────────┤     see note below the diagram)
│                   │                   │
│ Player 1 · starts  │    Player 2      │  ← PlayerCard (background color
│                   │                   │     = player's color)
│   [-]   38   [+]   │   [-]   40   [+]  │
│                   │                   │
│  Commander Damage │  Commander Damage │  ← hint (tap to see panel)
├───────────────────┼───────────────────┤
│                   │                   │
│    Player 3       │    Player 4       │
│  Mulligans: 1     │                   │  ← badge only if mulligans > 0
│   [-]   35   [+]   │   [-]   22   [+]  │
│                   │                   │
│  Commander Damage │  Commander Damage │
└───────────────────┴───────────────────┘
```
`state.players.chunked(2)` builds rows of 2: 2 players → 1 row, 4 → 2
rows, 5-6 → 3 rows (the last one with 1 or 2 cards). Each `PlayerCard` has
`weight(1f)` both horizontally and vertically, so the grid fills the whole
screen regardless of the number of players.

### Commander damage panel (on tapping a `PlayerCard`)

```
┌───────────────────┐
│ ████████████████  │  ← black overlay 80% opacity over the whole card
│  Commander Damage  │
│                    │
│  ●3     ●5    ●0   │  ← CommanderDamageItem x (N-1 opponents), 3-col grid
│ [-][+] [-][+] [-][+]│     dot = attacker's color, number = accumulated damage
│                    │
└───────────────────┘
```

**Full hierarchy:**
root `Column` →
1. Header `Row` (SpaceBetween): `Button "<"` (turn -1) — `Text "Turn: N"` —
   `Row` (`Button ">"` turn +1, `OutlinedButton "Finish"`).
2. **`RemoteSyncBanner` (new, 2026-07-27)**: full-width text strip,
   conditional — it takes up no space if `remoteSync.status == Synced`
   (silent case, everything went fine) or there is no message. With
   `Connecting`/`Disabled`/`WaitingForPlayers` it shows the message in
   `surfaceVariant`; with `Failed`, in `errorContainer` (red). See use case
   1 in `docs/ux/casos-de-uso.md` for what triggers each state.
3. `Column` (weight 1f) with rows (`Row`, weight 1f) of `PlayerCard`
   (weight 1f each).
4. Each `PlayerCard` (clickable `Surface`, color = player's color) →
   centered `Column`: name (+ "· starts" if applicable) → conditional
   mulligan badge → life `Row` (`IconButton "-"`, large number, `IconButton
   "+"`) → "Commander Damage" hint (only if the panel isn't open). If
   `showCommanderDamage` is true, it is replaced by an overlay `Surface`
   with a grid of `CommanderDamageItem` (attacker's color dot, number,
   `IconButton -`/`+`) per opponent.
5. Conditional: "Finish" confirmation `AlertDialog` (if
   `showFinishConfirm`).
6. Conditional: final result `AlertDialog` (if `state.isFinished`) — title
   with the winner or "Game finished", list of each player's final life,
   "Back to home" button.

**Interactive:**
- Header `<` / `>`: turn -1 / +1 (minimum 1).
- **"Finish"** (header): opens the confirmation dialog.
- Per `PlayerCard`: tapping the whole card toggles the commander damage
  panel; life `-`/`+`; if the panel is open, damage `-`/`+` for each
  opponent.
- Confirmation dialog: "Finish" (confirms) / "Cancel".
- Final dialog: "Back to home" → returns to `DashboardRoute`.

---

## 6. `HistoryScreen`

**File:** `presentation/screens/history/HistoryScreen.kt`
**Route:** `HistoryRoute`

```
┌─────────────────────────────────────┐
│ [<]   Game history                   │  ← TopAppBar
├─────────────────────────────────────┤
│ ┌─────────────────────────────────┐ │
│ │ 07/26/2026 18:40      Finished   │ │  ← Row SpaceBetween (date / status)
│ │ Won: Player 1                     │ │  ← titleMedium
│ │ ● Player 1: 12   ● Player 2: 0    │ │  ← color dots + name + final
│ │ ● Player 3: 0 (1m) ● Player 4: 0  │ │     life (+ mulligan suffix)
│ └─────────────────────────────────┘ │
│ ┌─────────────────────────────────┐ │
│ │ 07/25/2026 21:05      In progress│ │
│ │ 4 players                         │ │  ← if there is no winner (won=false
│ │ ● P1: 40  ● P2: 40  ● P3: 40 ...  │ │     for everyone), shows player count
│ └─────────────────────────────────┘ │     instead of "Won: X"
│              ⋮                       │
└─────────────────────────────────────┘

(if there are no games)
┌─────────────────────────────────────┐
│ [<]   Game history                   │
├─────────────────────────────────────┤
│                                       │
│   No games recorded yet              │  ← centered, bodyLarge
│                                       │
└─────────────────────────────────────┘
```

**Hierarchy:** `Column` → `TopAppBar` (title + `TextButton "<"` as
navigationIcon) → if `games.isEmpty()`: centered `Box` with message;
otherwise, `LazyColumn` of `GameHistoryCard` (`key = game.id`).

Each `GameHistoryCard` (`Card`) → `Column`: SpaceBetween `Row` (formatted
date `dd/MM/yyyy HH:mm` — status `Text` "Finished"/"In progress") → result
`Text` ("Won: {name}" if `won == true`, or "{N} players" if not) → `Row` of
players ordered by `seatIndex` (color dot + "{name}: {final life}" + `(Nm)`
suffix if they had mulligans).

**Interactive:**
- `TextButton "<"` in the `TopAppBar` → `onBack()` → `popBackStack()`.
- The list is read-only: no swipe-to-delete, no tap-to-expand, no filters
  or search.

**Fidelity note:** the data comes 100% from Room
(`gameDao.getGamesWithPlayers()`), local to the device — there is no
synchronization indicator because there is no synchronization.

---

## Navigation summary between wireframes

See also `docs/diagrams/android-navigation-flow.md` for the complete graph
with the routes and their arguments.

```
LoginScreen → DashboardScreen ─┬─→ PlayerSetupScreen → PreGameScreen → GameTrackerScreen ─┐
                                │                                                          │
                                └─→ HistoryScreen                    (returns to) ─────────┘
                                              ↑______________________________________________|
```
