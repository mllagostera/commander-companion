<script setup lang="ts">
/**
 * One pill per available locale; the active one is filled. Shared by the
 * landing (header and mobile footer) and the app layout's user menu, so the
 * three stay identical. Layout classes passed by the parent land on the root.
 * Pills grow below sm, where they are tapped rather than clicked.
 */
const { locale, locales, setLocale } = useI18n()

const availableLocales = computed(() => locales.value as { code: string, name?: string }[])
</script>

<template>
  <div class="flex items-center gap-1" role="group" :aria-label="$t('nav.language')">
    <button
      v-for="l in availableLocales"
      :key="l.code"
      type="button"
      :aria-pressed="l.code === locale"
      :title="l.name"
      class="rounded-full border px-2.5 py-1 text-[11px] font-semibold uppercase transition-colors sm:px-2 sm:py-0.5"
      :style="l.code === locale
        ? { background: 'linear-gradient(135deg, #8b5cf6, #a855f7)', color: '#0a0714', borderColor: 'transparent' }
        : { color: 'var(--text-muted)', borderColor: 'var(--input-border)' }"
      @click="setLocale(l.code as 'es' | 'en' | 'ca')"
    >
      {{ l.code }}
    </button>
  </div>
</template>
