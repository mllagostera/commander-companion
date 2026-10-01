<script setup lang="ts">
import type { NuxtError } from '#app'

/**
 * App-wide error page: unknown URLs (404, see auth.global.ts, which lets them
 * through instead of bouncing to /login) and unexpected failures. Rendered
 * outside any layout, so it draws its own shell like verify-email.vue.
 * Always noindex: an error response has nothing worth indexing.
 */
const props = defineProps<{ error: NuxtError }>()

const { t, localeProperties } = useI18n()

const isNotFound = computed(() => props.error.statusCode === 404)

// Nuxt renders this instead of app.vue, so <html lang> has to be set here too.
useHead({
  htmlAttrs: {
    lang: computed(() => localeProperties.value.language ?? localeProperties.value.code),
  },
})
useSeoMeta({
  title: () => (isNotFound.value ? t('errorPage.notFound.title') : t('errorPage.generic.title')),
  robots: 'noindex, nofollow',
})

function goHome() {
  clearError({ redirect: '/' })
}
</script>

<template>
  <main class="relative min-h-screen overflow-hidden" style="background: var(--bg-gradient); color: var(--text);">
    <div class="cc-blob top-[-160px] right-[-140px] h-[460px] w-[460px] rounded-[63%_37%_54%_46%/48%_42%_58%_52%]" style="background: radial-gradient(circle, rgba(167,139,250,0.35), rgba(167,139,250,0) 70%);" />
    <div class="cc-blob bottom-[-200px] left-[-160px] h-[520px] w-[520px] rounded-[42%_58%_65%_35%/55%_45%_55%_45%]" style="background: radial-gradient(circle, rgba(168,85,247,0.22), rgba(168,85,247,0) 70%);" />

    <div class="relative z-[1] flex min-h-screen flex-col items-center justify-center gap-8 p-6">
      <span class="flex flex-col items-center gap-3.5">
        <AppLogo size="lg" />
        <span class="cc-gradient-text text-[22px] font-semibold tracking-wide">TapeandoCartones</span>
      </span>

      <div
        class="flex w-full max-w-[340px] flex-col gap-4 rounded-[var(--radius-xl)] border p-[26px] text-center"
        style="background: var(--card-bg-strong); border-color: var(--card-border);"
      >
        <p class="text-[13px] font-semibold tracking-wide" style="color: var(--accent-link);">{{ error.statusCode }}</p>
        <h1 class="text-xl font-semibold">
          {{ isNotFound ? $t('errorPage.notFound.title') : $t('errorPage.generic.title') }}
        </h1>
        <p class="text-sm" style="color: var(--text-muted);">
          {{ isNotFound ? $t('errorPage.notFound.body') : $t('errorPage.generic.body') }}
        </p>
        <button
          type="button"
          class="rounded-full px-5 py-3 text-center text-[13px] font-semibold text-[#0a0714]"
          style="background: linear-gradient(90deg, #8b5cf6, #a855f7);"
          @click="goHome"
        >
          {{ $t('errorPage.goHome') }}
        </button>
      </div>
    </div>
  </main>
</template>
