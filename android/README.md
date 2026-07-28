# Commander Companion — Android

Cliente Android nativo (Kotlin + Jetpack Compose), pensado para trackear vida
*durante* la partida con la app en la mesa — la prioridad es que cualquier
acción tome menos de dos segundos. Ver [ADR-0009](../docs/decisions/0009-android-nativo-vs-crossplatform.md)
para el porqué de nativo vs. cross-platform.

Estado actual: life tracker local completo (2-6 jugadores, daño de comandante,
mulligans, historial en Room), auth real contra el backend (email/password +
Google Sign-In) y un espejo best-effort del asiento local contra
`games`/`game-actions` de la API. Detalle exacto de qué está hecho y qué no,
tarea por tarea: [`docs/roadmap/TASKS.md`](../docs/roadmap/TASKS.md) (Stage 4
y 5).

## Stack

Kotlin 2.2 + Jetpack Compose (Material 3), Hilt (DI), Room (persistencia
local), Retrofit + OkHttp + kotlinx.serialization (red), Navigation Compose,
Credential Manager + Google Identity Services (Google Sign-In). AGP 9.3 /
Gradle 9.5, requiere **JDK 21** para buildear (ver nota abajo).

## Setup

```bash
cd android
./gradlew assembleDebug
```

Necesita el backend corriendo (ver [`backend/README.md`](../backend/README.md)).
Por defecto apunta a `http://10.0.2.2:8080/` (el alias que usa el emulador de
Android para llegar al `localhost` de la máquina host). Para un dispositivo
físico en la misma red, o para apuntar a otro host:

```bash
./gradlew :app:assembleDebug -PAPI_BASE_URL=http://192.168.1.50:8080/
```

(o seteando `API_BASE_URL` en `gradle.properties`; ver `app/build.gradle.kts`).

### Google Sign-In

`GOOGLE_WEB_CLIENT_ID` es hoy un placeholder (`app/build.gradle.kts`) — las
credenciales OAuth reales de Google Cloud todavía no existen (paso manual
externo, ver `docs/roadmap/TASKS.md` Stage 1). El flujo de Credential Manager
está completo e implementado, pero falla contra Google real hasta que se
cree el Web Client ID. Una vez creado, se pasa sin tocar código:

```bash
./gradlew :app:assembleDebug -PGOOGLE_WEB_CLIENT_ID=...apps.googleusercontent.com
```

### JDK requerido

Gradle 9.5 / AGP 9.3 no arrancan con JDK 8. Si `java -version` del sistema no
es 17+, apuntá `JAVA_HOME` a un JDK compatible (Android Studio trae uno
embebido en `Android Studio/jbr`) antes de invocar `gradlew`:

```bash
JAVA_HOME=/path/a/jdk-21 ./gradlew assembleDebug
```

## Comandos

```bash
./gradlew assembleDebug          # build
./gradlew testDebugUnitTest      # tests unitarios (JUnit + kotlinx-coroutines-test)
./gradlew lintDebug              # Android Lint
./gradlew connectedAndroidTest   # tests instrumentados (requiere emulador/dispositivo)
```

Los tres primeros son los mismos que corre `.github/workflows/android-ci.yml`.

## Estructura

```
android/app/src/main/java/com/commandercompanion/
├── data/
│   ├── remote/
│   │   ├── api/          # CommanderApi.kt (decks/games/game-actions/statistics), AuthApi.kt
│   │   ├── dto/           # DTOs de Retrofit
│   │   └── interceptor/   # AuthInterceptor (Bearer), AuthAuthenticator (refresh-on-401)
│   ├── repository/        # GameRepository, DeckRepository — deciden Room vs. backend
│   ├── local/
│   │   ├── dao/           # GameDao
│   │   └── entity/        # GameEntity, PlayerResultEntity
│   └── session/           # SessionManager (DataStore: access/refresh token + expiry)
├── presentation/
│   ├── screens/           # login, dashboard, setup, pregame, game, history
│   ├── navigation/        # AppNavigation.kt, Routes.kt, PlayerConfigCodec.kt
│   ├── components/        # PlayerCard, etc.
│   └── theme/             # Color.kt, Theme.kt, Type.kt (Material 3)
└── core/
    ├── di/                 # DatabaseModule, NetworkModule (Hilt)
    └── util/               # ApiCall (mapea HttpException/IOException -> ApiError)
```

Nota: no hay capa `domain/` (casos de uso) — los `ViewModel` van directo
contra `data/repository/` (o, en auth, directo contra `AuthApi`). Decisión
consciente, ver ADR-0009 y `docs/roadmap/TASKS.md` Stage 4.

## Notas

- El life tracker local (`GameViewModel`, Room) funciona 100% sin red ni
  sesión — el espejo contra el backend (`GameRepository.bootstrapRemoteGame`)
  es best-effort y aditivo, nunca bloquea la partida local. Estado visible en
  `GameState.remoteSync` / `RemoteSyncBanner`.
- Solo el asiento del usuario autenticado tiene `GamePlayer` remoto; los
  demás asientos y el daño de comandante de cualquier asiento nunca se
  espejan (ver comentario en `GameViewModel.adjustCommanderDamage`).
- Cliente WebSocket (para ver en vivo lo que hacen otros dispositivos en la
  misma partida) todavía no existe — el servidor y el protocolo ya están
  implementados (ver [ADR-0005](../docs/decisions/0005-websocket-protocol.md)),
  falta el consumidor Android. Es la mayor brecha señalada en
  `docs/roadmap/TASKS.md`.
- Wireframes de las 6 pantallas reales: [`docs/ux/wireframes.md`](../docs/ux/wireframes.md).
  Grafo de navegación real: [`docs/diagrams/android-navigation-flow.md`](../docs/diagrams/android-navigation-flow.md).
