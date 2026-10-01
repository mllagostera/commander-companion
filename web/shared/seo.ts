/**
 * Which URLs search engines should know about, shared by the sitemap and
 * robots.txt (server/routes/) and the per-page robots meta (app/).
 *
 * Only the landing is listed in the sitemap: /login and /register stay
 * crawlable (people do search "<app name> login") but carry no content worth
 * ranking. Everything behind a session is disallowed: without a cookie it
 * just redirects to /login, so a crawler would only ever see the login form
 * under dozens of different URLs.
 */
export const SITEMAP_PATHS = ['/']

export const DISALLOWED_PATHS = [
  '/api/',
  '/admin',
  '/decks',
  '/friends',
  '/play',
  '/playgroups',
  '/settings',
  '/statistics',
  '/tournaments',
  '/verify-email',
]
