/**
 * Hidrata el usuario de la sesión antes de que corra el middleware de rutas.
 *
 * En SSR llena `useState('auth-user')`, que viaja en el payload, así que el
 * cliente no repite la llamada. En una navegación puramente cliente (sin SSR
 * previo) el mismo plugin la hace en el navegador.
 */
export default defineNuxtPlugin(async () => {
  const { user, isAuthenticated, fetchSession } = useAuth()

  // Sin marcador de sesión no hay nada que hidratar; con el usuario ya en el
  // payload, tampoco.
  if (!isAuthenticated.value || user.value) return

  await fetchSession()
})
