import type { H3Event } from 'h3'

/**
 * Public origin used in absolute SEO URLs (sitemap, robots.txt). Mirrors
 * app/composables/useSiteUrl.ts: NUXT_PUBLIC_SITE_URL when set, otherwise the
 * request's own origin.
 */
export function siteOrigin(event: H3Event): string {
  const configured = useRuntimeConfig(event).public.siteUrl
  return (configured || getRequestURL(event).origin).replace(/\/+$/, '')
}
