// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2026-07-27',
  devtools: { enabled: true },
  modules: ['@nuxtjs/tailwindcss', '@nuxt/eslint'],
  css: ['~/assets/css/main.css'],
  app: {
    head: {
      title: 'Commander Companion',
      titleTemplate: (title) => (title ? `${title} · Commander Companion` : 'Commander Companion'),
    },
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
    },
  },
})
