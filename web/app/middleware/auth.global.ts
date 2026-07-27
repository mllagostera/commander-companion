export default defineNuxtRouteMiddleware((to) => {
  const { accessToken } = useAuth()
  const isLoginPage = to.path === '/login'

  if (!accessToken.value && !isLoginPage) {
    return navigateTo('/login')
  }
  if (accessToken.value && isLoginPage) {
    return navigateTo('/')
  }
})
