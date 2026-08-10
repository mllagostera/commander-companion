<script setup lang="ts">
// Wraps every client-side page navigation (nav links, redirects, etc.),
// regardless of which page's useAsyncData is slow to resolve — unlike the
// per-page loading state, this catches the case where the backend cold-starts
// while the user is already inside the app, not just at login.
const { active, start, stop } = useSlowRequestHint()

if (import.meta.client) {
  const nuxtApp = useNuxtApp()
  nuxtApp.hook('page:loading:start', start)
  nuxtApp.hook('page:loading:end', stop)
}
</script>

<template>
  <Transition name="cc-fade">
    <div v-if="active" class="fixed inset-x-0 top-0 z-40 flex justify-center p-3">
      <span
        role="status"
        aria-live="polite"
        class="rounded-full border px-4 py-2 text-xs shadow-[0_8px_30px_rgba(0,0,0,0.45)] backdrop-blur-xl"
        style="background: var(--toast-bg); border-color: var(--card-border); color: var(--text);"
      >
        {{ $t('common.slowBackendHint') }}
      </span>
    </div>
  </Transition>
</template>

<style scoped>
.cc-fade-enter-active,
.cc-fade-leave-active {
  transition: opacity 0.25s ease;
}
.cc-fade-enter-from,
.cc-fade-leave-to {
  opacity: 0;
}
</style>
