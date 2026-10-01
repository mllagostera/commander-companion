import { SITEMAP_PATHS } from '#shared/seo'

export default defineEventHandler((event) => {
  const origin = siteOrigin(event)
  const urls = SITEMAP_PATHS.map(path => `  <url><loc>${origin}${path}</loc></url>`)
  setResponseHeader(event, 'Content-Type', 'application/xml; charset=utf-8')
  return [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
    ...urls,
    '</urlset>',
    '',
  ].join('\n')
})
