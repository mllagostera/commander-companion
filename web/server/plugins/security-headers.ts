/**
 * Sets the response security headers (see server/utils/security-headers.ts)
 * on `beforeResponse`, not as regular middleware: script-src needs the final
 * rendered HTML body to compute this response's inline-script hashes, which
 * only exists once rendering has finished — a route handler or earlier
 * middleware never sees the composed output.
 */
export default defineNitroPlugin((nitroApp) => {
  nitroApp.hooks.hook('beforeResponse', (event, response) => {
    if (!shouldApplyStrictHeaders()) return
    if (typeof response.body !== 'string') return

    applySecurityHeaders(event, response.body)
  })
})
