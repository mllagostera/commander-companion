import * as Sentry from '@sentry/nuxt'

// Nitro-side Sentry, bundled into the server build and run at startup by
// @sentry/nuxt's module. It runs before Nuxt's runtime config exists, so it
// reads the same variables as runtimeConfig.public.sentryDsn/
// sentryTracesSampleRate straight from the environment. Empty DSN = Sentry
// disabled.
// sendDefaultPii stays off (the SDK default): the BFF endpoints proxy
// passwords and session tokens, none of which may end up in an event.
//
// Outgoing fetches to the Go backend carry the sentry-trace/baggage headers,
// which is what links backend errors and spans to this trace.
const dsn = process.env.NUXT_PUBLIC_SENTRY_DSN

if (dsn) {
  Sentry.init({
    dsn,
    tracesSampleRate: Number(process.env.NUXT_PUBLIC_SENTRY_TRACES_SAMPLE_RATE) || 0,
  })
}
