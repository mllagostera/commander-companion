// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2026-07-27',
  devtools: { enabled: true },
  modules: ['@nuxtjs/tailwindcss', '@nuxt/eslint', '@nuxtjs/i18n', '@sentry/nuxt/module'],
  sentry: {
    // Source maps are only uploaded when the build has a Sentry auth token
    // (SENTRY_AUTH_TOKEN, plus SENTRY_ORG/SENTRY_PROJECT, read by the plugin
    // from the environment). Without one — local builds, CI, forks — the build
    // stays exactly as it was: no upload attempt and no hidden source maps.
    sourcemaps: {
      disable: !process.env.SENTRY_AUTH_TOKEN,
    },
  },
  css: ['~/assets/css/main.css'],
  features: {
    // Inlines the CSS into the SSR HTML instead of linking `/_nuxt/entry.*.css`.
    // Nuxt's default only inlines styles defined inside .vue components, so the
    // global bundle (Tailwind + assets/css/main.css, ~29 kB raw / ~6 kB brotli)
    // stayed a render-blocking <link>: a whole extra round trip before the first
    // paint, which on mobile is what Lighthouse flagged as ~150 ms. Inlining
    // costs almost nothing in bytes (the document goes from ~2.3 kB to ~7.9 kB
    // brotli, roughly what the separate stylesheet weighed) and removes the
    // request. The trade-off is that the CSS is no longer cached on its own
    // across full page loads — acceptable here because the document itself is
    // never cached (`max-age=0, must-revalidate`) and navigation after
    // hydration is client-side, so a session pays for it once.
    // No CSP impact: style-src already needs 'unsafe-inline' for the
    // attribute-level `:style` bindings (see server/utils/security-headers.ts).
    inlineStyles: true,
  },
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
      // Sentry project DSN for this web client (a different project from the
      // backend's SENTRY_DSN). Empty = Sentry disabled, on both the browser
      // (sentry.client.config.ts) and Nitro (sentry.server.config.ts) side.
      // It's public on purpose: a DSN only allows sending events, and the
      // browser SDK needs it anyway.
      sentryDsn: process.env.NUXT_PUBLIC_SENTRY_DSN || '',
      // Fraction (0..1) of page loads/navigations recorded as Sentry
      // performance traces. The decision is made here, at the start of the
      // trace, and travels in the sentry-trace header to Nitro and on to the Go
      // backend, which follow it — so a sampled trace shows browser, Nitro and
      // backend spans together. 0 (default) records none; errors in all three
      // are still linked by trace ID either way.
      sentryTracesSampleRate: Number(process.env.NUXT_PUBLIC_SENTRY_TRACES_SAMPLE_RATE) || 0,
    },
  },
})
