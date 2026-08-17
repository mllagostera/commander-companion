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
    // El destino viaja a /login para volver después. Sin esto, abrir un enlace
    // profundo sin sesión (el QR de perfil escaneado desde el navegador, ver
    // pages/friends/add/[id].vue) te deja en el dashboard tras iniciar sesión,
    // con el enlace ya consumido y sin forma de recuperarlo.
    return navigateTo({ path: '/login', query: { redirect: to.fullPath } })
  }
  if (isAuthenticated.value && isPublic) {
    return navigateTo('/')
  }
})
