# ADR-0009: Native Android client (Kotlin + Compose) instead of cross-platform

**Status:** Accepted and implemented — **inherited decision, context
reconstructed** (see the method note in ADR-0006; written retroactively
on 2026-07-27 based on `android/app/build.gradle.kts` and the actual
structure of `android/app/src/main/java/com/commandercompanion/`).

## Context

The ROADMAP sets "Simplicity, Speed, Data" as the project's three pillars,
with the explicit clarification that "the priority is that any
action can be done in under two seconds" — the life tracker
(`GameTrackerScreen`) is the screen that gets used constantly *during* an
actual Commander game, with the app on the table between turns, so
interaction latency (tapping `+`/`-` for life, opening the commander damage
panel) is a product requirement, not just a technical one. The platform for
the first mobile client had to be decided: native per OS, or a
cross-platform framework that shares code with an eventual iOS client.

## Decision

**Native Android with Kotlin + Jetpack Compose**, Material 3, Navigation
Compose (with `kotlinx.serialization` for typed routes, see
`Routes.kt`/`AppNavigation.kt`), Hilt for dependency injection, Room
for local persistence, and Retrofit for consuming the REST API
(`android/app/build.gradle.kts`). Declared architecture: Clean Architecture
+ MVVM + UDF (unidirectional data flow).

**Updated 2026-07-27**: `CommanderApi.kt` already has the real endpoints for
`decks`/`games`/`game-actions`/`statistics` (15 methods, see
`data/remote/api/CommanderApi.kt`), and there is now a repository layer
(`data/repository/GameRepository.kt`, `DeckRepository.kt`) between the
`ViewModel`s and `CommanderApi`/`GameDao` — see Consequences, also
updated.

## Alternatives considered

- **Flutter**: cross-platform with a single Dart codebase for Android and iOS,
  good UI performance (its own rendering engine, not dependent on a
  native bridge). Ruled out — presumably because there was (and still is)
  no explicit iOS roadmap in `ROADMAP.md`, and because Compose already
  gives fine-grained control over UI performance on Android without the
  extra layer of a cross-platform rendering engine nor the risk of
  missing native plugins for Android-specific integrations
  (Credential Manager / Google Identity Services, mentioned as
  pending in `TASKS.md`, already assume native Android APIs).
- **React Native**: would allow more superficial sharing with an
  eventual React web frontend, but the project had already decided on a
  decoupled, different web stack (Nuxt/Vue, see ADR-0004) — there is no
  synergy in sharing components between RN and Nuxt. The JS-native
  bridge also introduces a source of variable latency, undesirable for a
  screen that is used constantly during the game.
- **Kotlin Multiplatform (KMP) sharing only logic, native UI per
  platform**: the "middle ground" option closest to what was chosen — it
  would have allowed sharing `GameState`/`GameViewModel`/rules logic with a
  future iOS client in Swift/SwiftUI while keeping Compose for
  Android. Not adopted, probably because there is no iOS client planned
  yet and the complexity of structuring `commonMain`/
  `androidMain` modules is not justified without a second real consumer of
  that shared logic. It remains as a less costly future migration option
  than rewriting from a full cross-platform framework, should iOS enter
  the roadmap.
- **Traditional UI with XML Views (no Compose)**: ruled out as the
  older and more verbose Android approach; Compose was already Google's
  official recommendation for new projects at the time this project started,
  and fits better with a highly dynamic UI (2-to-6-player grid
  that is rebuilt based on count, commander damage overlay) than XML +
  manual `RecyclerView`.

## Consequences

- The app's UI/logic code **is not shared with any other
  client** — the web client (ADR-0004, Nuxt) is a completely
  separate project that only shares the HTTP contract (`docs/api/openapi.yaml`), not
  components or domain logic. Any business rule of the life
  tracker (e.g. how the winner is calculated, see `GameViewModel
  .finishGame`) has to be re-implemented if an iOS client ever exists.
- The native stack (Compose + Hilt + Room + Retrofit) is the one recommended by
  Google and has first-class tooling support (Android Studio,
  Compose Preview, `androidx.lifecycle`), which explains why the
  development speed of the local life tracker was high (full life tracker
  with persistence in a single session, per `TASKS.md`).
- **Partially resolved (2026-07-27)**: `GameRepository` is now the single
  point that decides what goes to Room (local history, always) and what goes to
  the backend (best-effort mirror of the authenticated user's seat —
  `bootstrapRemoteGame`/`recordLifeChange`/`finishGame`); `DeckRepository`
  is 100% remote. There is still no `domain/` layer with its own
  use cases (`GameViewModel`/`LoginViewModel` still call the repository
  directly) — acceptable while the scope stays as it is, revisit whether
  a separate `domain/` layer is warranted later (see `TASKS.md` Stage 4).

## References

- `android/app/build.gradle.kts`
- `docs/roadmap/ROADMAP.md`, "Stage 4: Android Client" section and
  "Project philosophy"
- `docs/roadmap/TASKS.md`, Stage 4
