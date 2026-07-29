/**
 * Aplica el tema persistido apenas arranca la app en el cliente — corre para
 * cualquier página, con o sin layout (login/register usan layout: false), a
 * diferencia de inicializarlo desde dentro de layouts/default.vue.
 */
export default defineNuxtPlugin(() => {
  useTheme().initTheme()
})
