/**
 * Hydrates the session user before the route middleware runs.
 *
 * In SSR it fills `useState('auth-user')`, which travels in the payload, so the
 * client doesn't repeat the call. On a purely client-side navigation (with no prior
 * SSR) the same plugin does it in the browser.
 */
export default defineNuxtPlugin(async () => {
  const { user, isAuthenticated, fetchSession } = useAuth()

  // With no session marker there's nothing to hydrate; with the user already in the
  // payload, there's nothing to do either.
  if (!isAuthenticated.value || user.value) return

  await fetchSession()
})
