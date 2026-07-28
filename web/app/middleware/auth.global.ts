const PUBLIC_ROUTES = ['/login', '/register', '/verify-email']

/**
 * Gating de rutas. Se apoya en la cookie marcador `cc_session` (no httpOnly,
 * sin valor sensible) para poder decidir igual en SSR que en el cliente sin
 * pegarle a la API.
 */
export default defineNuxtRouteMiddleware((to) => {
  const { isAuthenticated } = useAuth()
  const isPublic = PUBLIC_ROUTES.includes(to.path)

  if (!isAuthenticated.value && !isPublic) {
    return navigateTo('/login')
  }
  if (isAuthenticated.value && isPublic) {
    return navigateTo('/')
  }
})
