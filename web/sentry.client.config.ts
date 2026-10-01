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
    beforeSend: diagnoseEmptyError,
  })
}
