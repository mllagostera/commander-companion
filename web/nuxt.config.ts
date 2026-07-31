// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2026-07-27',
  devtools: { enabled: true },
  modules: ['@nuxtjs/tailwindcss', '@nuxt/eslint', '@nuxtjs/i18n'],
  css: ['~/assets/css/main.css'],
  app: {
    head: {
      title: 'Commander Companion',
    },
  },
  i18n: {
    locales: [
      { code: 'es', language: 'es-ES', file: 'es.json', name: 'Español' },
      { code: 'en', language: 'en-US', file: 'en.json', name: 'English' },
      { code: 'ca', language: 'ca-ES', file: 'ca.json', name: 'Català' },
    ],
    defaultLocale: 'es',
    strategy: 'no_prefix',
    // Detects the browser language only on the first visit: once there's a
    // cookie (from detection or the layout's manual selector), it doesn't
    // re-detect — so the manual selector isn't overridden on the next load.
    detectBrowserLanguage: {
      useCookie: true,
      cookieKey: 'cc_locale',
    },
  },
  runtimeConfig: {
    // API URL for calls made from the server (SSR). In Docker
    // Compose this points at the service's internal hostname ("http://api:8080/api/v1"),
    // which isn't reachable from the browser. Without NUXT_API_BASE, it falls back to the
    // same public value (the "npm run dev" without Docker case, where both processes
    // are on localhost).
    apiBase: process.env.NUXT_API_BASE || process.env.NUXT_PUBLIC_API_BASE || 'http://localhost:8080/api/v1',
    public: {
      // API URL for calls made from the browser (with /api/v1 at
      // the end). See backend/.env.example.
      apiBase: process.env.NUXT_PUBLIC_API_BASE || 'http://localhost:8080/api/v1',
      // Google Cloud Console Web Client ID, same value as GOOGLE_CLIENT_ID
      // in the backend. Empty = Google button disabled.
      googleClientId: process.env.NUXT_PUBLIC_GOOGLE_CLIENT_ID || '',
      // Bulk deck import by Moxfield username: the endpoint
      // (POST /moxfield-import) returns 501 because MoxfieldClient.ListDecksByUsername
      // is still an unimplemented stub (see backend/internal/moxfield/client.go).
      // The UI stays hidden until the endpoint actually works.
      enableBulkMoxfieldImport: process.env.NUXT_PUBLIC_ENABLE_BULK_MOXFIELD_IMPORT === 'true',
    },
  },
})
