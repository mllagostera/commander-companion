package com.commandercompanion.presentation.screens.settings

import com.commandercompanion.R

/**
 * The 3 languages the app ships (`values`/`values-en`/`values-ca`) — same set and same endonym
 * labels as the web client's language selector (`web/nuxt.config.ts`'s `i18n.locales`).
 *
 * [tag] is the BCP-47 tag passed to `LocaleListCompat.forLanguageTags()`/matched against
 * `AppCompatDelegate.getApplicationLocales().toLanguageTags()` (see `SettingsViewModel`).
 */
enum class AppLanguage(val tag: String, val labelRes: Int) {
    SPANISH(tag = "es", labelRes = R.string.settings_language_spanish),
    ENGLISH(tag = "en", labelRes = R.string.settings_language_english),
    CATALAN(tag = "ca", labelRes = R.string.settings_language_catalan);

    companion object {
        /**
         * Resolves the currently applied language from AppCompat's language tags
         * (`AppCompatDelegate.getApplicationLocales().toLanguageTags()`), which is `""` when no
         * per-app override has been set yet (i.e. still following the system locale) — that, and
         * any tag outside the 3 we ship, falls back to [SPANISH] (the app's default locale, same
         * as the web's `defaultLocale: 'es'`).
         */
        fun fromTag(tag: String?): AppLanguage = entries.firstOrNull { it.tag == tag } ?: SPANISH
    }
}
