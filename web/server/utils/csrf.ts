import type { H3Event } from 'h3'

/**
 * Whether a state-changing request to the BFF was made by a page of another
 * origin. The session cookies are `SameSite=Lax`, which already keeps them off
 * cross-*site* POSTs, but "site" is the registrable domain: a page on a sibling
 * subdomain is same-site and would still get them sent. This closes that gap.
 *
 * `Sec-Fetch-Site` is the primary signal (every current browser sends it and a
 * page can't forge it); `Origin` is the fallback for older ones. A request
 * carrying neither isn't coming from a browser page, so it can't be riding a
 * victim's cookies, and is let through — curl and server-to-server callers
 * keep working.
 */
export function isCrossOriginRequest(event: H3Event): boolean {
  const fetchSite = getRequestHeader(event, 'sec-fetch-site')
  if (fetchSite) return fetchSite !== 'same-origin'

  const origin = getRequestHeader(event, 'origin')
  if (origin) return origin !== getRequestURL(event).origin

  return false
}
