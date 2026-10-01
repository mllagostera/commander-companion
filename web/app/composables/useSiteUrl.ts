/**
 * Public origin for absolute SEO URLs (canonical, og:url, og:image):
 * NUXT_PUBLIC_SITE_URL when set, otherwise the current request's origin.
 * Mirrors server/utils/site-origin.ts.
 */
export function useSiteUrl() {
  const configured = useRuntimeConfig().public.siteUrl
  const origin = configured || useRequestURL().origin
  return origin.replace(/\/+$/, '')
}
