# Commander Companion — Android

Native Android client (Kotlin + Jetpack Compose), designed to track life
*during* the game with the app on the table — the priority is that any
action takes less than two seconds. See [ADR-0009](../docs/decisions/0009-android-nativo-vs-crossplatform.md)
for the rationale behind native vs. cross-platform.

Current status: complete local life tracker (2-6 players, commander
damage, mulligans, history in Room), real auth against the backend
(email/password + Google Sign-In), and a best-effort mirror of the local
seat against the API's `games`/`game-actions`. Exact detail of what's
done and what isn't, task by task: [`docs/roadmap/TASKS.md`](../docs/roadmap/TASKS.md) (Stage 4
and 5).

## Stack

Kotlin 2.2 + Jetpack Compose (Material 3), Hilt (DI), Room (local
persistence), Retrofit + OkHttp + kotlinx.serialization (network), Navigation Compose,
Credential Manager + Google Identity Services (Google Sign-In). AGP 9.3 /
Gradle 9.5, requires **JDK 21** to build (see note below).

## Setup

```bash
cd android
./gradlew assembleDebug
```

Needs the backend running (see [`backend/README.md`](../backend/README.md)).
By default it points to `http://10.0.2.2:8080/` (the alias the Android
emulator uses to reach the host machine's `localhost`). For a physical
device on the same network, or to point at another host:

```bash
./gradlew :app:assembleDebug -PAPI_BASE_URL=http://192.168.1.50:8080/
```

(or by setting `API_BASE_URL` in `gradle.properties`; see `app/build.gradle.kts`).

### Google Sign-In

`GOOGLE_WEB_CLIENT_ID` is currently a placeholder (`app/build.gradle.kts`) —
the real Google Cloud OAuth credentials don't exist yet (an external manual
step, see `docs/roadmap/TASKS.md` Stage 1). The Credential Manager flow
is fully implemented, but fails against real Google until the
Web Client ID is created. Once created, it's passed without touching code:

```bash
./gradlew :app:assembleDebug -PGOOGLE_WEB_CLIENT_ID=...apps.googleusercontent.com
```

### Required JDK

Gradle 9.5 / AGP 9.3 won't start with JDK 8. If the system's `java -version`
isn't 17+, point `JAVA_HOME` at a compatible JDK (Android Studio ships one
bundled at `Android Studio/jbr`) before invoking `gradlew`:

```bash
JAVA_HOME=/path/to/jdk-21 ./gradlew assembleDebug
```

## Commands

```bash
./gradlew assembleDebug          # build
./gradlew testDebugUnitTest      # unit tests (JUnit + kotlinx-coroutines-test)
./gradlew lintDebug              # Android Lint
./gradlew connectedAndroidTest   # instrumented tests (requires emulator/device)
```

The first three are the same ones run by `.github/workflows/android-ci.yml`.

## Structure

```
android/app/src/main/java/com/commandercompanion/
├── data/
│   ├── remote/
│   │   ├── api/          # CommanderApi.kt (decks/games/game-actions/statistics), AuthApi.kt
│   │   ├── dto/           # Retrofit DTOs
│   │   └── interceptor/   # AuthInterceptor (Bearer), AuthAuthenticator (refresh-on-401)
│   ├── repository/        # GameRepositoryImpl, DeckRepositoryImpl, etc. — decide Room vs. backend
│   ├── local/
│   │   ├── dao/           # GameDao
│   │   └── entity/        # GameEntity, PlayerResultEntity
│   └── session/           # SessionManager (DataStore: access/refresh token + expiry)
├── domain/
│   ├── repository/        # GameRepository, DeckRepository, PlaygroupRepository, StatisticsRepository (interfaces)
│   ├── model/              # LocalSeat/RemoteGameSession/SeatAssignment, DeckWithStats/PlaygroupSummary, PlayerOutcome
│   └── usecase/            # ResolveGameOutcomeUseCase, ReplayCommanderDamageUseCase, LoadStatisticsUseCase
├── presentation/
│   ├── screens/           # login, dashboard, setup, pregame, game, history
│   ├── navigation/        # AppNavigation.kt, Routes.kt, PlayerConfigCodec.kt
│   ├── components/        # AppComponents (buttons, cards, chips), PlayerQuadrant
│   └── theme/             # Color.kt, Theme.kt, Type.kt (Material 3)
└── core/
    ├── di/                 # DatabaseModule, NetworkModule, RepositoryModule (Hilt)
    └── util/               # ApiCall (maps HttpException/IOException -> ApiError)
```

`ViewModel`s depend on the `domain/repository/` interfaces, not on the
`data/repository/` classes directly — Hilt binds each interface to its
`*Impl` via `core/di/RepositoryModule.kt`. Auth is the one area that still
goes straight against `AuthApi`/`SessionManager` (no `AuthRepository`):
`SessionManager` has a `Context` constructor, unfakeable in a pure-JVM
test without Robolectric, so wrapping it wouldn't add testability on its
own — see `docs/roadmap/TASKS.md` Stage 4.

## Notes

- Language (`SettingsScreen` → "Idioma") is overridable per-app via
  `AppCompatDelegate.setApplicationLocales()` (`androidx.appcompat`), same 3
  languages as the web (`values`/`values-en`/`values-ca`) — persisted across
  restarts via `autoStoreLocales` in `AndroidManifest.xml`. See
  `presentation/screens/settings/AppLanguage.kt`.
- The local life tracker (`GameViewModel`, Room) works 100% without network
  or session — the mirror against the backend (`GameRepository.bootstrapRemoteGame`)
  is best-effort and additive, it never blocks the local game. Status is visible in
  `GameState.remoteSync` / `RemoteSyncBanner`.
- Only the authenticated user's seat has a remote `GamePlayer`; the
  other seats and any seat's commander damage are never
  mirrored (see the comment in `GameViewModel.adjustCommanderDamage`).
- A WebSocket client (to see live what other devices in the
  same game are doing) doesn't exist yet — the server and protocol are
  already implemented (see [ADR-0005](../docs/decisions/0005-websocket-protocol.md)),
  the Android consumer is missing. It's the biggest gap flagged in
  `docs/roadmap/TASKS.md`.
- Wireframes of the 6 actual screens: [`docs/ux/wireframes.md`](../docs/ux/wireframes.md).
  Actual navigation graph: [`docs/diagrams/android-navigation-flow.md`](../docs/diagrams/android-navigation-flow.md).
