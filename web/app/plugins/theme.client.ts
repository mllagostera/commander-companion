/**
 * Applies the persisted theme as soon as the app starts on the client — runs for
 * any page, with or without a layout (login/register use layout: false), unlike
 * initializing it from within layouts/default.vue.
 */
export default defineNuxtPlugin(() => {
  useTheme().initTheme()
})
