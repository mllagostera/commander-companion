/**
 * Sets the response security headers (see server/utils/security-headers.ts)
 * on `beforeResponse`, not as regular middleware: script-src needs the final
 * rendered HTML body to compute this response's inline-script hashes, which
 * only exists once rendering has finished — a route handler or earlier
 * middleware never sees the composed output.
 *
 * `render:html` runs first, while the document is still split into chunks:
 * it records which inline scripts sit in the app-rendered region, so
 * `beforeResponse` can leave those out of the allowlist.
 */
export default defineNitroPlugin((nitroApp) => {
  nitroApp.hooks.hook('render:html', (html, { event }) => {
    if (!shouldApplyStrictHeaders()) return

    event.context.untrustedInlineScripts = collectUntrustedInlineScripts([...html.bodyPrepend, ...html.body])
  })

  nitroApp.hooks.hook('beforeResponse', (event, response) => {
    if (!shouldApplyStrictHeaders()) return
    if (typeof response.body !== 'string') return

    applySecurityHeaders(event, response.body)
  })
})
