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
    locales: [{ code: 'es', language: 'es-ES', file: 'es.json' }],
    defaultLocale: 'es',
    strategy: 'no_prefix',
    detectBrowserLanguage: false,
  },
  runtimeConfig: {
    // URL de la API para llamadas hechas desde el servidor (SSR). En Docker
    // Compose esto apunta al hostname interno del servicio ("http://api:8080/api/v1"),
    // que no es alcanzable desde el navegador. Sin NUXT_API_BASE, cae al mismo
    // valor público (caso de "npm run dev" sin Docker, donde ambos procesos
    // están en localhost).
    apiBase: process.env.NUXT_API_BASE || process.env.NUXT_PUBLIC_API_BASE || 'http://localhost:8080/api/v1',
    public: {
      // URL de la API para llamadas hechas desde el navegador (con /api/v1 al
      // final). Ver backend/.env.example.
      apiBase: process.env.NUXT_PUBLIC_API_BASE || 'http://localhost:8080/api/v1',
      // Web Client ID de Google Cloud Console, mismo valor que GOOGLE_CLIENT_ID
      // en el backend. Vacío = botón de Google deshabilitado.
      googleClientId: process.env.NUXT_PUBLIC_GOOGLE_CLIENT_ID || '',
      // Import masivo de decks por username de Moxfield: el endpoint
      // (POST /moxfield-import) devuelve 501 porque MoxfieldClient.ListDecksByUsername
      // sigue siendo un stub sin implementar (ver backend/internal/moxfield/client.go).
      // La UI queda oculta hasta que el endpoint funcione de verdad.
      enableBulkMoxfieldImport: process.env.NUXT_PUBLIC_ENABLE_BULK_MOXFIELD_IMPORT === 'true',
    },
  },
})
