import * as Sentry from '@sentry/nuxt'

// Browser-side Sentry. Loaded by @sentry/nuxt's module; does nothing unless
// NUXT_PUBLIC_SENTRY_DSN is set (see runtimeConfig.public.sentryDsn).
// sendDefaultPii stays off (the SDK default): no IPs, cookies or request
// bodies are sent.
//
// The module adds browserTracingIntegration on its own, which continues the
// trace Nitro started for the SSR render (the sentry-trace/baggage meta tags)
// and sends those headers on same-origin requests, i.e. to Nitro's BFF
// endpoints. Nitro forwards them to the Go backend, so an error in any of the
// three is linked to the same trace in Sentry.
const { sentryDsn, sentryTracesSampleRate } = useRuntimeConfig().public

if (sentryDsn) {
  Sentry.init({
    dsn: sentryDsn,
    tracesSampleRate: sentryTracesSampleRate,
  })
}
