package com.commandercompanion.data.session

/**
 * Narrow seam over [SessionManager.currentAccessToken].
 *
 * Exists so a class that only needs "give me the current access token" (e.g. `GameViewModel`,
 * to authenticate the live-sync WebSocket, see ADR-0005) doesn't have to depend on the full
 * [SessionManager] — a concrete class with an Android `Context` in its constructor, which can't
 * be faked in a pure-JVM test without Robolectric (the project has neither, see the equivalent
 * gap documented for `SettingsViewModelTest` in `docs/roadmap/TASKS.md`). A `fun interface` is
 * trivially fakeable with a lambda instead.
 */
fun interface AccessTokenProvider {
    suspend fun currentAccessToken(): String?
}
