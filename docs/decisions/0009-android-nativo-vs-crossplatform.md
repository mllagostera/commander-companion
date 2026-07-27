# ADR-0009: Cliente Android nativo (Kotlin + Compose) en vez de cross-platform

**Estado:** Aceptada e implementada — **decisión heredada, contexto
reconstruido** (ver nota de método en ADR-0006; redactado retroactivamente
el 2026-07-27 a partir de `android/app/build.gradle.kts` y la estructura
real de `android/app/src/main/java/com/commandercompanion/`).

## Contexto

El ROADMAP fija "Simplicidad, Velocidad, Datos" como los tres pilares del
proyecto, con la aclaración explícita de que "la prioridad es que cualquier
acción pueda hacerse en menos de dos segundos" — el life tracker
(`GameTrackerScreen`) es la pantalla que se usa constantemente *durante* una
partida real de Commander, con la app en la mesa entre turnos, así que la
latencia de interacción (tocar `+`/`-` de vida, abrir el panel de daño de
comandante) es un requisito de producto, no solo técnico. Había que decidir
la plataforma del primer cliente móvil: nativo por SO, o un framework
cross-platform que comparta código con un eventual cliente iOS.

## Decisión

**Android nativo con Kotlin + Jetpack Compose**, Material 3, Navigation
Compose (con `kotlinx.serialization` para rutas tipadas, ver
`Routes.kt`/`AppNavigation.kt`), Hilt para inyección de dependencias, Room
para persistencia local, y Retrofit para el consumo futuro de la API REST
(`android/app/build.gradle.kts`). Arquitectura declarada: Clean Architecture
+ MVVM + UDF (unidirectional data flow), aunque en la práctica actual el
life tracker todavía no tiene capa de dominio/repositorio separada (`GameDao`
se inyecta directo en los `ViewModel`, ver `TASKS.md` Stage 4).

## Alternativas consideradas

- **Flutter**: cross-platform con un único codebase Dart para Android e iOS,
  buen rendimiento de UI (motor de renderizado propio, no depende del
  puente nativo). Se descartó — presumiblemente porque no había (ni hay
  todavía) un roadmap de iOS explícito en `ROADMAP.md`, y porque Compose ya
  da control fino sobre el rendimiento de UI en Android sin la capa
  adicional de un motor de renderizado cross-platform ni el riesgo de
  plugins nativos faltantes para integraciones específicas de Android
  (Credential Manager / Google Identity Services, mencionados como
  pendientes en `TASKS.md`, ya asumen APIs nativas de Android).
- **React Native**: permitiría compartir más superficialmente con un
  eventual frontend web en React, pero el proyecto ya decidió un stack web
  desacoplado y distinto (Nuxt/Vue, ver ADR-0004) — no hay sinergia de
  compartir componentes entre RN y Nuxt. El puente JS-nativo también
  introduce una fuente de latencia variable, indeseable para una pantalla
  que se usa constantemente durante la partida.
- **Kotlin Multiplatform (KMP) compartiendo solo lógica, UI nativa por
  plataforma**: la opción "intermedia" más cercana a lo elegido — hubiera
  permitido compartir `GameState`/`GameViewModel`/lógica de reglas con un
  futuro cliente iOS en Swift/SwiftUI mientras se mantiene Compose para
  Android. No se adoptó, probablemente porque no hay cliente iOS planeado
  todavía y la complejidad de estructurar módulos `commonMain`/
  `androidMain` no se justifica sin un segundo consumidor real de esa
  lógica compartida. Queda como opción de migración futura menos costosa
  que reescribir desde un framework cross-platform completo, si iOS entra
  al roadmap.
- **UI tradicional con XML Views (sin Compose)**: descartado por ser el
  enfoque más antiguo y verboso de Android; Compose ya era la recomendación
  oficial de Google para proyectos nuevos al momento de iniciar este
  proyecto, y encaja mejor con una UI muy dinámica (grid de 2 a 6 jugadores
  que se re-arma según cantidad, overlay de daño de comandante) que XML +
  `RecyclerView` manual.

## Consecuencias

- El código de UI/lógica de la app **no se comparte con ningún otro
  cliente** — el cliente web (ADR-0004, Nuxt) es un proyecto completamente
  aparte que solo comparte el contrato HTTP (`docs/api/openapi.yaml`), no
  componentes ni lógica de dominio. Cualquier regla de negocio del life
  tracker (p. ej. cómo se calcula el ganador, ver `GameViewModel
  .finishGame`) debe re-implementarse si algún día existe un cliente iOS.
- El stack nativo (Compose + Hilt + Room + Retrofit) es el recomendado por
  Google y tiene soporte de tooling de primera clase (Android Studio,
  Compose Preview, `androidx.lifecycle`), lo cual explica por qué la
  velocidad de desarrollo del life tracker local fue alta (life tracker
  completo con persistencia en una sola sesión, según `TASKS.md`).
- Sin capa de dominio/repositorio (todavía pendiente, ver `TASKS.md` Stage
  4: "Repositorios reales en `data/repository/` — no existe todavía"), el
  acoplamiento actual entre `ViewModel` y `GameDao` (Room) es aceptable
  mientras el life tracker sea 100% local, pero es deuda técnica explícita
  a resolver cuando se integre el backend real (Stage 5) — mezclar fuente
  local (Room) y remota (Retrofit) sin una capa de repositorio que decida
  cuál usar sería más costoso de deshacer cuanto más código dependa
  directamente de `GameDao`.

## Referencias

- `android/app/build.gradle.kts`
- `docs/roadmap/ROADMAP.md`, sección "Stage 4: Cliente Android" y
  "Filosofía del proyecto"
- `docs/roadmap/TASKS.md`, Stage 4
