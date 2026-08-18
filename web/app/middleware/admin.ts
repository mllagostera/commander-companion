/**
 * Route guard for /admin/*. Applied per-page via `definePageMeta({ middleware: 'admin' })`
 * rather than as a second global middleware, since only a handful of pages need it.
 *
 * Runs after `auth.global.ts` (global middlewares run first, alphabetically), so by the
 * time this executes the user is already known to be authenticated for any non-public
 * route — this only adds the is_admin check on top. is_admin itself is only ever
 * trustworthy as a server-side gate (see auth.RequireAdmin, ADR-0018): every /admin/*
 * backend call is re-checked there regardless of what this client-side guard decides,
 * so this only exists to avoid flashing admin UI to a user who'll get a 403 from the API.
 */
export default defineNuxtRouteMiddleware(() => {
  const { user } = useAuth()

  if (!user.value?.is_admin) {
    return navigateTo('/')
  }
})
