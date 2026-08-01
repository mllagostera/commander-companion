package com.commandercompanion.presentation.screens.settings

import org.junit.Assert.assertEquals
import org.junit.Test

class AppLanguageTest {

    @Test
    fun `reconoce cada tag soportado`() {
        assertEquals(AppLanguage.SPANISH, AppLanguage.fromTag("es"))
        assertEquals(AppLanguage.ENGLISH, AppLanguage.fromTag("en"))
        assertEquals(AppLanguage.CATALAN, AppLanguage.fromTag("ca"))
    }

    /** `AppCompatDelegate.getApplicationLocales().toLanguageTags()` devuelve "" mientras no haya override. */
    @Test
    fun `sin override todavia (tag vacio) usa el idioma por defecto`() {
        assertEquals(AppLanguage.SPANISH, AppLanguage.fromTag(""))
    }

    @Test
    fun `tag null usa el idioma por defecto`() {
        assertEquals(AppLanguage.SPANISH, AppLanguage.fromTag(null))
    }

    @Test
    fun `un tag fuera de los 3 idiomas soportados usa el idioma por defecto`() {
        assertEquals(AppLanguage.SPANISH, AppLanguage.fromTag("fr"))
    }

    @Test
    fun `cada idioma manda su propio tag BCP-47`() {
        assertEquals("es", AppLanguage.SPANISH.tag)
        assertEquals("en", AppLanguage.ENGLISH.tag)
        assertEquals("ca", AppLanguage.CATALAN.tag)
    }
}
