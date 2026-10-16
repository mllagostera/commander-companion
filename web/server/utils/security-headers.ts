import { createHash } from 'node:crypto'
import type { H3Event } from 'h3'

/**
 * Response security headers (see the 2026-08-01 security audit,
 * docs/roadmap/TASKS.md): CSP, HSTS, X-Frame-Options, X-Content-Type-Options,
 * Referrer-Policy. Applied by server/plugins/security-headers.ts's
 * `beforeResponse` hook, which is what gives applySecurityHeaders the final
 * rendered HTML body to hash (see inlineScriptHashes below).
 *
 * Disabled entirely in dev (see shouldApplyStrictHeaders): Vite's dev client
 * needs eval-based HMR and a WebSocket connection to itself that a
 * production-appropriate CSP would block, and dev is never internet-facing.
 */

const GOOGLE_IDENTITY_ORIGIN = 'https://accounts.google.com'
const MOXFIELD_ASSETS_ORIGIN = 'https://assets.moxfield.net'
/**
 * Card art for decks created by hand, whose commander is picked from the
 * Scryfall typeahead (server/api/scryfall/commanders.get.ts). Only img-src
 * needs it: the API call itself is made by Nitro, so connect-src stays 'self'.
 */
const SCRYFALL_IMAGES_ORIGIN = 'https://cards.scryfall.io'

/**
 * Origin the browser Sentry SDK sends events to: the DSN's own host (e.g.
 * https://o123.ingest.de.sentry.io). connect-src needs it or the CSP would
 * silently drop every client-side event. Undefined when Sentry is disabled
 * (empty DSN) or the DSN doesn't parse, so nothing is added to the policy.
 */
export function sentryIngestOrigin(dsn: string): string | undefined {
  if (!dsn) return undefined
  try {
    return new URL(dsn).origin
  }
  catch {
    return undefined
  }
}

export function shouldApplyStrictHeaders(): boolean {
  return !import.meta.dev
}

declare module 'h3' {
  interface H3EventContext {
    /**
     * Bodies of the inline scripts found in the app-rendered part of this
     * response's HTML (see collectUntrustedInlineScripts). Set by the
     * `render:html` hook in server/plugins/security-headers.ts.
     */
    untrustedInlineScripts?: Set<string>
  }
}

const INLINE_SCRIPT_RE = /<script(?![^>]*\ssrc=)([^>]*)>([\s\S]*?)<\/script>/gi
const SCRIPT_TYPE_RE = /\stype\s*=\s*["']?([^"'\s>]+)/i

/**
 * Script types the browser executes, i.e. the ones script-src governs.
 * `importmap` is included: it is checked against script-src too. Data blocks
 * (`application/json` for the Nuxt payload, `application/ld+json` for SEO) are
 * never executed, so they need no hash.
 */
const EXECUTABLE_SCRIPT_TYPES = new Set(['', 'text/javascript', 'module', 'importmap'])

function inlineScripts(html: string): { type: string, content: string }[] {
  const scripts: { type: string, content: string }[] = []
  for (const match of html.matchAll(INLINE_SCRIPT_RE)) {
    const content = match[2]
    if (!content) continue
    const type = (match[1]?.match(SCRIPT_TYPE_RE)?.[1] ?? '').toLowerCase()
    scripts.push({ type, content })
  }
  return scripts
}

/**
 * Bodies of every inline script inside the chunks Vue rendered for the app
 * (the `#__nuxt` root, teleports). Nuxt itself never puts an inline script
 * there — its own (the runtime-config one, the importmap) live in <head> and
 * after the app root — so anything found here can only come from rendered
 * content, i.e. an HTML injection, and must not be allowed. A `useHead()`
 * script with `tagPosition: 'bodyOpen'` lands in that region as well and would
 * be blocked: keep inline scripts in the head or at bodyClose.
 */
export function collectUntrustedInlineScripts(appChunks: string[]): Set<string> {
  return new Set(inlineScripts(appChunks.join('')).map(s => s.content))
}

/**
 * Hashes the executable inline <script>s (no src=) in the rendered HTML, so
 * script-src can allow exactly those instead of 'unsafe-inline'. Nuxt always
 * renders at least one (the runtime-config script, `window.__NUXT__=...` —
 * there's no external file to allowlist it by URL instead), and Vite adds a
 * `type="importmap"` one that isn't reachable through Nuxt's own `render:html`
 * hook, which is why the final composed body is hashed.
 *
 * Hashing the whole body blindly would also allow a script an attacker managed
 * to inject into the rendered page (stored/reflected XSS) — its hash would be
 * computed from the same response. `untrusted` is what prevents that: scripts
 * found in the app-rendered region (collectUntrustedInlineScripts) are left
 * out, so they stay blocked.
 */
export function inlineScriptHashes(html: string, untrusted: Set<string> = new Set()): string[] {
  const hashes = new Set<string>()
  for (const { type, content } of inlineScripts(html)) {
    if (!EXECUTABLE_SCRIPT_TYPES.has(type) || untrusted.has(content)) continue
    hashes.add(`'sha256-${createHash('sha256').update(content, 'utf8').digest('base64')}'`)
  }
  return [...hashes]
}

export function buildCsp(scriptHashes: string[], isHttps: boolean, sentryOrigin?: string): string {
  const directives: Record<string, string[]> = {
    'default-src': ["'self'"],
    'base-uri': ["'self'"],
    'object-src': ["'none'"],
    'frame-ancestors': ["'none'"],
    'form-action': ["'self'"],
    'script-src': ["'self'", ...scriptHashes, GOOGLE_IDENTITY_ORIGIN],
    // Attribute-level `:style="..."` bindings (used across app/pages,
    // app/components) can't be covered by a hash/nonce — only <style>
    // blocks/<link> can — so style-src stays 'unsafe-inline'. CSS injection
    // is a materially weaker primitive than script injection, which is what
    // script-src's hashes above actually defend against.
    'style-src': ["'self'", "'unsafe-inline'"],
    'img-src': ["'self'", 'data:', MOXFIELD_ASSETS_ORIGIN, SCRYFALL_IMAGES_ORIGIN],
    'font-src': ["'self'", 'data:'],
    'connect-src': ["'self'", GOOGLE_IDENTITY_ORIGIN, ...(sentryOrigin ? [sentryOrigin] : [])],
    'frame-src': [GOOGLE_IDENTITY_ORIGIN],
  }

  // Same condition as the HSTS header below: only meaningful (and not actively
  // harmful) when this response itself was served over HTTPS. On a plain-HTTP
  // response it forces every sub-resource fetch (the `/_nuxt/*` scripts and
  // stylesheets included) to be upgraded to https:// regardless of whether
  // anything is listening there — breaking the page instead of securing it.
  if (isHttps) directives['upgrade-insecure-requests'] = []

  return Object.entries(directives)
    .map(([directive, sources]) => (sources.length ? `${directive} ${sources.join(' ')}` : directive))
    .join('; ')
}

export function applySecurityHeaders(event: H3Event, body: string) {
  const isHttps = getRequestProtocol(event) === 'https'

  setResponseHeader(event, 'X-Content-Type-Options', 'nosniff')
  setResponseHeader(event, 'X-Frame-Options', 'DENY')
  setResponseHeader(event, 'Referrer-Policy', 'strict-origin-when-cross-origin')
  setResponseHeader(event, 'Content-Security-Policy', buildCsp(
    inlineScriptHashes(body, event.context.untrustedInlineScripts),
    isHttps,
    sentryIngestOrigin(useRuntimeConfig(event).public.sentryDsn),
  ))

  // HSTS only makes sense (and is only honored by browsers) over an actual
  // HTTPS connection — same check the CSP's upgrade-insecure-requests above
  // uses, and that setSessionCookies uses for the Secure cookie flag.
  // includeSubDomains without preload: a strong default without the
  // harder-to-reverse commitment of submitting to browsers' preload lists.
  if (isHttps) {
    setResponseHeader(event, 'Strict-Transport-Security', 'max-age=63072000; includeSubDomains')
  }
}
