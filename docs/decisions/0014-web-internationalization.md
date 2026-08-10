# ADR-0014: Web internationalization with `@nuxtjs/i18n`

**Status:** Accepted (2026-07-29), updated (2026-07-30) — English and Catalan added

## Context

The entire web UI (`web/app`) had text hardcoded
directly in templates, attributes, and composables: ~200-250 distinct
literals spread across the 9 pages, the layout, and the composables that build
error/toast messages. There was no i18n package installed nor
any precedent for this decision in any prior ADR — internationalizing the web
was a task starting from scratch.

Additionally, the original copy was written in Argentine "voseo" ("tenés",
"sos", "jugás", "Creá", "Registrate"...), without this being a conscious
style decision — it was simply the dialect the app happened to be written in.

## Decision

### Library: `@nuxtjs/i18n`

`@nuxtjs/i18n` (which wraps vue-i18n) is adopted as the base for
web internationalization, instead of integrating vue-i18n manually. It's the
official Nuxt module: it's registered under `modules` just like
`@nuxtjs/tailwindcss`/`@nuxt/eslint` (already present), auto-imports
`useI18n()`/`$t`/`t`/`d`, and integrates SSR without its own plugin.

Configuration (`web/nuxt.config.ts`), updated when English and
Catalan were added (2026-07-30):

```ts
i18n: {
  locales: [
    { code: 'es', language: 'es-ES', file: 'es.json', name: 'Español' },
    { code: 'en', language: 'en-US', file: 'en.json', name: 'English' },
    { code: 'ca', language: 'ca-ES', file: 'ca.json', name: 'Català' },
  ],
  defaultLocale: 'es',
  strategy: 'no_prefix',
  detectBrowserLanguage: {
    useCookie: true,
    cookieKey: 'cc_locale',
  },
},
```

- `strategy: 'no_prefix'` is kept: with `detectBrowserLanguage` +
  a manual selector it's enough to choose a language without needing `/en/...`/`/ca/...`
  routes — simpler for a single domain without real multi-language
  SEO behind it.
- `detectBrowserLanguage` goes from `false` to active now that there are 3 locales:
  it detects the browser's language only if there is no cookie yet (`cc_locale`)
  — once the user explicitly chooses via the layout selector
  (`setLocale()`, see below) or it has already been detected once, that cookie
  takes precedence and it isn't re-detected on subsequent loads.
- Messages live in `web/i18n/locales/{es,en,ca}.json`, the same ~250
  keys across all three files (namespacing unchanged, see below) — key
  parity and interpolation placeholder parity (`{count}`,
  `{username}`, etc.) across all three were verified before merging. Date formatting
  (`datetimeFormats`) in `web/i18n/i18n.config.ts` now has an entry for
  all three locales (previously only `es`).
- New language selector in the user menu of the layout
  (`app/layouts/default.vue`, next to the dark mode toggle): three pills
  `ES`/`EN`/`CA`, `useI18n().setLocale(code)` on click — the only way to
  change language manually today (there wasn't one before).

### Key naming convention

Namespacing by page/composable (`login.*`, `settings.*`,
`playgroups.list.*`, `playgroups.detail.*`, etc.), with `errors.*` reserved
for the messages built by the error composables (`useSettings.ts`,
`usePlaygroups.ts`, `useDecks.ts`, `useGames.ts`, `useUsers.ts`) — they are
shared among their callers, not duplicated per page — and `toast.*`
for `useToast()` messages. `common.*` groups the labels repeated
across pages ("Cancel," "Save," "Saving…").

Pluralization (`playgroups.list.gamesPlayedCount`/`memberCount`) uses vue-i18n's
native syntax (`"{count} singular form | {count} plural
form"`) instead of the manual ternaries used before. Variable interpolation uses
`{variable}` with `t(key, { variable })`.

### The base language becomes Spanish from Spain ("tuteo")

When extracting each string into `es.json`, the "voseo" conjugation was corrected
to "tuteo" ("tenés" → "tienes", "sos" → "eres", "Creá" → "Crea",
"Registrate" → "Regístrate", etc.) — it was not a mechanical 1:1 extraction of the text
that was there. Spanish from Spain ("tuteo") is now the style criterion for all
new text in the web going forward.

## 2026-07-30 update: English and Catalan

This confirms what the original ADR predicted: adding the two locales was
exactly adding `en.json`/`ca.json` (same key structure,
translated) + one entry in `locales` — without touching any component. The
only thing new beyond that was the language selector in the layout (there was
no UI control to change language before) and enabling
`detectBrowserLanguage` (which made no sense with a single locale).

## Next steps (explicitly out of scope for this task)

- The Android app (Kotlin/Compose) has the same hardcoded-text problem,
  but it's a completely different stack (string resources,
  `res/values-en/`, `res/values-ca/`) and is out of scope for this decision —
  extraction into `strings.xml` handled separately; adding the Android
  locales is documented in `docs/roadmap/TASKS.md`, not in this ADR.
- Backend error messages (`backend/internal/common/errors.go`)
  remain plain English strings + HTTP status, not stable error codes.
  The web no longer depends on that raw text for the happy-path
  translation flows (it builds its own Spanish copy by status code), except for
  one specific case in `useDecks.ts`/`useSettings.ts` that does
  `.includes(...)` on the backend's raw English message to distinguish two
  400-error cases — that was kept as-is, it's a pre-existing API design
  issue separate from this ADR.

## References

- `web/nuxt.config.ts` (module registration and `i18n` config)
- `web/i18n/i18n.config.ts` (`datetimeFormats`)
- `web/i18n/locales/{es,en,ca}.json` (all keys, in all three languages)
- `web/app/layouts/default.vue` (language selector)
- `web/app/composables/useDecks.ts` (`moxfieldImportError`, an example of a
  composable converted from raw error text to keys)
