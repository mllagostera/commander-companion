import { DISALLOWED_PATHS } from '#shared/seo'

/**
 * Served from a route instead of public/robots.txt because the Sitemap line
 * must be an absolute URL, and the origin depends on the deployment.
 */
export default defineEventHandler((event) => {
  const lines = [
    'User-agent: *',
    ...DISALLOWED_PATHS.map(path => `Disallow: ${path}`),
    '',
    `Sitemap: ${siteOrigin(event)}/sitemap.xml`,
    '',
  ]
  setResponseHeader(event, 'Content-Type', 'text/plain; charset=utf-8')
  return lines.join('\n')
})
