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

export function shouldApplyStrictHeaders(): boolean {
  return !import.meta.dev
}

const INLINE_SCRIPT_RE = /<script(?![^>]*\ssrc=)[^>]*>([\s\S]*?)<\/script>/gi

/**
 * Hashes every inline <script> (no src=) actually present in the rendered
 * HTML, so script-src can allow exactly those instead of 'unsafe-inline'.
 * Nuxt always renders at least one inline script (the runtime-config payload,
 * `window.__NUXT__=...` — there's no external file to allowlist it by URL
 * instead), and some pages also get a `type="importmap"` one from Vite that
 * isn't reachable through Nuxt's own `render:html` hook, which is why this
 * hashes the final composed body instead of trying to tag individual
 * fragments as they're assembled. A hash computed from this response's own
 * body still won't match anything an attacker injects (reflected/stored
 * XSS), unlike 'unsafe-inline', which allows any inline script indiscriminately.
 */
export function inlineScriptHashes(html: string): string[] {
  const hashes = new Set<string>()
  for (const match of html.matchAll(INLINE_SCRIPT_RE)) {
    const content = match[1]
    if (!content) continue
    hashes.add(`'sha256-${createHash('sha256').update(content, 'utf8').digest('base64')}'`)
  }
  return [...hashes]
}

export function buildCsp(scriptHashes: string[], isHttps: boolean): string {
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
    'img-src': ["'self'", 'data:', MOXFIELD_ASSETS_ORIGIN],
    'font-src': ["'self'", 'data:'],
    'connect-src': ["'self'", GOOGLE_IDENTITY_ORIGIN],
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
  setResponseHeader(event, 'Content-Security-Policy', buildCsp(inlineScriptHashes(body), isHttps))

  // HSTS only makes sense (and is only honored by browsers) over an actual
  // HTTPS connection — same check the CSP's upgrade-insecure-requests above
  // uses, and that setSessionCookies uses for the Secure cookie flag.
  // includeSubDomains without preload: a strong default without the
  // harder-to-reverse commitment of submitting to browsers' preload lists.
  if (isHttps) {
    setResponseHeader(event, 'Strict-Transport-Security', 'max-age=63072000; includeSubDomains')
  }
}
