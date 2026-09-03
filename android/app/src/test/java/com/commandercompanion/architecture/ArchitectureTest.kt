package com.commandercompanion.architecture

import com.lemonappdev.konsist.api.Konsist
import com.lemonappdev.konsist.api.declaration.KoFileDeclaration
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * The layering rules from `docs/architecture/ARCHITECTURE.md` and
 * `docs/architecture/PROJECT-STRUCTURE.md` §4, asserted instead of assumed.
 *
 * These run inside `testDebugUnitTest`, which is already a required CI check, so
 * a layering violation fails the same gate as a broken unit test. Konsist is
 * used as the parser — a real Kotlin AST rather than a grep over import lines —
 * while the assertions are plain JUnit, so each failure can name the offending
 * files and say what to do about them.
 *
 * Every rule here is an invariant: it holds today and must keep holding. Two of
 * them started life as ratchets over known debt (domain depending on data, and
 * screens reading DTOs) and became invariants once that debt was paid off — see
 * `docs/architecture/PROJECT-STRUCTURE.md` §9.
 */
class ArchitectureTest {

    private val productionFiles: List<KoFileDeclaration> =
        Konsist.scopeFromProduction().files

    private fun KoFileDeclaration.inLayer(layer: String): Boolean =
        path.replace('\\', '/').contains("/com/commandercompanion/$layer/")

    private fun KoFileDeclaration.importsPrefix(prefix: String): Boolean =
        imports.any { it.name.startsWith(prefix) }

    private fun filesIn(layer: String): List<KoFileDeclaration> =
        productionFiles.filter { it.inLayer(layer) }

    // KoFileDeclaration.name is the file name WITHOUT the .kt extension, which is
    // what every list in this file is written against.
    private fun describe(files: List<KoFileDeclaration>): String =
        files.joinToString("\n") { "  - " + it.name }

    // -----------------------------------------------------------------------
    // Invariants
    // -----------------------------------------------------------------------

    @Test
    fun `domain does not depend on presentation`() {
        val offenders = filesIn("domain")
            .filter { it.importsPrefix("com.commandercompanion.presentation") }

        assertTrue(
            "The domain layer must not know about the UI. Offending files:\n" +
                describe(offenders),
            offenders.isEmpty(),
        )
    }

    @Test
    fun `data does not depend on presentation`() {
        val offenders = filesIn("data")
            .filter { it.importsPrefix("com.commandercompanion.presentation") }

        assertTrue(
            "Repositories, DAOs and API clients must not import UI code. " +
                "Offending files:\n" + describe(offenders),
            offenders.isEmpty(),
        )
    }

    @Test
    fun `domain is free of the Android framework`() {
        // Keeps use cases and repository interfaces unit-testable on the JVM
        // without Robolectric or an emulator.
        val offenders = filesIn("domain").filter {
            it.importsPrefix("android.") || it.importsPrefix("androidx.")
        }

        assertTrue(
            "The domain layer must stay plain Kotlin. Offending files:\n" +
                describe(offenders),
            offenders.isEmpty(),
        )
    }

    @Test
    fun `only the auth screens talk to Retrofit directly`() {
        // ARCHITECTURE.md records this exception: the auth surface injects
        // AuthApi/CommanderApi instead of going through a domain repository.
        // Enumerated rather than waived, so a fourth screen doing the same has
        // to be added here deliberately — which is where someone asks whether
        // it should exist at all.
        val offenders = filesIn("presentation")
            .filter { it.importsPrefix("com.commandercompanion.data.remote.api") }
            .filterNot { it.name in AUTH_SCREENS }

        assertTrue(
            "ViewModels reach the backend through a domain repository, not " +
                "through Retrofit. If this is a deliberate new exception, add it " +
                "to AUTH_SCREENS in this test and to ARCHITECTURE.md. Offending files:\n" +
                describe(offenders),
            offenders.isEmpty(),
        )
    }

    @Test
    fun `every ViewModel is a Hilt ViewModel`() {
        val offenders = Konsist.scopeFromProduction()
            .classes()
            .filter { it.name.endsWith("ViewModel") }
            .filterNot { klass -> klass.annotations.any { it.name == "HiltViewModel" } }
            .map { it.name }

        assertTrue(
            "A ViewModel without @HiltViewModel cannot be injected and will " +
                "fail at runtime, not at compile time. Offending classes:\n" +
                offenders.joinToString("\n") { "  - $it" },
            offenders.isEmpty(),
        )
    }

    @Test
    fun `repository implementations live in data and are named Impl`() {
        val offenders = Konsist.scopeFromProduction()
            .classes()
            .filter { klass ->
                klass.parents().any { it.name.endsWith("Repository") }
            }
            .filterNot { it.name.endsWith("Impl") }
            .map { it.name }

        assertTrue(
            "A class implementing a domain repository interface is named " +
                "<Name>Impl and lives in data/repository/. Offending classes:\n" +
                offenders.joinToString("\n") { "  - $it" },
            offenders.isEmpty(),
        )
    }

    @Test
    fun `domain does not depend on data`() {
        // Was a ratchet until the 2026-09-03 refactor: the domain's repository
        // interfaces used to be declared in terms of Retrofit DTOs, and
        // GameRepository took a Room entity, so `domain` could not be reasoned
        // about without `data`. The payload types now live in domain/model/ and
        // `data` depends on them; Room's GameWithPlayers is mapped to
        // PlayedGame at the boundary. The list is empty, so this is an
        // invariant now, not a budget.
        val offenders = filesIn("domain")
            .filter { it.importsPrefix("com.commandercompanion.data") }

        assertTrue(
            "The domain layer owns its types. A wire payload the domain names belongs in " +
                "domain/model/ (it may keep its @Serializable annotations); a persistence type " +
                "gets mapped at the data boundary, the way GameRepositoryImpl maps " +
                "GameWithPlayers to PlayedGame. Offending files:\n" + describe(offenders),
            offenders.isEmpty(),
        )
    }

    @Test
    fun `only the auth screens consume data types`() {
        // Same enumerated exception as the Retrofit rule above and for the same
        // reason: the auth surface deliberately sits outside the domain layer,
        // so it speaks the auth request/response bodies directly.
        val offenders = filesIn("presentation")
            .filter {
                it.importsPrefix("com.commandercompanion.data.remote.dto") ||
                    it.importsPrefix("com.commandercompanion.data.local")
            }
            .filterNot { it.name in AUTH_SCREENS }

        assertTrue(
            "Screens and ViewModels consume domain models, not DTOs or Room entities. " +
                "Expose a domain model or a UI state instead. Offending files:\n" +
                describe(offenders),
            offenders.isEmpty(),
        )
    }

    private companion object {
        /**
         * The auth surface, which by decision does not go through the domain layer
         * (ARCHITECTURE.md, TASKS.md Stage 4). Adding a name here widens a documented
         * exception — do it deliberately, and update those documents too.
         */
        val AUTH_SCREENS = setOf(
            "LoginViewModel",
            "RegisterViewModel",
            "SettingsViewModel",
        )
    }
}
