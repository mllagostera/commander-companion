# ADR-0014: Internacionalización de la web con `@nuxtjs/i18n`

**Estado:** Aceptada (2026-07-29)

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

Configuración (`web/nuxt.config.ts`):

```ts
i18n: {
  locales: [{ code: 'es', language: 'es-ES', file: 'es.json' }],
  defaultLocale: 'es',
  strategy: 'no_prefix',
  detectBrowserLanguage: false,
},
```

- `strategy: 'no_prefix'`: con un solo idioma activo no tiene sentido forzar
  `/es/...` en la URL.
- `detectBrowserLanguage: false`: no hay nada que detectar/redirigir todavía
  con un único locale.
- Los mensajes viven en `web/i18n/locales/es.json` (un solo archivo — con
  ~200-250 claves no vale la pena partirlo por dominio todavía) y el
  formateo de fechas (`datetimeFormats`) en `web/i18n/i18n.config.ts`, vía
  `defineI18nConfig`.

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

## Próximos pasos (explícitamente fuera de esta tarea)

- Traducir la app a **inglés** y **catalán** una vez la extracción a claves
  está completa. No se hace en este trabajo — la infraestructura queda lista
  para que agregar esos locales sea simplemente sumar `en.json`/`ca.json` y
  una entrada en `locales`, sin tocar componentes de nuevo.
- No se agrega selector de idioma visible en la UI todavía.
- La app Android (Kotlin/Compose) tiene el mismo problema de texto
  hardcodeado, pero es una stack completamente distinta (string resources)
  y queda fuera de esta decisión.
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
- `web/i18n/locales/es.json` (todas las claves)
- `web/app/composables/useDecks.ts` (`moxfieldImportError`, ejemplo de
  composable de error convertido a claves)
