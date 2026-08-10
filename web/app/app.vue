<script setup lang="ts">
// Moves keyboard focus to the new page's <h1> after each client-side
// navigation (skipping the very first load, where the browser's own
// start-of-document focus is already correct) — otherwise a keyboard or
// screen-reader user who follows a nav link keeps the previous page's focus
// context and has no signal that new content has loaded.
if (import.meta.client) {
  const nuxtApp = useNuxtApp()
  let hasNavigated = false

  nuxtApp.hook('page:finish', () => {
    if (!hasNavigated) {
      hasNavigated = true
      return
    }
    nextTick(() => {
      const heading = document.querySelector<HTMLElement>('main h1')
      if (!heading) return
      if (!heading.hasAttribute('tabindex')) heading.setAttribute('tabindex', '-1')
      heading.focus()
    })
  })
}
</script>

<template>
  <NuxtLoadingIndicator color="linear-gradient(90deg, #8b5cf6, #a855f7)" />
  <NuxtLayout>
    <NuxtPage />
  </NuxtLayout>
  <ToastHost />
  <SlowNavHint />
</template>
