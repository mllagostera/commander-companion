import * as Sentry from '@sentry/nuxt'
import type { ErrorEvent, EventHint } from '@sentry/nuxt'

// Diagnostic helper: turn whatever was actually thrown into a readable string,
// never throwing itself. Used by beforeSend to attach the raw captured value to
// events that arrive without a usable message ("No error message" in Sentry).
function describeRawException(raw: unknown): string {
  if (raw === undefined) return 'undefined'
  if (raw === null) return 'null'
  if (typeof raw === 'string') return raw === '' ? '<empty string>' : raw
  try {
    const json = JSON.stringify(raw)
    return json === '{}' || json === undefined
      ? `${Object.prototype.toString.call(raw)} ${String(raw)}`
      : `${Object.prototype.toString.call(raw)} ${json}`
  } catch {
    return `${Object.prototype.toString.call(raw)} ${String(raw)}`
  }
}

// Diagnostic beforeSend: tags events that carry no usable message with the raw
// captured value, so "No error message" events can be filtered and inspected in
// Sentry. Robust (never throws) and non-destructive (always returns the event).
function diagnoseEmptyError(event: ErrorEvent, hint: EventHint): ErrorEvent {
  try {
    const hasMessage = typeof event.message === 'string' && event.message.trim() !== ''
    const hasExceptionValue = (event.exception?.values ?? []).some(
      v => typeof v.value === 'string' && v.value.trim() !== '',
    )
    if (!hasMessage && !hasExceptionValue) {
      const raw = hint?.originalException ?? hint?.syntheticException
      event.extra = { ...event.extra, nonErrorOrEmpty: describeRawException(raw) }
      event.tags = { ...event.tags, 'diagnostic.empty_error': 'true' }
    }
  } catch {
    // Never let diagnostics drop an event.
  }
  return event
}

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
    beforeSend: diagnoseEmptyError,
  })
}
