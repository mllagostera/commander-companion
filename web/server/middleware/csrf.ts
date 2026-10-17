/**
 * Rejects state-changing calls to the BFF (`/api/**`) made from another
 * origin, see isCrossOriginRequest. Safe methods are left alone: they don't
 * change anything, and `/api/backend/**` never turns a GET into a write.
 */
const SAFE_METHODS = new Set(['GET', 'HEAD', 'OPTIONS'])

export default defineEventHandler((event) => {
  if (SAFE_METHODS.has(event.method)) return
  if (!event.path.startsWith('/api/')) return
  if (!isCrossOriginRequest(event)) return

  const message = 'Cross-origin request blocked.'
  throw createError({
    statusCode: 403,
    statusMessage: message,
    message,
    data: { code: 403, message },
  })
})
