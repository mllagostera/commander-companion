/** Only for visitors without a session: a logged-in user is sent to `/`. */
const GUEST_ONLY_ROUTES = ['/login', '/register', '/verify-email']

/**
 * Reachable with or without a session. `/` renders the landing page for
 * anonymous visitors and the dashboard otherwise (see pages/index.vue).
 */
const PUBLIC_ROUTES = ['/', ...GUEST_ONLY_ROUTES]

/**
 * Route gating. Relies on the `cc_session` marker cookie (not httpOnly, no
 * sensitive value) so it can decide the same way in SSR and on the client
 * without hitting the API.
 */
export default defineNuxtRouteMiddleware((to) => {
  // Unknown URLs fall through to Nuxt's 404 (app/error.vue) instead of being
  // treated as private pages: bouncing them to /login answered every broken
  // link with a 302, which search engines read as a soft 404.
  if (to.matched.length === 0) return

  const { isAuthenticated } = useAuth()

  if (!isAuthenticated.value && !PUBLIC_ROUTES.includes(to.path)) {
    // The destination travels to /login so the user comes back to it after.
    // Without this, opening a deep link with no session (the profile QR
    // scanned from the browser, see pages/friends/add/[id].vue) lands you on
    // the dashboard after logging in, with the link already consumed and no
    // way to get it back.
    return navigateTo({ path: '/login', query: { redirect: to.fullPath } })
  }
  if (isAuthenticated.value && GUEST_ONLY_ROUTES.includes(to.path)) {
    return navigateTo('/')
  }
})
