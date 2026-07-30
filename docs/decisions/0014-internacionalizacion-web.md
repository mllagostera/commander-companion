# ADR-0014: Internacionalización de la web con `@nuxtjs/i18n`

**Estado:** Aceptada (2026-07-29), actualizada (2026-07-30) — inglés y catalán agregados

## Contexto

Toda la interfaz de la web (`web/app`) tenía el texto hardcodeado
directamente en templates, atributos y composables: ~200-250 literales
distintos repartidos en las 9 páginas, el layout y los composables que arman
mensajes de error/toast. No había ningún paquete de i18n instalado ni
precedente de esta decisión en ningún ADR previo — internacionalizar la web
era una tarea desde cero.

Además, el copy original estaba escrito en voseo argentino ("tenés", "sos",
"jugás", "Creá", "Registrate"...), sin que eso fuera una decisión consciente
de estilo — simplemente el dialecto con el que se fue escribiendo la app.

## Decisión

### Librería: `@nuxtjs/i18n`

Se adopta `@nuxtjs/i18n` (envuelve vue-i18n) como base de
internacionalización de la web, en vez de integrar vue-i18n a mano. Es el
módulo oficial de Nuxt: se registra en `modules` igual que
`@nuxtjs/tailwindcss`/`@nuxt/eslint` (ya presentes), auto-importa
`useI18n()`/`$t`/`t`/`d`, e integra SSR sin plugin propio.

Configuración (`web/nuxt.config.ts`), actualizada al agregar inglés y
catalán (2026-07-30):

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

- `strategy: 'no_prefix'` se mantiene: con `detectBrowserLanguage` +
  selector manual alcanza para elegir idioma sin necesidad de rutas
  `/en/...`/`/ca/...` — más simple para un dominio único sin SEO
  multi-idioma real detrás.
- `detectBrowserLanguage` pasa de `false` a activo ahora que hay 3 locales:
  detecta el idioma del navegador solo si todavía no hay cookie (`cc_locale`)
  — una vez que el usuario elige explícitamente por el selector del layout
  (`setLocale()`, ver más abajo) o ya se detectó una vez, esa cookie
  prevalece y no se vuelve a re-detectar en cargas siguientes.
- Los mensajes viven en `web/i18n/locales/{es,en,ca}.json`, mismas ~250
  claves en los tres archivos (namespacing sin cambios, ver más abajo) — se
  verificó paridad de claves y de placeholders de interpolación (`{count}`,
  `{username}`, etc.) entre los tres antes de mergear. El formateo de fechas
  (`datetimeFormats`) en `web/i18n/i18n.config.ts` ahora tiene entrada para
  los tres locales (antes solo `es`).
- Selector de idioma nuevo en el menú de usuario del layout
  (`app/layouts/default.vue`, junto al toggle de tema oscuro): tres pills
  `ES`/`EN`/`CA`, `useI18n().setLocale(code)` al click — la única forma de
  cambiar de idioma manualmente hoy (antes no había ninguna).

### Convención de claves

Namespacing por página/composable (`login.*`, `settings.*`,
`playgroups.list.*`, `playgroups.detail.*`, etc.), con `errors.*` reservado
para los mensajes que arman los composables de error (`useSettings.ts`,
`usePlaygroups.ts`, `useDecks.ts`, `useGames.ts`, `useUsers.ts`) — son
compartidos entre quien los llama, no se duplican por página — y `toast.*`
para los mensajes de `useToast()`. `common.*` agrupa los labels repetidos
entre páginas ("Cancelar", "Guardar", "Guardando…").

Pluralización (`playgroups.list.gamesPlayedCount`/`memberCount`) usa la
sintaxis nativa de vue-i18n (`"{count} forma singular | {count} forma
plural"`) en vez de los ternarios manuales que había antes. Interpolación de
variables usa `{variable}` con `t(key, { variable })`.

### El idioma base pasa a ser español de España (tuteo)

Al extraer cada string a `es.json` se corrigió la conjugación de voseo a
tuteo ("tenés" → "tienes", "sos" → "eres", "Creá" → "Crea", "Registrate" →
"Regístrate", etc.) — no fue una extracción mecánica 1:1 del texto que
había. Español de España (tuteo) queda como el criterio de estilo para todo
texto nuevo en la web de acá en adelante.

## Actualización 2026-07-30: inglés y catalán

Confirmado lo que predecía la ADR original: agregar los dos locales fue
exactamente sumar `en.json`/`ca.json` (misma estructura de claves,
traducidas) + una entrada en `locales` — sin tocar ningún componente. Lo
único nuevo fuera de eso fue el selector de idioma en el layout (no existía
ningún control de UI para cambiar de idioma) y activar
`detectBrowserLanguage` (no tenía sentido con un solo locale).

## Próximos pasos (explícitamente fuera de esta tarea)

- La app Android (Kotlin/Compose) tiene el mismo problema de texto
  hardcodeado, pero es una stack completamente distinta (string resources,
  `res/values-en/`, `res/values-ca/`) y queda fuera de esta decisión —
  extracción a `strings.xml` resuelta por separado, agregar los locales de
  Android se documenta en `docs/roadmap/TASKS.md`, no en esta ADR.
- Los mensajes de error del backend (`backend/internal/common/errors.go`)
  siguen siendo strings en inglés + status HTTP, no códigos de error
  estables. La web ya no depende de ese texto crudo para las rutas felices
  de traducción (arma su propio copy en español por status code), salvo un
  caso puntual en `useDecks.ts`/`useSettings.ts` que hace `.includes(...)`
  sobre el mensaje crudo en inglés del backend para distinguir dos casos de
  error 400 — eso se preservó tal cual, es un problema de diseño de API
  preexistente y separado de esta ADR.

## Referencias

- `web/nuxt.config.ts` (registro del módulo y config `i18n`)
- `web/i18n/i18n.config.ts` (`datetimeFormats`)
- `web/i18n/locales/{es,en,ca}.json` (todas las claves, en los tres idiomas)
- `web/app/layouts/default.vue` (selector de idioma)
- `web/app/composables/useDecks.ts` (`moxfieldImportError`, ejemplo de
  composable de error convertido a claves)
